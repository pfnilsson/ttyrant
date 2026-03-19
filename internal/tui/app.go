package tui

import (
	"context"
	"fmt"
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

const refreshInterval = 2 * time.Second

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

	viewMode viewMode
	wtRows   []wtRow
	wtCursor int

	wtConfirmDelete bool
	wtCloneActive   bool
	wtNewStep       int // 0=none, 1=repo picker, 2=branch picker
	wtNewRepoName string
	wtNewRepoPath string
	picker        picker

	openStep    int // 0=none, 1=project picker, 2=worktree picker (bare repo)
	openProject worktree.Project
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
	rows           []model.SessionRow
	hooksInstalled bool
	err            error
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
			rows:           rows,
			hooksInstalled: install.IsInstalled(),
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
		if firstLoad && !msg.hooksInstalled {
			m.showHooksPrompt = true
		}
		m.loaded = true
		m.err = msg.err
		m.rows = msg.rows
		m.hooksInstalled = msg.hooksInstalled
		sortRows(m.rows)
		m.clampCursor()
		if firstLoad && len(m.rows) == 0 && !m.showHooksPrompt {
			return m.startOpen()
		}
		return m, nil

	case installResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.hooksInstalled = true
		}
		return m, nil

	case wtCloneResultMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, wtRefreshCmd()

	case wtRefreshMsg:
		m.wtRows = msg.rows
		if msg.err != nil {
			m.err = msg.err
		}
		m.clampWtCursor()
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

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHooksPrompt {
		return m.handleHooksPrompt(msg)
	}

	if m.confirmKill {
		return m.handleConfirmKill(msg)
	}

	if m.wtConfirmDelete {
		return m.handleWtConfirmDelete(msg)
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
		m.err = fmt.Errorf("no projects found")
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
			return m, nil
		}
	}

	cmd := tmux.AttachSessionCmd(name)
	return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
}

func (m Model) quitCmd() tea.Msg {
	state.WriteCache(m.rows)
	return tea.QuitMsg{}
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

	// === Status bar (errors) ===
	var statusLines []string

	if m.err != nil {
		statusLines = append(statusLines,
			styleUnknown.Render(fmt.Sprintf(" Error: %v", m.err)))
	}
	statusBar := strings.Join(statusLines, "\n")

	// === Main content ===
	// Calculate available height: help(1) - borders(2) - status lines - padding
	usedLines := 1 // help bar
	if len(statusLines) > 0 {
		usedLines += len(statusLines)
	}
	contentH := max(
		// 2 for frame border
		m.height-usedLines-2, 3)

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
	if statusBar != "" {
		body.WriteString(statusBar)
		body.WriteByte('\n')
	}
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
	}

	return frame
}

func (m Model) renderHelp(width int) string {
	type binding struct {
		key  string
		desc string
	}

	bindings := []binding{
		{"q", "quit"},
		{"j/k", "navigate"},
		{"1-9", "attach"},
		{"a", "attach:1"},
		{"A", "attach:2"},
		{"o", "open"},
		{"w", "worktrees"},
		{"d", "kill"},
	}

	var parts []string
	for _, b := range bindings {
		parts = append(parts, styleHelpKey.Render(b.key)+" "+styleHelp.Render(b.desc))
	}
	line := strings.Join(parts, styleHelp.Render("  "))

	pad := max(width-lipgloss.Width(line), 0)
	return line + strings.Repeat(" ", pad)
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
