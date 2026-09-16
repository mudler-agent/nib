package render

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// TestWrapTextMultibyteStaysValid moved verbatim from tui/model_test.go,
// renamed to call Wrap.
func TestWrapTextMultibyteStaysValid(t *testing.T) {
	long := strings.Repeat("é", 40) // one 2-byte rune, repeated, no spaces
	out := Wrap(long, 10)
	if !utf8.ValidString(out) {
		t.Fatalf("Wrap produced invalid UTF-8: %q", out)
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if lipgloss.Width(line) > 10 {
			t.Errorf("line %q exceeds width 10 (got %d)", line, lipgloss.Width(line))
		}
	}
}

// TestTruncateLine moved verbatim from tui/approval_test.go, renamed to call
// TruncateLine.
func TestTruncateLine(t *testing.T) {
	if got := TruncateLine("hello", 10); got != "hello" {
		t.Fatalf("fits: got %q", got)
	}
	if got := TruncateLine("hello world", 5); got != "hell…" {
		t.Fatalf("truncates: got %q", got)
	}
	if got := TruncateLine("hello", 0); got != "…" {
		t.Fatalf("non-positive budget must clamp, got %q", got)
	}
	if got := TruncateLine("héllo wörld", 6); got != "héllo…" {
		t.Fatalf("rune-aware: got %q", got)
	}
}

// TestWrapPreservesExistingBehaviour pins Wrap's contract post-move. Note:
// Wrap terminates every emitted line (including the last) with "\n" — this is
// the pre-existing, verbatim-preserved behaviour of the old wrapText, and
// callers throughout tui/model.go already strings.TrimRight the result before
// splitting it back into lines. The "want" values below reflect that real
// behaviour rather than a trimmed idealization of it.
func TestWrapPreservesExistingBehaviour(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"empty", "", 10, "\n"},
		{"short line untouched", "hello", 10, "hello\n"},
		{"wraps on word boundary", "hello world again", 11, "hello world\nagain\n"},
		{"zero width does not panic", "hello", 0, "hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Wrap(c.in, c.width); got != c.want {
				t.Errorf("Wrap(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
			}
		})
	}
}
