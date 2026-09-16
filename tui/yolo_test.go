package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
)

// TestDispatchYoloTogglesBareForm verifies bare "/yolo" flips the session's
// current state and reports the new one, rather than blindly setting it —
// important because "/yolo" can also appear inside /loop payloads and
// scripts, where a blind flip each tick would be ambiguous.
func TestDispatchYoloTogglesBareForm(t *testing.T) {
	m := newQueueTestModel()
	m.session = &chat.Session{}

	if m.session.AutoApprove() {
		t.Fatal("fixture assumption: session starts with auto-approve off")
	}

	if cmd := m.dispatchResolved("/yolo"); cmd != nil {
		t.Fatal("/yolo must not start a turn")
	}
	if !m.session.AutoApprove() {
		t.Fatal("bare /yolo should have turned auto-approve on")
	}
	msg := lastMessage(t, m)
	if msg.Content != theme.YoloOn {
		t.Fatalf("notice = %q, want %q", msg.Content, theme.YoloOn)
	}

	// Toggling again flips it back off.
	if cmd := m.dispatchResolved("/yolo"); cmd != nil {
		t.Fatal("/yolo must not start a turn")
	}
	if m.session.AutoApprove() {
		t.Fatal("second bare /yolo should have turned auto-approve back off")
	}
	if msg := lastMessage(t, m); msg.Content != theme.YoloOff {
		t.Fatalf("notice = %q, want %q", msg.Content, theme.YoloOff)
	}
}

// TestDispatchYoloExplicitForms verifies "/yolo on" and "/yolo off" set the
// state explicitly rather than toggling it.
func TestDispatchYoloExplicitForms(t *testing.T) {
	m := newQueueTestModel()
	m.session = &chat.Session{}

	m.dispatchResolved("/yolo on")
	if !m.session.AutoApprove() {
		t.Fatal("/yolo on should turn auto-approve on")
	}
	// Idempotent: "on" again while already on stays on.
	m.dispatchResolved("/yolo on")
	if !m.session.AutoApprove() {
		t.Fatal("/yolo on should remain on")
	}

	m.dispatchResolved("/yolo off")
	if m.session.AutoApprove() {
		t.Fatal("/yolo off should turn auto-approve off")
	}
}

// TestHeaderBadgeReadsLiveSessionState guards against the badge reading a
// startup-only snapshot (m.cfg.ApprovalMode, which never changes after
// launch) instead of the session's live AutoApprove state.
func TestHeaderBadgeReadsLiveSessionState(t *testing.T) {
	m := newQueueTestModel()

	// No session yet (header renders before the session is ready): must not
	// panic and must read as off.
	if vs := m.viewState(); vs.AutoApprove {
		t.Fatal("AutoApprove should be false with no session")
	}

	m.session = &chat.Session{}
	if vs := m.viewState(); vs.AutoApprove {
		t.Fatal("AutoApprove should be false before the toggle")
	}

	m.session.SetAutoApprove(true)
	if vs := m.viewState(); !vs.AutoApprove {
		t.Fatal("AutoApprove should be true after SetAutoApprove(true)")
	}

	m.session.SetAutoApprove(false)
	if vs := m.viewState(); vs.AutoApprove {
		t.Fatal("AutoApprove should be false after SetAutoApprove(false)")
	}
}

// TestApprovalKeyFourEnablesSessionWideApproval verifies the "[4] yes to
// everything this session" approval option both approves the pending call and
// flips the session into auto-approve for everything after it.
func TestApprovalKeyFourEnablesSessionWideApproval(t *testing.T) {
	ch := make(chan chat.ToolCallResponse, 1)
	s := &chat.Session{}
	m := newTestModel(Model{
		textarea:         textarea.New(),
		viewport:         viewport.New(80, 10),
		awaitingApproval: true,
		pendingTool:      &chat.ToolCallRequest{Name: "bash", Arguments: `{"script":"git status"}`},
		toolResponseChan: ch,
		session:          s,
	})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if cmd != nil {
		cmd()
	}
	if next.(Model).awaitingApproval {
		t.Fatal("4 should clear awaitingApproval")
	}
	resp := <-ch
	if !resp.Approved {
		t.Fatalf("4 should approve the pending call, got %+v", resp)
	}
	if !s.AutoApprove() {
		t.Fatal("4 should have turned session-wide auto-approve on")
	}
}
