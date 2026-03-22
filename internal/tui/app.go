package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pfnilsson/ttyrant/internal/install"
	"github.com/pfnilsson/ttyrant/internal/merge"
	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/scanner"
	"github.com/pfnilsson/ttyrant/internal/state"
	"github.com/pfnilsson/ttyrant/internal/tmux"
	"github.com/pfnilsson/ttyrant/internal/worktree"
)

const refreshInterval = 1 * time.Second

type viewMode int

const (
	viewSessions  viewMode = iota
	viewWorktrees
)

// Model is the Bubble Tea model for the ttyrant TUI.
type Model struct {
	rows   []model.SessionRow
	cursor int
	width  int
	height int

	confirmKill    bool // awaiting y/n to kill session
	showHooksPrompt bool // awaiting y/n to install hooks

	loaded         bool // true after first refresh completes
	hooksInstalled bool
	scanner        *scanner.Scanner
	err            error
	showError      bool // awaiting keypress to dismiss error

	viewMode viewMode
	wtRows   []wtRow
	wtCursor int

	wtConfirmDelete bool
	wtCloneActive   bool
	wtNewStep       int // 0=none, 1=repo picker, 2=branch picker, 3=base branch picker
	wtNewRepoName string
	wtNewRepoPath string
	wtNewBranch   string   // new branch name (set when entering step 3)
	wtRemotes     []string // cached remote branches for current repo
	picker        picker

	openStep    int // 0=none, 1=project picker, 2=worktree picker (bare repo)
	openProject worktree.Project

	copyFlash bool // briefly show "Copied!" after yank
	showHelp  bool // help popup visible
}

// New creates the initial TUI model.
func New() Model {
	cached := state.ReadCache()
	sortRows(cached)
	return Model{
		rows:    cached,
		scanner: scanner.New(),
	}
}

// tickMsg triggers periodic refresh.
type tickMsg time.Time

// refreshMsg carries fresh data after a scan.
type refreshMsg struct {
	rows []model.SessionRow
	err  error
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refreshCmd(s *scanner.Scanner) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		ctx := context.Background()

		tmuxSessions := tmux.ListSessions()

		procs, err := s.Scan(ctx)
		if err != nil {
			return refreshMsg{err: err}
		}

		hookStates, err := state.ReadAllStates()
		if err != nil {
			return refreshMsg{err: err}
		}

		rows := merge.Merge(tmuxSessions, procs, hookStates, now)
		return refreshMsg{
			rows: rows,
		}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), refreshCmd(m.scanner))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.viewMode == viewWorktrees {
			return m, tea.Batch(tickCmd(), wtRefreshCmd())
		}
		return m, tea.Batch(tickCmd(), refreshCmd(m.scanner))

	case refreshMsg:
		firstLoad := !m.loaded
		if firstLoad {
			m.hooksInstalled = install.IsInstalled()
			if !m.hooksInstalled {
				m.showHooksPrompt = true
			}
		}
		m.loaded = true
		m.err = msg.err
		m.rows = msg.rows
		sortRows(m.rows)
		m.clampCursor()

		if firstLoad && len(m.rows) == 0 && !m.showHooksPrompt {
			return m.startOpen()
		}
		return m, nil

	case installResultMsg:
		if msg.err != nil {
			m.setError(msg.err)
		} else {
			m.hooksInstalled = true
		}
		return m, nil

	case wtCloneResultMsg:
		if msg.err != nil {
			m.setError(msg.err)
		}
		return m, wtRefreshCmd()

	case wtCreateResultMsg:
		if msg.err != nil {
			m.setError(msg.err)
			return m, nil
		}
		attachCmd := tmux.AttachSessionCmd(msg.sessionName)
		return m, tea.Sequence(tea.ExecProcess(attachCmd, nil), m.quitCmd)

	case wtDeleteResultMsg:
		if msg.err != nil {
			m.setError(msg.err)
		}
		return m, wtRefreshCmd()

	case wtRefreshMsg:
		m.wtRows = msg.rows
		if msg.err != nil {
			m.setError(msg.err)
		}
		m.clampWtCursor()
		return m, nil

	case clearCopyFlashMsg:
		m.copyFlash = false
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	default:
		// Forward non-key messages (e.g. cursor blink) to active picker.
		if m.wtNewStep > 0 || m.openStep > 0 {
			var cmd tea.Cmd
			m.picker.input, cmd = m.picker.input.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

type installResultMsg struct{ err error }
type clearCopyFlashMsg struct{}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showError {
		switch {
		case msg.Type == tea.KeyEsc:
			m.showError = false
			m.err = nil
		case msg.String() == "y" && m.err != nil:
			copyToClipboard(m.err.Error())
			m.copyFlash = true
			return m, tea.Tick(time.Second, func(time.Time) tea.Msg { return clearCopyFlashMsg{} })
		}
		return m, nil
	}

	if m.showHooksPrompt {
		return m.handleHooksPrompt(msg)
	}

	if m.confirmKill {
		return m.handleConfirmKill(msg)
	}

	if m.wtConfirmDelete {
		return m.handleWtConfirmDelete(msg)
	}

	if m.showHelp {
		if msg.Type == tea.KeyEsc || msg.String() == "?" {
			m.showHelp = false
		}
		return m, nil
	}

	if m.wtCloneActive {
		return m.handleWtClone(msg)
	}

	if m.wtNewStep > 0 {
		return m.handlePickerKey(msg)
	}

	if m.openStep > 0 {
		return m.handleOpenKey(msg)
	}

	if msg.String() == "?" {
		m.showHelp = true
		return m, nil
	}

	if m.viewMode == viewWorktrees {
		return m.handleWorktreeKey(msg)
	}

	// Number keys 1-9: attach to session by row number.
	if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx < len(m.rows) {
			m.cursor = idx
			return m.attachTmuxSession()
		}
		return m, nil
	}

	switch matchKey(msg) {
	case keyQuit:
		return m, m.quitCmd
	case keyKill:
		if len(m.rows) > 0 && m.cursor < len(m.rows) && m.rows[m.cursor].SessionName != "" {
			m.confirmKill = true
		}
		return m, nil
	case keyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case keyDown:
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		return m, nil
	case keyAttachTmux:
		return m.attachTmux(1)
	case keyAttachTmux2:
		return m.attachTmux(2)
	case keyWorktree:
		m.viewMode = viewWorktrees
		return m, wtRefreshCmd()
	case keyOpen:
		return m.startOpen()
	case keyScratch:
		return m.openScratch()
	}

	return m, nil
}

func (m Model) handleConfirmKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmKill = false
		if m.cursor < len(m.rows) {
			name := m.rows[m.cursor].SessionName
			if name != "" {
				tmux.KillSession(name)
			}
		}
		return m, refreshCmd(m.scanner)
	default:
		m.confirmKill = false
		return m, nil
	}
}

func (m Model) handleHooksPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.showHooksPrompt = false
		return m, func() tea.Msg {
			err := install.Install(false)
			return installResultMsg{err: err}
		}
	default:
		m.showHooksPrompt = false
		return m, nil
	}
}

func (m Model) attachTmuxSession() (tea.Model, tea.Cmd) {
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		name := m.rows[m.cursor].SessionName
		if name != "" {
			cmd := tmux.AttachSessionCmd(name)
			return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
		}
	}
	return m, nil
}

func (m Model) attachTmux(window int) (tea.Model, tea.Cmd) {
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		name := m.rows[m.cursor].SessionName
		if name != "" {
			cmd := tmux.AttachCmd(name, window)
			return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
		}
	}
	return m, nil
}

func (m Model) startOpen() (tea.Model, tea.Cmd) {
	projects, err := worktree.ScanAllProjects()
	if err != nil || len(projects) == 0 {
		m.setError(fmt.Errorf("no projects found"))
		return m, nil
	}

	names := make([]string, len(projects))
	paths := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name
		paths[i] = p.Path
	}

	var cmd tea.Cmd
	m.picker, cmd = newPickerWithLabels("Open project", "", names, paths)
	m.openStep = 1
	return m, cmd
}

func (m Model) handleOpenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, m.quitCmd
	}

	selected, cancelled, cmd := m.picker.update(msg)

	if cancelled {
		m.openStep = 0
		return m, nil
	}

	if selected == "" {
		return m, cmd
	}

	switch m.openStep {
	case 1: // project selected
		projects, _ := worktree.ScanAllProjects()
		var proj worktree.Project
		for _, p := range projects {
			if p.Name == selected {
				proj = p
				break
			}
		}

		if !proj.IsBare {
			return m.openNonBareProject(proj)
		}

		// Bare repo: show existing worktrees to open.
		m.openProject = proj
		wts, _ := worktree.ListWorktrees(proj.Path)

		if len(wts) == 0 {
			m.err = fmt.Errorf("no worktrees for %s", proj.Name)
			m.showError = true
			m.openStep = 0
			return m, nil
		}

		names := make([]string, len(wts))
		for i, wt := range wts {
			names[i] = wt.Branch
		}

		var pickerCmd tea.Cmd
		m.picker, pickerCmd = newPicker(
			fmt.Sprintf("Worktree for %s", proj.Name),
			"",
			names,
		)
		m.openStep = 2
		return m, pickerCmd

	case 2: // worktree selected for bare repo — open it.
		wts, _ := worktree.ListWorktrees(m.openProject.Path)
		for _, wt := range wts {
			if wt.Branch == selected {
				return m.openBareWorktree(m.openProject, wt)
			}
		}
		m.err = fmt.Errorf("worktree %q not found", selected)
		m.showError = true
		m.openStep = 0
		return m, nil
	}

	return m, nil
}

func (m Model) openNonBareProject(proj worktree.Project) (tea.Model, tea.Cmd) {
	m.openStep = 0
	name := worktree.SanitizeName(proj.Name)

	if !tmux.HasSession(name) {
		if err := tmux.CreateSession(name, proj.Path); err != nil {
			m.err = err
			m.showError = true
			return m, nil
		}
	}

	cmd := tmux.AttachSessionCmd(name)
	return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
}

func (m Model) openBareWorktree(proj worktree.Project, wt worktree.Worktree) (tea.Model, tea.Cmd) {
	m.openStep = 0
	name := worktree.SessionName(proj.Name, wt.Branch)

	if !tmux.HasSession(name) {
		if err := tmux.CreateWorktreeSession(name, wt.Path, proj.Name, wt.Branch); err != nil {
			m.err = err
			m.showError = true
			return m, nil
		}
	}

	cmd := tmux.AttachSessionCmd(name)
	return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
}

func (m Model) openScratch() (tea.Model, tea.Cmd) {
	name := tmux.NextScratchName()
	if err := tmux.CreateScratchSession(name); err != nil {
		m.err = err
		m.showError = true
		return m, nil
	}
	cmd := tmux.AttachSessionCmd(name)
	return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
}

func (m Model) quitCmd() tea.Msg {
	if err := state.WriteCache(m.rows); err != nil {
		fmt.Fprintf(os.Stderr, "ttyrant: warning: cache write failed: %v\n", err)
	}
	return tea.QuitMsg{}
}

func copyToClipboard(text string) {
	// Try common clipboard tools in order of preference.
	for _, name := range []string{"xclip", "xsel", "wl-copy", "pbcopy"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		var args []string
		switch name {
		case "xclip":
			args = []string{"-selection", "clipboard"}
		case "xsel":
			args = []string{"--clipboard", "--input"}
		}
		cmd := exec.Command(path, args...)
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()
		return
	}
}

func (m *Model) setError(err error) {
	m.err = err
	m.showError = err != nil
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	if m.openStep > 0 {
		return m.picker.view(m.width, m.height)
	}

	if m.viewMode == viewWorktrees {
		return m.viewWorktrees()
	}

	innerW := m.width - 2 // border left + right

	// === Main content ===
	// Calculate available height: help(1) - borders(2)
	contentH := max(m.height-1-2, 3)

	var content string
	header := renderTableHeader(innerW) + "\n"
	if len(m.rows) == 0 {
		content = header + m.renderEmpty(innerW, contentH-1)
	} else {
		content = header + renderTable(m.rows, m.cursor, innerW)
	}

	// Pad content to fill available height.
	contentLines := strings.Count(content, "\n")
	if contentLines < contentH {
		content += strings.Repeat("\n", contentH-contentLines)
	}

	// === Help bar ===
	helpBar := m.renderHelp(innerW)

	// === Assemble frame ===
	var body strings.Builder
	body.WriteString(content)
	body.WriteString(helpBar)

	frame := styleFrame.Width(innerW).Height(m.height - 2).Render(body.String())

	if m.confirmKill && m.cursor < len(m.rows) {
		name := m.rows[m.cursor].SessionName
		dialog := styleDialogTitle.Render("Kill session") + "\n\n" +
			fmt.Sprintf("Delete tmux session %q?", name) + "\n\n" +
			styleDialogHint.Render("y") + " confirm  " +
			styleDialogHint.Render("n") + " cancel"
		popup := styleDialog.Render(dialog)
		frame = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else if m.showError && m.err != nil {
		copyLabel := styleDialogHint.Render("y") + " yank"
		if m.copyFlash {
			copyLabel = styleDialogTitle.Render("Yanked!")
		}
		dialog := styleDialogTitle.Render("Error") + "\n\n" +
			m.err.Error() + "\n\n" +
			copyLabel + "  " +
			styleDialogHint.Render("esc") + " dismiss"
		popup := styleDialog.MaxWidth(m.width - 4).Render(dialog)
		frame = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else if m.showHooksPrompt {
		dialog := styleDialogTitle.Render("Install hooks?") + "\n\n" +
			"Hooks provide live Claude Code status.\n" +
			"Install them now?\n\n" +
			styleDialogHint.Render("y") + " install  " +
			styleDialogHint.Render("n") + " skip"
		popup := styleDialog.Render(dialog)
		frame = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else if m.showHelp {
		frame = renderHelpPopup(sessionBindings, m.width, m.height)
	}

	return frame
}


type binding struct {
	key  string
	desc string
}

var sessionBindings = []binding{
	{"q", "quit"},
	{"j/k", "navigate"},
	{"1-9", "attach"},
	{"a", "attach:1"},
	{"A", "attach:2"},
	{"o", "open"},
	{"s", "scratch"},
	{"w", "worktrees"},
	{"d", "kill"},
}

func (m Model) renderHelp(width int) string {
	return renderHelpBar(sessionBindings, width)
}

func renderHelpBar(bindings []binding, width int) string {
	sep := styleHelp.Render("  ")
	more := styleHelp.Render("  ") + styleHelpKey.Render("?") + " " + styleHelp.Render("help")
	moreW := lipgloss.Width(more)

	// Render all parts.
	parts := make([]string, len(bindings))
	for i, b := range bindings {
		parts[i] = styleHelpKey.Render(b.key) + " " + styleHelp.Render(b.desc)
	}

	line := strings.Join(parts, sep)
	if lipgloss.Width(line) <= width {
		pad := max(width-lipgloss.Width(line), 0)
		return line + strings.Repeat(" ", pad)
	}

	// Progressively drop from the right until it fits with the "? help" indicator.
	for n := len(parts) - 1; n >= 1; n-- {
		line = strings.Join(parts[:n], sep) + more
		if lipgloss.Width(line) <= width {
			pad := max(width-lipgloss.Width(line), 0)
			return line + strings.Repeat(" ", pad)
		}
	}

	// Absolute minimum: just the "? help" indicator.
	pad := max(width-moreW, 0)
	return more + strings.Repeat(" ", pad)
}

func renderHelpPopup(bindings []binding, width, height int) string {
	var b strings.Builder
	for _, bind := range bindings {
		b.WriteString(styleHelpKey.Render(bind.key))
		b.WriteString("  ")
		b.WriteString(styleHelp.Render(bind.desc))
		b.WriteByte('\n')
	}
	dialog := styleDialogTitle.Render("Keybindings") + "\n\n" +
		b.String() + "\n" +
		styleDialogHint.Render("esc") + " " + styleHelp.Render("dismiss")
	popup := styleDialog.Render(dialog)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m Model) renderEmpty(width, height int) string {
	var b strings.Builder

	emptyH := height / 3
	for range emptyH {
		b.WriteByte('\n')
	}

	title := styleEmptyTitle.Render("No Claude Code sessions found")
	titlePad := max((width-lipgloss.Width(title))/2, 0)
	b.WriteString(strings.Repeat(" ", titlePad) + title + "\n\n")

	hints := []string{
		"Start a Claude Code session to see it here.",
	}

	for _, h := range hints {
		line := styleEmptyHint.Render(h)
		pad := max((width-lipgloss.Width(line))/2, 0)
		b.WriteString(strings.Repeat(" ", pad) + line + "\n")
	}

	return b.String()
}
