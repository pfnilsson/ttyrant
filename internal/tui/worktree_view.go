package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pfnilsson/ttyrant/internal/tmux"
	"github.com/pfnilsson/ttyrant/internal/worktree"
)

type wtRow struct {
	repoName     string
	repoPath     string
	branch       string
	worktreePath string
	head         string
	sessionName  string
	hasSession   bool
}

type wtRefreshMsg struct {
	rows []wtRow
	err  error
}

func wtRefreshCmd() tea.Cmd {
	return func() tea.Msg {
		repos, err := worktree.ScanRepos(worktree.ProjectsDir())
		if err != nil {
			return wtRefreshMsg{err: err}
		}

		sessions := tmux.ListSessions()
		sessionSet := make(map[string]bool, len(sessions))
		for _, s := range sessions {
			sessionSet[s.Name] = true
		}

		var rows []wtRow
		for _, repo := range repos {
			wts, err := worktree.ListWorktrees(repo.Path)
			if err != nil {
				continue
			}
			for _, wt := range wts {
				sn := worktree.SessionName(repo.Name, wt.Branch)
				rows = append(rows, wtRow{
					repoName:     repo.Name,
					repoPath:     repo.Path,
					branch:       wt.Branch,
					worktreePath: wt.Path,
					head:         wt.Head,
					sessionName:  sn,
					hasSession:   sessionSet[sn],
				})
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].repoName != rows[j].repoName {
				return rows[i].repoName < rows[j].repoName
			}
			return rows[i].branch < rows[j].branch
		})

		return wtRefreshMsg{rows: rows}
	}
}

// Column widths for worktree view.
const (
	wtPrefixW  = 6 // cursor(2) + id(1) + space(1) + dot(1) + space(1)
	wtRepoW    = 20
)

func wtSessionDot(hasSession bool) string {
	if hasSession {
		return styleDot.Foreground(colorGreen).Render(dotFilled)
	}
	return styleDot.Foreground(colorDim).Render(dotEmpty)
}

func renderWtHeader(width int) string {
	hdr := fmt.Sprintf("%*s%-*s %s",
		wtPrefixW, "",
		wtRepoW, "REPO",
		"BRANCH",
	)
	return styleHeader.Width(width).Render(hdr)
}

func renderWtTable(rows []wtRow, cursor int, width int) string {
	var b strings.Builder

	for i, row := range rows {
		selected := i == cursor
		line := formatWtRow(i, row, selected)
		if selected {
			b.WriteString(styleSelected.Render(line))
		} else if !row.hasSession {
			b.WriteString(styleDimmed.Render(line))
		} else {
			b.WriteString(styleRow.Render(line))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func formatWtRow(idx int, row wtRow, selected bool) string {
	cursor := "  "
	if selected {
		cursor = styleDot.Foreground(colorPrimary).Render("▶ ")
	}

	id := styleHelp.Render(fmt.Sprintf("%d", idx+1))
	if idx >= 9 {
		id = " "
	}

	dot := wtSessionDot(row.hasSession)

	repoName := row.repoName
	if len(repoName) > wtRepoW {
		repoName = repoName[:wtRepoW-1] + "~"
	}

	return fmt.Sprintf("%s%s %s %-*s %s",
		cursor, id, dot,
		wtRepoW, repoName,
		row.branch,
	)
}

func (m Model) handleWorktreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, m.quitCmd
	}

	// Number keys: attach to worktree by row number.
	if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx < len(m.wtRows) {
			m.wtCursor = idx
			return m.attachWorktreeWindow(1)
		}
		return m, nil
	}

	switch msg.String() {
	case "q":
		return m, m.quitCmd
	case "w", "esc":
		m.viewMode = viewSessions
		return m, nil
	case "a":
		if m.wtCursor < len(m.wtRows) {
			return m.attachWorktreeWindow(1)
		}
	case "A":
		if m.wtCursor < len(m.wtRows) {
			return m.attachWorktreeWindow(2)
		}
	case "o":
		return m.startOpen()
	case "n":
		return m.startNewWorktree()
	case "d":
		if m.wtCursor < len(m.wtRows) {
			m.wtConfirmDelete = true
		}
		return m, nil
	case "C":
		return m.startClone()
	case "j":
		if m.wtCursor < len(m.wtRows)-1 {
			m.wtCursor++
		}
	case "k":
		if m.wtCursor > 0 {
			m.wtCursor--
		}
	}

	return m, nil
}

func (m Model) startClone() (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.picker, cmd = newPicker("Clone bare repo", "paste git URL", nil)
	m.wtCloneActive = true
	return m, cmd
}

type wtCloneResultMsg struct{ err error }

type wtCreateResultMsg struct {
	sessionName string
	err         error
}

type wtDeleteResultMsg struct {
	err error
}

func (m Model) handleWtClone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, m.quitCmd
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.wtCloneActive = false
		return m, nil
	case tea.KeyEnter:
		url := strings.TrimSpace(m.picker.input.Value())
		if url == "" {
			return m, nil
		}
		m.wtCloneActive = false
		gitCmd, err := worktree.CloneBareCmd(url, worktree.ProjectsDir())
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, tea.ExecProcess(gitCmd, func(err error) tea.Msg {
			return wtCloneResultMsg{err: err}
		})
	}

	var cmd tea.Cmd
	m.picker.input, cmd = m.picker.input.Update(msg)
	return m, cmd
}

func (m Model) handleWtConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.wtConfirmDelete = false
		if m.wtCursor < len(m.wtRows) {
			row := m.wtRows[m.wtCursor]
			if row.hasSession {
				tmux.KillSession(row.sessionName)
			}
			cmd := worktree.RemoveWorktreeCmd(row.repoPath, row.worktreePath)
			return m, tea.Sequence(
				tea.ExecProcess(cmd, func(err error) tea.Msg {
					return wtDeleteResultMsg{err: err}
				}),
			)
		}
		return m, nil
	default:
		m.wtConfirmDelete = false
		return m, nil
	}
}

func (m Model) startNewWorktree() (tea.Model, tea.Cmd) {
	repos, err := worktree.ScanRepos(worktree.ProjectsDir())
	if err != nil || len(repos) == 0 {
		m.err = fmt.Errorf("no bare repos found")
		return m, nil
	}

	if len(repos) == 1 {
		// Skip repo picker, go directly to branch picker.
		m.wtNewRepoName = repos[0].Name
		m.wtNewRepoPath = repos[0].Path
		return m.openBranchPicker()
	}

	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Name
	}
	var cmd tea.Cmd
	m.picker, cmd = newPicker("Select repo", "", names)
	m.wtNewStep = 1
	return m, cmd
}

func (m Model) openBranchPicker() (tea.Model, tea.Cmd) {
	branches, err := worktree.ListRemoteBranches(m.wtNewRepoPath)
	if err != nil {
		m.err = err
		m.wtNewStep = 0
		return m, nil
	}

	var cmd tea.Cmd
	m.picker, cmd = newPicker(
		fmt.Sprintf("Branch for %s", m.wtNewRepoName),
		"no match = new branch",
		branches,
	)
	m.wtNewStep = 2
	return m, cmd
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, m.quitCmd
	}

	selected, cancelled, cmd := m.picker.update(msg)

	if cancelled {
		m.wtNewStep = 0
		return m, nil
	}

	if selected == "" {
		return m, cmd
	}

	switch m.wtNewStep {
	case 1: // repo selected
		m.wtNewRepoName = selected
		// Find path for selected repo.
		repos, _ := worktree.ScanRepos(worktree.ProjectsDir())
		for _, r := range repos {
			if r.Name == selected {
				m.wtNewRepoPath = r.Path
				break
			}
		}
		return m.openBranchPicker()

	case 2: // branch selected
		repoPath := m.wtNewRepoPath
		repoName := m.wtNewRepoName
		branch := selected
		m.wtNewStep = 0

		wtPath, gitCmd := worktree.CreateWorktreeCmd(repoPath, branch)
		if gitCmd == nil {
			// Worktree already exists, just create session and attach.
			sessionName := worktree.SessionName(repoName, branch)
			if err := tmux.CreateWorktreeSession(sessionName, wtPath, repoName, branch); err != nil {
				m.err = err
				return m, nil
			}
			attachCmd := tmux.AttachSessionCmd(sessionName)
			return m, tea.Sequence(tea.ExecProcess(attachCmd, nil), m.quitCmd)
		}

		return m, tea.ExecProcess(gitCmd, func(err error) tea.Msg {
			if err != nil {
				return wtCreateResultMsg{err: err}
			}
			sessionName := worktree.SessionName(repoName, branch)
			if err := tmux.CreateWorktreeSession(sessionName, wtPath, repoName, branch); err != nil {
				return wtCreateResultMsg{err: err}
			}
			return wtCreateResultMsg{sessionName: sessionName}
		})
	}

	m.wtNewStep = 0
	return m, nil
}

func (m Model) attachWorktreeWindow(window int) (tea.Model, tea.Cmd) {
	row := m.wtRows[m.wtCursor]

	if !row.hasSession {
		if err := tmux.CreateWorktreeSession(row.sessionName, row.worktreePath, row.repoName, row.branch); err != nil {
			m.err = err
			return m, nil
		}
	}

	cmd := tmux.AttachCmd(row.sessionName, window)
	return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
}

func (m Model) viewWorktrees() string {
	if m.wtNewStep > 0 || m.wtCloneActive {
		return m.picker.view(m.width, m.height)
	}

	innerW := m.width - 2

	var statusLines []string
	if m.err != nil {
		statusLines = append(statusLines,
			styleUnknown.Render(fmt.Sprintf(" Error: %v", m.err)))
	}
	statusBar := strings.Join(statusLines, "\n")

	usedLines := 1 // help bar
	if len(statusLines) > 0 {
		usedLines += len(statusLines)
	}
	contentH := max(m.height-usedLines-2, 3)

	var content string
	header := renderWtHeader(innerW) + "\n"
	if len(m.wtRows) == 0 {
		content = header + m.renderWtEmpty(innerW, contentH-1)
	} else {
		content = header + renderWtTable(m.wtRows, m.wtCursor, innerW)
	}

	contentLines := strings.Count(content, "\n")
	if contentLines < contentH {
		content += strings.Repeat("\n", contentH-contentLines)
	}

	helpBar := m.renderWtHelp(innerW)

	var body strings.Builder
	if statusBar != "" {
		body.WriteString(statusBar)
		body.WriteByte('\n')
	}
	body.WriteString(content)
	body.WriteString(helpBar)

	frame := styleFrame.Width(innerW).Height(m.height - 2).Render(body.String())

	if m.wtConfirmDelete && m.wtCursor < len(m.wtRows) {
		row := m.wtRows[m.wtCursor]
		dialog := styleDialogTitle.Render("Delete worktree") + "\n\n" +
			fmt.Sprintf("Remove worktree %s/%s?", row.repoName, row.branch) + "\n\n" +
			styleDialogHint.Render("y") + " confirm  " +
			styleDialogHint.Render("n") + " cancel"
		popup := styleDialog.Render(dialog)
		frame = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	return frame
}

func (m Model) renderWtHelp(width int) string {
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
		{"w", "sessions"},
		{"n", "new worktree"},
		{"d", "delete"},
		{"C", "clone"},
	}

	var parts []string
	for _, b := range bindings {
		parts = append(parts, styleHelpKey.Render(b.key)+" "+styleHelp.Render(b.desc))
	}
	line := strings.Join(parts, styleHelp.Render("  "))

	pad := max(width-lipgloss.Width(line), 0)
	return line + strings.Repeat(" ", pad)
}

func (m Model) renderWtEmpty(width, height int) string {
	var b strings.Builder

	emptyH := height / 3
	for range emptyH {
		b.WriteByte('\n')
	}

	title := styleEmptyTitle.Render("No bare repos found")
	titlePad := max((width-lipgloss.Width(title))/2, 0)
	b.WriteString(strings.Repeat(" ", titlePad) + title + "\n\n")

	hint := styleEmptyHint.Render("Clone a repo with bare setup to manage worktrees here.")
	hintPad := max((width-lipgloss.Width(hint))/2, 0)
	b.WriteString(strings.Repeat(" ", hintPad) + hint + "\n")

	return b.String()
}

func (m *Model) clampWtCursor() {
	if m.wtCursor >= len(m.wtRows) {
		m.wtCursor = len(m.wtRows) - 1
	}
	if m.wtCursor < 0 {
		m.wtCursor = 0
	}
}
