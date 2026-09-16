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
	if _, ok := goalFooterRow(""); ok {
		t.Fatal("no goal should report nothing to show")
	}
	row, ok := goalFooterRow("make all tests pass")
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
