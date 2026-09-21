package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mudler/nib/internal/textdiff"
	"github.com/mudler/nib/theme"
)

func TestDiffRowsTintFullWidth(t *testing.T) {
	d := textdiff.Compute("a\nb\nc\n", "a\nB\nc\n", 1)
	const w = 30
	for _, row := range DiffRows(d, w, 0) {
		plain := ansi.Strip(row)
		if strings.Contains(plain, "+ ") || strings.Contains(plain, "- ") {
			if got := lipgloss.Width(row); got != w {
				t.Errorf("changed row %q is %d cells wide, want %d", plain, got, w)
			}
		}
	}
}

func TestDiffRowsSignsAndNumbers(t *testing.T) {
	d := textdiff.Compute("x\nold\ny\n", "x\nnew\ny\n", 1)
	rows := DiffRows(d, 40, 0)
	var plain []string
	for _, r := range rows {
		plain = append(plain, strings.TrimRight(ansi.Strip(r), " "))
	}
	want := []string{"1   x", "2 - old", "2 + new", "3   y"}
	if strings.Join(plain, "|") != strings.Join(want, "|") {
		t.Fatalf("rows = %q, want %q", plain, want)
	}
}

func TestDiffRowsFragmentHasNoNumberGutter(t *testing.T) {
	rows := DiffRows(textdiff.Fragment("a\n", "b\n"), 20, 0)
	if got := ansi.Strip(rows[0]); !strings.HasPrefix(got, "- a") {
		t.Fatalf("fragment row = %q, want no number gutter", got)
	}
}

func TestDiffRowsFoldsPastMax(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("line\n")
	}
	d := textdiff.Compute("", b.String(), 0)
	rows := DiffRows(d, 40, 10)
	if len(rows) != 10 {
		t.Fatalf("got %d rows, want 10 (9 lines + fold)", len(rows))
	}
	if last := ansi.Strip(rows[9]); !strings.Contains(last, "21 more lines") {
		t.Fatalf("fold line = %q, want it to count the 21 hidden rows", last)
	}
}

func TestDiffRowsExpandsTabs(t *testing.T) {
	rows := DiffRows(textdiff.Compute("", "\tx\n", 0), 20, 0)
	if strings.Contains(rows[0], "\t") {
		t.Fatalf("row kept a tab: %q", rows[0])
	}
}

func TestDiffStat(t *testing.T) {
	for _, c := range []struct {
		d    textdiff.Diff
		want string
	}{
		{textdiff.Diff{Added: 3, Removed: 1}, "+3 -1"},
		{textdiff.Diff{Added: 7}, "+7"},
		{textdiff.Diff{Removed: 2}, "-2"},
	} {
		if got := DiffStat(c.d); got != c.want {
			t.Errorf("DiffStat(%+v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestToolHeaderMarksStatus(t *testing.T) {
	for _, c := range []struct {
		status ToolStatus
		mark   string
	}{
		{ToolStatusOK, theme.Check},
		{ToolStatusFailed, theme.Cross},
		{ToolStatusUnknown, theme.Sep},
	} {
		got := ansi.Strip(ToolHeader("edit main.go", "+1", "", c.status, 80))
		if want := c.mark + " edit main.go  +1"; got != want {
			t.Errorf("status %v: header = %q, want %q", c.status, got, want)
		}
	}
}

func TestToolHeaderTruncatesToWidth(t *testing.T) {
	got := ToolHeader("$ "+strings.Repeat("x", 200), "exit 1", "", ToolStatusFailed, 40)
	if w := lipgloss.Width(got); w > 40 {
		t.Fatalf("header is %d cells, want at most 40", w)
	}
	if !strings.HasSuffix(ansi.Strip(got), "exit 1") {
		t.Fatalf("truncation dropped the meta: %q", ansi.Strip(got))
	}
}

func TestToolBlockPrefersDiffOverContent(t *testing.T) {
	d := textdiff.Compute("a\n", "b\n", 0)
	out := ansi.Strip(ToolBlock(Message{Role: RoleTool, Label: "edit f", Content: "edited · 1 replacement", Diff: &d}, 40))
	if strings.Contains(out, "replacement") || !strings.Contains(out, "+ b") {
		t.Fatalf("block = %q, want the diff instead of the content", out)
	}
}

func TestToolBlockHeaderOnlyWhenNoBody(t *testing.T) {
	out := ToolBlock(Message{Role: RoleTool, Label: "read main.go", Meta: "11 lines", Status: ToolStatusOK}, 40)
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("block = %q, want the header line alone", out)
	}
}

func TestTruncateLeft(t *testing.T) {
	if got := truncateLeft("/very/long/path/to/work", 10); got != "…h/to/work" {
		t.Fatalf("truncateLeft = %q", got)
	}
	if got := truncateLeft("~/work", 10); got != "~/work" {
		t.Fatalf("short path changed: %q", got)
	}
}
