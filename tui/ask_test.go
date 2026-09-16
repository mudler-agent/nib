package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

func TestRenderAskAndParse(t *testing.T) {
	req := chat.AskRequest{Question: "Pick one", Options: []string{"alpha", "beta"}}
	list := &render.SelectList{Items: req.Options}
	d := buildAskDialog(req, list, false)
	if d.Title != "Pick one" || len(d.Options) != 2 || d.Options[0].Text != "alpha" || d.Options[1].Text != "beta" {
		t.Fatalf("ask dialog missing question/options: %+v", d)
	}
	if got := parseAskAnswer("2", req); got != "beta" {
		t.Fatalf("numeric pick: %q", got)
	}
	if got := parseAskAnswer("something else", req); got != "something else" {
		t.Fatalf("free text: %q", got)
	}
	if got := parseAskAnswer("9", req); got != "9" {
		t.Fatalf("out-of-range should be verbatim: %q", got)
	}
	if got := parseAskAnswer("hi", chat.AskRequest{Question: "q"}); got != "hi" {
		t.Fatalf("no-options verbatim: %q", got)
	}
	// Single-select shows a radio marker via the presenter.
	out := testPresenter().Dialog(d, 80)
	if !strings.Contains(out, theme.RadioOff) {
		t.Fatalf("single-select should show a radio marker:\n%s", out)
	}
	// The hint line beneath the options still tells the user the free-text
	// escape hatch is available — the fact the old renderAsk's "type a number
	// to pick, or type your own answer" hint asserted, now carried by
	// buildAskDialog's Hint field and rendered by the presenter.
	if d.Hint != theme.AskHintSingleSelect {
		t.Fatalf("single-select Hint = %q, want %q", d.Hint, theme.AskHintSingleSelect)
	}
	if !strings.Contains(out, theme.AskHintSingleSelect) {
		t.Fatalf("single-select dialog should render its hint line:\n%s", out)
	}
}

func TestAskMultiSelect(t *testing.T) {
	req := chat.AskRequest{
		Question:    "Pick some",
		Options:     []string{"red", "green", "blue"},
		MultiSelect: true,
	}
	list := &render.SelectList{Items: req.Options, MultiSelect: true, Checked: make([]bool, 3)}
	d := buildAskDialog(req, list, false)
	out := testPresenter().Dialog(d, 80)
	if !strings.Contains(out, theme.CheckOff) {
		t.Fatalf("multi-select render missing checkbox:\n%s", out)
	}
	// Restores the old renderAsk assertion that the multi-select hint (once
	// literally "type numbers separated by commas...") still renders — now
	// theme.AskHintMultiSelect, carried by buildAskDialog and placed by the
	// presenter beneath the option rows.
	if d.Hint != theme.AskHintMultiSelect {
		t.Fatalf("multi-select Hint = %q, want %q", d.Hint, theme.AskHintMultiSelect)
	}
	if !strings.Contains(out, theme.AskHintMultiSelect) {
		t.Fatalf("multi-select dialog should render its hint line:\n%s", out)
	}
	if got := parseAskAnswer("1,3", req); got != "red, blue" {
		t.Fatalf("comma indices: %q", got)
	}
	if got := parseAskAnswer("2 3", req); got != "green, blue" {
		t.Fatalf("space indices: %q", got)
	}
	if got := parseAskAnswer("2", req); got != "green" {
		t.Fatalf("single index in multi: %q", got)
	}
	if got := parseAskAnswer("1,9", req); got != "1,9" {
		t.Fatalf("invalid index should be verbatim: %q", got)
	}
	if got := parseAskAnswer("my own answer", req); got != "my own answer" {
		t.Fatalf("free text in multi: %q", got)
	}
}

// TestBuildAskDialogBlockedByApprovalHint: a background sub-agent's tool
// approval and a foreground ask_user question can both be pending at once
// (cogito propagates the tool-call callback into spawned sub-agents — see
// chat/session.go), and the approval's key-driven choice mode swallows every
// keypress but its own until it resolves. When that's the case, the ask
// dialog's hint must say so instead of advertising affordances (arrows,
// typing) that currently do nothing — for both single- and multi-select,
// since the approval blocks either one identically.
func TestBuildAskDialogBlockedByApprovalHint(t *testing.T) {
	single := chat.AskRequest{Question: "which?", Options: []string{"a", "b"}}
	multi := chat.AskRequest{Question: "which?", Options: []string{"a", "b"}, MultiSelect: true}

	if got := buildAskDialog(single, &render.SelectList{Items: single.Options}, true).Hint; got != theme.AskBlockedByApproval {
		t.Errorf("single-select blocked Hint = %q, want %q", got, theme.AskBlockedByApproval)
	}
	if got := buildAskDialog(multi, &render.SelectList{Items: multi.Options, MultiSelect: true, Checked: make([]bool, 2)}, true).Hint; got != theme.AskBlockedByApproval {
		t.Errorf("multi-select blocked Hint = %q, want %q", got, theme.AskBlockedByApproval)
	}
	// Unblocked, the normal per-mode hints are unaffected.
	if got := buildAskDialog(single, &render.SelectList{Items: single.Options}, false).Hint; got != theme.AskHintSingleSelect {
		t.Errorf("unblocked single-select Hint = %q, want %q", got, theme.AskHintSingleSelect)
	}
}

// TestAskDialogBlockedWhileApprovalPending pins the precedence as documented
// behaviour rather than a silent dead end: while an unrelated tool approval
// is pending, the ask dialog's own keys (up/down/space) do nothing at all —
// the approval's choice-mode block runs first in Update's key switch and
// swallows every key but its own (and Ctrl+C) — so none of the SelectList
// state changes and no answer is ever sent. The approval itself is still
// answerable in the same keystroke.
func TestAskDialogBlockedWhileApprovalPending(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:         textarea.New(),
		viewport:         viewport.New(80, 20),
		width:            80,
		awaitingApproval: true,
		pendingTool:      &chat.ToolCallRequest{Name: "ask_user"},
		toolResponseChan: make(chan chat.ToolCallResponse, 1),
		awaitingAsk:      true,
		pendingAsk:       &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta", "gamma"}},
		askResponseChan:  respCh,
		presenter:        testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := next.(Model)
	if got.askList.Selected != 0 {
		t.Errorf("askList.Selected = %d, want 0 — Down must not reach the ask dialog while an approval is pending", got.askList.Selected)
	}
	if !got.awaitingApproval || !got.awaitingAsk {
		t.Fatalf("Down should not resolve either dialog: awaitingApproval=%v awaitingAsk=%v", got.awaitingApproval, got.awaitingAsk)
	}
	select {
	case v := <-respCh:
		t.Fatalf("ask was answered (%q) while an approval was still pending", v)
	default:
	}

	// The approval itself is still answerable in the same keystroke — this
	// isn't a fully dead key switch, just an ask dialog that has to wait its
	// turn (see buildAskDialog's blockedByApproval hint).
	next2, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if cmd != nil {
		cmd()
	}
	if next2.(Model).awaitingApproval {
		t.Error("'1' should have resolved the pending approval")
	}
}

// TestAskDialogKeyboardSelection: arrow keys move the selection and enter
// answers with the selected option — no number typing required.
func TestAskDialogKeyboardSelection(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:        textarea.New(),
		viewport:        viewport.New(80, 20),
		width:           80,
		awaitingAsk:     true,
		pendingAsk:      &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta", "gamma"}},
		askResponseChan: respCh,
		presenter:       testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, cmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	select {
	case got := <-respCh:
		if got != "beta" {
			t.Errorf("answer = %q, want %q", got, "beta")
		}
	default:
		t.Fatal("enter did not answer the question")
	}
	if next.(Model).awaitingAsk {
		t.Error("awaitingAsk still set after answering")
	}
}

// TestAskDialogFreeTextStillWorks preserves the existing escape hatch: typing
// an answer instead of picking one must still send that text.
func TestAskDialogFreeTextStillWorks(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:        textarea.New(),
		viewport:        viewport.New(80, 20),
		width:           80,
		awaitingAsk:     true,
		pendingAsk:      &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta"}},
		askResponseChan: respCh,
		presenter:       testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options}
	// bubbles' textarea silently drops key input while unfocused; production
	// code focuses it in the askMsg branch (tui/model.go) before the user can
	// ever type, which this direct Model literal bypasses.
	m.textarea.Focus()

	var cur tea.Model = m
	for _, r := range "custom" {
		cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := cur.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	select {
	case got := <-respCh:
		if got != "custom" {
			t.Errorf("answer = %q, want %q", got, "custom")
		}
	default:
		t.Fatal("free-text answer was not sent")
	}
}

func TestAskDialogMultiSelectTogglesWithSpace(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:        textarea.New(),
		viewport:        viewport.New(80, 20),
		width:           80,
		awaitingAsk:     true,
		pendingAsk:      &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta", "gamma"}, MultiSelect: true},
		askResponseChan: respCh,
		presenter:       testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options, MultiSelect: true, Checked: make([]bool, 3)}

	var cur tea.Model = m
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeySpace}) // check alpha
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeySpace}) // check gamma
	_, cmd := cur.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	select {
	case got := <-respCh:
		if got != "alpha, gamma" {
			t.Errorf("answer = %q, want %q", got, "alpha, gamma")
		}
	default:
		t.Fatal("multi-select answer was not sent")
	}
}
