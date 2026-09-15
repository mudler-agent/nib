package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mudler/nib/theme"
)

type modelPicker struct {
	active    bool
	loading   bool
	requestID uint64
	all       []string
	matches   []string
	query     string
	selected  int
	offset    int
}

func (p *modelPicker) open(requestID uint64) {
	*p = modelPicker{active: true, loading: true, requestID: requestID}
}

func (p *modelPicker) close() {
	*p = modelPicker{}
}

func (p *modelPicker) setModels(models []string, current string, visible int) {
	p.loading = false
	p.all = append([]string(nil), models...)
	p.matches = append([]string(nil), models...)
	p.selected = 0
	p.offset = 0
	for i, model := range p.matches {
		if model == current {
			p.selected = i
			break
		}
	}
	p.scrollSelectionIntoView(visible)
}

func (p *modelPicker) appendQuery(text string, visible int) {
	p.query += text
	p.filter(visible)
}

func (p *modelPicker) backspace(visible int) {
	runes := []rune(p.query)
	if len(runes) == 0 {
		return
	}
	p.query = string(runes[:len(runes)-1])
	p.filter(visible)
}

func (p *modelPicker) filter(visible int) {
	p.matches = p.matches[:0]
	query := strings.ToLower(p.query)
	for _, model := range p.all {
		if strings.Contains(strings.ToLower(model), query) {
			p.matches = append(p.matches, model)
		}
	}
	p.selected = 0
	p.offset = 0
	p.scrollSelectionIntoView(visible)
}

func (p *modelPicker) move(delta int, visible int) {
	if len(p.matches) == 0 {
		p.selected = 0
		p.offset = 0
		return
	}
	p.selected += delta
	if p.selected < 0 {
		p.selected = 0
	}
	if p.selected >= len(p.matches) {
		p.selected = len(p.matches) - 1
	}
	p.scrollSelectionIntoView(visible)
}

func (p *modelPicker) scrollSelectionIntoView(visible int) {
	if visible < 1 {
		visible = 1
	}
	if p.selected < p.offset {
		p.offset = p.selected
	}
	if p.selected >= p.offset+visible {
		p.offset = p.selected - visible + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

func (p modelPicker) choice() (string, bool) {
	if p.selected < 0 || p.selected >= len(p.matches) {
		return "", false
	}
	return p.matches[p.selected], true
}

func modelPickerVisibleRows(height int) int {
	if rows := height - 3; rows > 1 {
		return rows
	}
	return 1
}

func renderModelPicker(p modelPicker, current string, width int, height int) string {
	if !p.active {
		return ""
	}

	var lines []string
	search := theme.ModelPickerSearchLabel
	if p.query != "" {
		search += " " + p.query
	}
	lines = append(lines, theme.Prompt.Render(clipModelPickerLine(search, width)))

	switch {
	case p.loading:
		lines = append(lines, theme.Meta.Render(clipModelPickerLine(theme.ModelPickerLoading, width)))
	case len(p.all) == 0:
		lines = append(lines, theme.Meta.Render(clipModelPickerLine(theme.ModelPickerEmpty, width)))
	case len(p.matches) == 0:
		lines = append(lines, theme.Meta.Render(clipModelPickerLine(theme.ModelPickerNoMatches, width)))
	default:
		visible := modelPickerVisibleRows(height)
		start := p.offset
		if start < 0 {
			start = 0
		}
		if start >= len(p.matches) {
			start = len(p.matches) - 1
		}
		end := min(start+visible, len(p.matches))
		for i := start; i < end; i++ {
			prefix := "  "
			style := theme.Help
			if i == p.selected {
				prefix = theme.PromptGlyph + " "
				style = theme.Prompt
			}
			line := prefix + p.matches[i]
			if p.matches[i] == current {
				line += " (current)"
			}
			lines = append(lines, style.Render(clipModelPickerLine(line, width)))
		}
	}

	lines = append(lines, theme.Hint.Render(clipModelPickerLine(theme.ModelPickerKeyHint, width)))
	return strings.Join(lines, "\n")
}

func clipModelPickerLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	if width == 1 {
		return "…"
	}

	var b strings.Builder
	used := 0
	for _, r := range line {
		runeWidth := lipgloss.Width(string(r))
		if used+runeWidth > width-1 {
			break
		}
		b.WriteRune(r)
		used += runeWidth
	}
	b.WriteRune('…')
	return b.String()
}
