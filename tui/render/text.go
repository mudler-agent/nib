package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TruncateLine caps a single line at w runes, ending with an ellipsis. A
// non-positive budget returns the bare ellipsis rather than an unclamped line.
func TruncateLine(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

// Wrap wraps text to fit within the specified width, preserving existing newlines
func Wrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if line == "" {
			result.WriteString("\n")
			continue
		}

		// Calculate the visual width (accounting for ANSI codes)
		visualWidth := lipgloss.Width(line)
		if visualWidth <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Need to wrap this line
		words := strings.Fields(line)
		if len(words) == 0 {
			result.WriteString("\n")
			continue
		}

		currentLine := strings.Builder{}
		currentWidth := 0

		for i, word := range words {
			wordWidth := lipgloss.Width(word)

			// If a single word is longer than width, truncate it on a rune
			// boundary (byte slicing here would split a multibyte rune).
			if wordWidth > width && currentWidth == 0 {
				result.WriteString(TruncateRunes(word, width))
				result.WriteString("\n")
				continue
			}

			if currentWidth > 0 {
				// Check if adding this word would exceed width
				if currentWidth+1+wordWidth > width {
					// Write current line and start new one
					result.WriteString(currentLine.String())
					result.WriteString("\n")
					currentLine.Reset()
					currentWidth = 0
				} else {
					// Add space before word
					currentLine.WriteString(" ")
					currentWidth += 1
				}
			}

			currentLine.WriteString(word)
			currentWidth += wordWidth

			// If this is the last word, write the line
			if i == len(words)-1 {
				result.WriteString(currentLine.String())
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

// TruncateRunes shortens word to at most width display columns, breaking on a
// rune boundary and appending an ellipsis when there is room for it.
func TruncateRunes(word string, width int) string {
	runes := []rune(word)
	if width <= 0 {
		return ""
	}
	if len(runes) <= width {
		return word
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

// ShortID truncates an id to a compact display form (an 8-character prefix).
// Shared by tui (tool/job labels) and every Presenter (agent-tagged tool
// labels), so the two never drift apart.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
