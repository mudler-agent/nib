package tui

import (
	"strings"
	"testing"

	"github.com/mudler/nib/tui/render"
)

// TestGoalFooterRow exercises goalFooterRow, the live path View() actually
// calls (renderGoalFooter, the fully-styled string-returning function this
// test used to pin, was deleted once it had zero production call sites left
// — see the Task 6 fix-round-2 report).
func TestGoalFooterRow(t *testing.T) {
	if _, ok := goalFooterRow("", false); ok {
		t.Fatal("no goal should report nothing to show")
	}
	row, ok := goalFooterRow("make all tests pass", false)
	if !ok {
		t.Fatal("expected a row when a goal is set")
	}
	if !strings.Contains(row.Text, "make all tests pass") {
		t.Fatalf("row missing goal text: %q", row.Text)
	}
	if !strings.Contains(row.Text, "/goal clear") {
		t.Fatalf("row should hint how to clear: %q", row.Text)
	}
	if row.Kind != render.FooterGoal {
		t.Fatalf("expected FooterGoal kind, got %v", row.Kind)
	}
}

// A paused goal still shows, marked paused, with how to resume it.
func TestGoalFooterRowPaused(t *testing.T) {
	row, ok := goalFooterRow("ship it", true)
	if !ok {
		t.Fatal("a paused goal should still show")
	}
	for _, want := range []string{"paused", "ship it", "/goal resume", "/goal clear"} {
		if !strings.Contains(row.Text, want) {
			t.Fatalf("paused row %q does not contain %q", row.Text, want)
		}
	}
}
