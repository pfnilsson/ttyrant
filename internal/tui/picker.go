package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type picker struct {
	title   string
	hint    string
	input   textinput.Model
	items   []string   // fuzzy match targets
	labels  []string   // display labels (if nil, items are used)
	matches []fuzzy.Match
	cursor  int
}

func newPicker(title, hint string, items []string) (picker, tea.Cmd) {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 100
	cmd := ti.Focus()

	p := picker{
		title: title,
		hint:  hint,
		input: ti,
		items: items,
	}
	p.refilter()
	return p, cmd
}

func newPickerWithLabels(title, hint string, items, labels []string) (picker, tea.Cmd) {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 100
	cmd := ti.Focus()

	p := picker{
		title:  title,
		hint:   hint,
		input:  ti,
		items:  items,
		labels: labels,
	}
	p.refilter()
	return p, cmd
}

func (p *picker) refilter() {
	query := p.input.Value()
	if query == "" {
		p.matches = make([]fuzzy.Match, len(p.items))
		for i, item := range p.items {
			p.matches[i] = fuzzy.Match{Str: item, Index: i}
		}
	} else {
		p.matches = fuzzy.Find(query, p.items)
	}
	if p.cursor >= len(p.matches) {
		p.cursor = max(len(p.matches)-1, 0)
	}
}

func (p *picker) label(m fuzzy.Match) string {
	if p.labels != nil && m.Index < len(p.labels) {
		return p.labels[m.Index]
	}
	return m.Str
}

func (p *picker) selected() string {
	if p.cursor < len(p.matches) {
		return p.matches[p.cursor].Str
	}
	return ""
}

// update handles a key event. Returns (selected, cancelled, cmd).
// selected is non-empty when the user presses Enter on a match or types a new name.
func (p *picker) update(msg tea.KeyMsg) (string, bool, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return "", true, nil
	case tea.KeyEnter:
		if sel := p.selected(); sel != "" {
			return sel, false, nil
		}
		// No match — return raw input for new branch creation.
		if raw := strings.TrimSpace(p.input.Value()); raw != "" {
			return raw, false, nil
		}
		return "", false, nil
	}

	switch msg.String() {
	case "ctrl+k":
		if p.cursor > 0 {
			p.cursor--
		}
		return "", false, nil
	case "ctrl+j":
		if p.cursor < len(p.matches)-1 {
			p.cursor++
		}
		return "", false, nil
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.refilter()
	return "", false, cmd
}

func (p picker) view(width, height int) string {
	innerW := width - 2 // border left + right

	var b strings.Builder

	b.WriteString(styleDialogTitle.Render(p.title))
	b.WriteString("\n\n")
	b.WriteString(p.input.View())
	b.WriteString("\n\n")

	// Use available height for rows: total - title(1) - gaps(2) - input(1) - gap(1) - footer(1-2) - frame(2).
	footerLines := 1
	if p.hint != "" {
		footerLines = 2
	}
	visibleRows := max(height-8-footerLines, 3)

	// Render visible rows with scrolling to keep cursor in view.
	n := len(p.matches)
	start := 0
	if p.cursor >= visibleRows {
		start = p.cursor - visibleRows + 1
	}
	end := min(start+visibleRows, n)

	for i := start; i < end; i++ {
		m := p.matches[i]
		prefix := "  "
		if i == p.cursor {
			prefix = styleDot.Foreground(colorPrimary).Render("▶ ")
		}
		display := p.label(m)
		sel := i == p.cursor
		if p.labels != nil {
			display = highlightMatchInLabel(display, m.Str, m, sel)
		} else {
			display = highlightMatch(m, sel)
		}
		b.WriteString(prefix + display + "\n")
	}

	// Pad remaining rows to keep window static.
	rendered := end - start
	for range visibleRows - rendered {
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styleDialogHint.Render("enter") + " select  " + styleDialogHint.Render("esc") + " cancel")
	if p.hint != "" {
		b.WriteString("\n" + styleHelp.Render(p.hint))
	}

	return styleFrame.Width(innerW).Height(height - 2).Render(b.String())
}

// highlightMatchInLabel highlights matched characters within the name portion
// of a label (e.g. path). MatchedIndexes refer to the name, so we offset them
// to where the name appears in the label.
func highlightMatchInLabel(label, name string, m fuzzy.Match, selected bool) string {
	selStyle := lipgloss.NewStyle().Foreground(colorPrimary)

	if len(m.MatchedIndexes) == 0 {
		if selected {
			return selStyle.Render(label)
		}
		return label
	}

	offset := strings.LastIndex(label, name)
	if offset < 0 {
		if selected {
			return selStyle.Render(label)
		}
		return label
	}

	matched := make(map[int]bool, len(m.MatchedIndexes))
	for _, idx := range m.MatchedIndexes {
		matched[idx+offset] = true
	}

	var b strings.Builder
	for i, ch := range label {
		if matched[i] {
			b.WriteString(stylePickerMatch.Render(string(ch)))
		} else if selected {
			b.WriteString(selStyle.Render(string(ch)))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func highlightMatch(m fuzzy.Match, selected bool) string {
	selStyle := lipgloss.NewStyle().Foreground(colorPrimary)

	if len(m.MatchedIndexes) == 0 {
		if selected {
			return selStyle.Render(m.Str)
		}
		return m.Str
	}

	matched := make(map[int]bool, len(m.MatchedIndexes))
	for _, idx := range m.MatchedIndexes {
		matched[idx] = true
	}

	var b strings.Builder
	for i, ch := range m.Str {
		if matched[i] {
			b.WriteString(stylePickerMatch.Render(string(ch)))
		} else if selected {
			b.WriteString(selStyle.Render(string(ch)))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
