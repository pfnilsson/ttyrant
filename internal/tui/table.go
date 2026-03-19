package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfnilsson/ttyrant/internal/model"
)

// Status dot indicators.
const (
	dotFilled = "●"
	dotEmpty  = "○"
	dotHalf   = "◐"
)

// Column layout constants.
const (
	prefixW  = 6  // cursor(2) + id(1) + space(1) + dot(1) + space(1)
	statusW  = 12 // fixed width for status label
	nameW    = 16
	eventW   = 18
	ageW     = 8
	maxDirW  = 50
	minDirW  = 12
)

func statusDot(s model.SessionStatus) string {
	switch s {
	case model.StatusNeedsInput:
		return styleDot.Foreground(colorNeedsInput).Render(dotFilled)
	case model.StatusWorking:
		return styleDot.Foreground(colorWorking).Render(dotFilled)
	case model.StatusStarting:
		return styleDot.Foreground(colorStarting).Render(dotHalf)
	case model.StatusReady:
		return styleDot.Foreground(colorReady).Render(dotEmpty)
	case model.StatusDone:
		return styleDot.Foreground(colorDone).Render(dotEmpty)
	case model.StatusExited:
		return styleDot.Foreground(colorExited).Render(dotEmpty)
	case model.StatusActive:
		return styleDot.Foreground(colorSubtle).Render(dotEmpty)
	default:
		return styleDot.Foreground(colorUnknown).Render(dotFilled)
	}
}

func statusLabel(s model.SessionStatus) string {
	switch s {
	case model.StatusNeedsInput:
		return styleNeedsInput.Render("NEEDS INPUT")
	case model.StatusWorking:
		return styleWorking.Render("WORKING")
	case model.StatusStarting:
		return styleStarting.Render("STARTING")
	case model.StatusReady:
		return styleReady.Render("READY")
	case model.StatusDone:
		return styleDone.Render("DONE")
	case model.StatusExited:
		return styleExited.Render("EXITED")
	case model.StatusActive:
		return styleHelp.Render("no claude")
	default:
		return styleUnknown.Render("UNKNOWN")
	}
}

func dirColWidth(totalWidth int) int {
	fixedW := prefixW + statusW + nameW + eventW + ageW + 4 // 4 = spaces between columns
	dirW := max(totalWidth-fixedW, minDirW)
	return min(dirW, maxDirW)
}

func renderTableHeader(width int) string {
	dirW := dirColWidth(width)
	hdr := fmt.Sprintf("%*s%-*s %-*s %-*s %-*s %s",
		prefixW, "",
		statusW, "STATUS",
		nameW, "SESSION",
		dirW, "DIRECTORY",
		eventW, "LAST EVENT",
		"AGE",
	)
	return styleHeader.Width(width).Render(hdr)
}

func renderTable(rows []model.SessionRow, cursor int, width int) string {
	if len(rows) == 0 {
		return ""
	}

	dirW := dirColWidth(width)

	var b strings.Builder

	// Rows.
	for i, row := range rows {
		selected := i == cursor
		line := formatTableRow(i, row, nameW, dirW, eventW, ageW, selected)
		if row.Status == model.StatusDone || row.Status == model.StatusExited {
			b.WriteString(styleDimmed.Render(line))
		} else if selected {
			b.WriteString(styleSelected.Render(line))
		} else {
			b.WriteString(styleRow.Render(line))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func formatTableRow(idx int, row model.SessionRow, nameW, dirW, eventW, _ int, selected bool) string {
	// Cursor indicator.
	cursor := "  "
	if selected {
		cursor = styleDot.Foreground(colorPrimary).Render("▶ ")
	}

	// Row number (1-based), doubles as attach shortcut.
	id := styleHelp.Render(fmt.Sprintf("%d", idx+1))
	if idx >= 9 {
		id = " "
	}

	dot := statusDot(row.Status)
	status := statusLabel(row.Status)

	name := row.SessionName
	if len(name) > nameW {
		name = name[:nameW-1] + "~"
	}

	dir := truncateDir(row.Cwd, dirW)

	event := row.LastEvent
	if event == "" {
		event = "-"
	}
	if len(event) > eventW {
		event = event[:eventW-1] + "~"
	}

	age := formatDuration(row.IdleFor)

	// Pad status label to fixed width.
	statusVisible := lipgloss.Width(status)
	statusPad := ""
	if statusVisible < statusW {
		statusPad = strings.Repeat(" ", statusW-statusVisible)
	}

	return fmt.Sprintf("%s%s %s %s%s %-*s %-*s %-*s %s",
		cursor,
		id,
		dot,
		status, statusPad,
		nameW, name,
		dirW, dir,
		eventW, event,
		age,
	)
}

func truncateDir(dir string, maxWidth int) string {
	if dir == "" || maxWidth <= 0 {
		return ""
	}

	// Shorten home prefix.
	if strings.HasPrefix(dir, "/home/") {
		parts := strings.SplitN(dir, "/", 4)
		if len(parts) >= 4 {
			dir = "~/" + parts[3]
		}
	}

	runes := []rune(dir)
	if len(runes) <= maxWidth {
		return dir
	}

	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}

	base := filepath.Base(dir)
	baseRunes := []rune(base)
	if len(baseRunes)+5 <= maxWidth {
		return ".../" + base
	}

	return string(runes[:maxWidth-3]) + "..."
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
