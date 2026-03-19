package tui

import (
	"context"
	"fmt"
	"os"
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
)

const refreshInterval = 2 * time.Second

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
		return m, tea.Batch(tickCmd(), refreshCmd(m.scanner))

	case refreshMsg:
		if !m.loaded && !msg.hooksInstalled {
			m.showHooksPrompt = true
		}
		m.loaded = true
		m.err = msg.err
		m.rows = msg.rows
		m.hooksInstalled = msg.hooksInstalled
		sortRows(m.rows)
		m.clampCursor()
		return m, nil

	case installResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.hooksInstalled = true
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
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
			if os.Getenv("TTYRANT_TMUX_CLIENT") != "" {
				return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
			}
			return m, tea.ExecProcess(cmd, nil)
		}
	}
	return m, nil
}

func (m Model) attachTmux(window int) (tea.Model, tea.Cmd) {
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		name := m.rows[m.cursor].SessionName
		if name != "" {
			cmd := tmux.AttachCmd(name, window)
			if os.Getenv("TTYRANT_TMUX_CLIENT") != "" {
				return m, tea.Sequence(tea.ExecProcess(cmd, nil), m.quitCmd)
			}
			return m, tea.ExecProcess(tmux.AttachCmd(name, window), nil)
		}
	}
	return m, nil
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
