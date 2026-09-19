package tui

import (
	"context"
	"testing"
	"time"

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

// /yolo typed while a run is in flight must take effect at once. It used to
// be queued like a follow-up message, and queued slash commands only run when
// the run ends — so every tool call of the very run the user was trying to
// unblock kept prompting.
func TestYoloMidRunAppliesImmediately(t *testing.T) {
	for _, state := range []string{"loading", "parked"} {
		t.Run(state, func(t *testing.T) {
			m := newQueueTestModel()
			m.session = &chat.Session{}
			if state == "loading" {
				m.loading = true
			} else {
				m.parked = true
			}
			m.textarea.SetValue("/yolo on")

			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			nm := next.(Model)

			if !nm.session.AutoApprove() {
				t.Fatal("/yolo typed mid-run was not applied")
			}
			if len(nm.queue) != 0 {
				t.Fatalf("queue = %v, want /yolo applied rather than queued", nm.queue)
			}
		})
	}
}

// /yolo typed into the approval prompt answers it: yolo turns on and the
// pending call is approved. It used to be sent to the model as an
// "adjustment" to the tool call.
func TestYoloDuringApprovalApprovesAndEnables(t *testing.T) {
	m := newQueueTestModel()
	m.session = &chat.Session{}
	m.loading = true
	m.awaitingApproval = true
	m.approvalEditing = true
	m.toolResponseChan = make(chan chat.ToolCallResponse, 1)
	m.textarea.SetValue("/yolo")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nm := next.(Model)
	if cmd != nil {
		cmd()
	}

	if !nm.session.AutoApprove() {
		t.Fatal("/yolo in the approval prompt did not turn auto-approve on")
	}
	if nm.awaitingApproval {
		t.Fatal("the pending approval was left open")
	}
	select {
	case resp := <-m.toolResponseChan:
		if !resp.Approved || resp.Adjustment != "" {
			t.Fatalf("response = %+v, want a plain approval", resp)
		}
	default:
		t.Fatal("no response was sent to the waiting tool call")
	}
}

// /resume rebuilds the session from m.cfg, whose approval_mode is a startup
// snapshot. A /yolo turned on at runtime must survive that rebuild rather than
// silently switching prompting back on.
func TestYoloSurvivesResume(t *testing.T) {
	m := newQueueTestModel()
	m.session = &chat.Session{}
	m.session.SetAutoApprove(true)

	m.applyResume(chat.SessionRecord{ID: "r1"})
	next, _ := m.Update(sessionReadyMsg{session: &chat.Session{}})

	if !next.(Model).session.AutoApprove() {
		t.Fatal("auto-approve was lost across /resume")
	}
}

// A tool call that was already waiting when yolo turned on — a parallel call,
// or a sub-agent's — must not raise a prompt yolo would not have raised.
func TestToolRequestUnderYoloIsApprovedWithoutPrompt(t *testing.T) {
	m := newQueueTestModel()
	m.session = &chat.Session{}
	m.session.SetAutoApprove(true)
	m.ctx = context.Background()
	m.toolResponseChan = make(chan chat.ToolCallResponse, 1)
	m.toolRequestChan = make(chan chat.ToolCallRequest)

	next, cmd := m.Update(toolCallMsg(chat.ToolCallRequest{Name: "bash"}))
	if next.(Model).awaitingApproval {
		t.Fatal("a prompt was raised with yolo on")
	}
	runCmds(cmd, m.toolRequestChan)
	select {
	case resp := <-m.toolResponseChan:
		if !resp.Approved {
			t.Fatalf("response = %+v, want approved", resp)
		}
	default:
		t.Fatal("the waiting call was never answered")
	}
}

// Approval requests are shown one at a time: the next one is read only after
// the current one is answered. Reading it early overwrote pendingTool, and
// with both callers blocked on the one response channel an answer could go to
// the wrong call.
func TestNextToolRequestIsReadOnlyAfterTheAnswer(t *testing.T) {
	m := newQueueTestModel()
	m.session = &chat.Session{}
	m.ctx = context.Background()
	m.toolResponseChan = make(chan chat.ToolCallResponse, 1)
	m.toolRequestChan = make(chan chat.ToolCallRequest, 1)
	m.toolRequestChan <- chat.ToolCallRequest{Name: "second"}

	next, cmd := m.Update(toolCallMsg(chat.ToolCallRequest{Name: "first"}))
	runCmds(cmd, nil)
	if len(m.toolRequestChan) != 1 {
		t.Fatal("the second request was read while the first was still on screen")
	}
	nm := next.(Model)
	if nm.pendingTool == nil || nm.pendingTool.Name != "first" {
		t.Fatalf("pendingTool = %+v, want the first request", nm.pendingTool)
	}

	nm2, cmd := nm.resolveApproval(chat.ToolCallResponse{Approved: true})
	if len(m.toolRequestChan) != 1 {
		t.Fatal("the second request was read before the answer was delivered")
	}
	answered := cmd()
	if resp := <-m.toolResponseChan; !resp.Approved {
		t.Fatalf("response = %+v", resp)
	}
	_, listen := nm2.(Model).Update(answered)
	msg := listen()
	if batch, ok := msg.(tea.BatchMsg); ok && len(batch) == 1 {
		msg = batch[0]()
	}
	if got, ok := msg.(toolCallMsg); !ok || got.Name != "second" {
		t.Fatalf("after answering, got %#v, want the second request", msg)
	}
}

// runCmds runs cmd and any batch it holds, skipping commands that would block
// reading reqs (the re-armed request listener).
func runCmds(cmd tea.Cmd, reqs chan chat.ToolCallRequest) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				runCmds(c, reqs)
			}
		}
	case <-time.After(100 * time.Millisecond):
		// Blocked on a channel read: a listener, which is what we expect when
		// nothing is queued.
	}
}
