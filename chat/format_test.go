package chat

import (
	"fmt"
	"strings"
	"testing"
)

func TestPreviewResult(t *testing.T) {
	t.Run("renders a known tool's result and truncates it", func(t *testing.T) {
		// 20 grep matches: fmtGrepResult renders one "path:line:content" row
		// per match, so this is 20 lines before truncation.
		var matches []string
		for i := 0; i < 20; i++ {
			matches = append(matches, fmt.Sprintf(`"f%d.go:%d:x"`, i, i))
		}
		result := `{"matches":[` + strings.Join(matches, ",") + `]}`

		out := PreviewResult("grep", result, 5)
		lines := strings.Split(out, "\n")
		if len(lines) != 6 {
			t.Fatalf("expected 6 lines (5 + note), got %d:\n%s", len(lines), out)
		}
		if !strings.Contains(lines[0], "f0.go:0") {
			t.Fatalf("expected a readable path:line row, got %q", lines[0])
		}
		if strings.Contains(out, `"matches"`) {
			t.Fatalf("expected readable rows, not raw JSON: %q", out)
		}
		note := lines[len(lines)-1]
		// 20 total lines, 5 kept -> 15 cut.
		if note != "… 15 more lines" {
			t.Fatalf("unexpected truncation note: %q", note)
		}
	})

	t.Run("short plain string returned unchanged", func(t *testing.T) {
		in := "line one\nline two"
		if got := PreviewResult("read", in, 12); got != in {
			t.Fatalf("expected %q unchanged, got %q", in, got)
		}
	})

	t.Run("single line truncation uses singular noun", func(t *testing.T) {
		in := "a\nb\nc"
		out := PreviewResult("read", in, 2)
		lines := strings.Split(out, "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d: %q", len(lines), out)
		}
		if lines[len(lines)-1] != "… 1 more line" {
			t.Fatalf("expected singular 'line' note, got %q", lines[len(lines)-1])
		}
	})

	t.Run("empty and whitespace return empty", func(t *testing.T) {
		if got := PreviewResult("read", "", 12); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
		if got := PreviewResult("read", "   \n\t  ", 12); got != "" {
			t.Fatalf("expected empty for whitespace, got %q", got)
		}
	})
}
