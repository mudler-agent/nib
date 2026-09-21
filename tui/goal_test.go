package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/tui/render"
	"github.com/mudler/nib/types"
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

// newGoalModel is a ctrl-c test model with a real (never-run) session.
func newGoalModel(t *testing.T) Model {
	t.Helper()
	s, err := chat.NewSession(context.Background(), types.Config{}, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	m := newCtrlCModel()
	m.session = s
	return m
}

// Setting a goal starts a turn at once, so the agent reacts to it instead of
// waiting for another message. Before, the goal sat idle until the user typed
// something, and even then the model did not see the goal text until it
// first stopped and got the reminder.
func TestGoalSetStartsATurn(t *testing.T) {
	m := newGoalModel(t)
	cmd := m.dispatchInput("/goal make all tests pass")
	if cmd == nil || !m.loading {
		t.Fatalf("cmd=%v loading=%v, want /goal to start a turn", cmd, m.loading)
	}
	if got := m.session.Goal(); got != "make all tests pass" {
		t.Fatalf("goal = %q", got)
	}
}

// Resuming a paused goal also starts a turn.
func TestGoalResumeStartsATurn(t *testing.T) {
	m := newGoalModel(t)
	m.session.SetGoal("ship it")
	m.session.PauseGoal()
	if cmd := m.dispatchInput("/goal resume"); cmd == nil || !m.loading {
		t.Fatalf("cmd=%v loading=%v, want /goal resume to start a turn", cmd, m.loading)
	}
	if m.session.GoalPaused() {
		t.Fatal("goal still paused")
	}
}

// Showing or clearing a goal starts nothing.
func TestGoalShowAndClearStartNoTurn(t *testing.T) {
	m := newGoalModel(t)
	m.session.SetGoal("ship it")
	for _, in := range []string{"/goal", "/goal clear", "/goal resume"} {
		if cmd := m.dispatchInput(in); cmd != nil || m.loading {
			t.Fatalf("%s: cmd=%v loading=%v, want no turn", in, cmd, m.loading)
		}
	}
}
