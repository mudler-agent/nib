package tui

import (
	"context"
	"strings"
	"testing"
)

// Turn-level errors are recorded in the transcript as transient lines: visible
// immediately, dropped once a reply arrives (the run recovered), and never
// pinned to the persistent banner.

func TestTransientErrorShownUntilReply(t *testing.T) {
	m := newQueueTestModel()
	m.width, m.height = 80, 24
	m.messages = []ChatMessage{
		{Role: "user", Content: "do the thing"},
		{Role: "error", Content: "tool selection failed: no content", Transient: true},
	}

	// The error is visible in the transcript while no reply has arrived.
	m.updateViewport()
	if out := m.View(); !strings.Contains(out, "tool selection failed: no content") {
		t.Fatalf("error line should be visible before a reply, got:\n%s", out)
	}
	// ...and it is not pinned to the persistent banner.
	if m.err != nil {
		t.Fatalf("turn-level error must not set the persistent banner, got %v", m.err)
	}

	// A reply arrives: the stale error line is dropped.
	next, _ := m.Update(responseMsg{content: "all done"})
	nm := next.(Model)
	for _, msg := range nm.messages {
		if msg.Role == "error" {
			t.Fatalf("stale transient error should be dropped after a reply, got %+v", msg)
		}
	}
	last := nm.messages[len(nm.messages)-1]
	if last.Role != "assistant" || last.Content != "all done" {
		t.Fatalf("last message = %+v, want assistant/all done", last)
	}
	nm.updateViewport()
	if out := nm.View(); strings.Contains(out, "tool selection failed") {
		t.Fatalf("error line should be gone from the view after a reply, got:\n%s", out)
	}
}

func TestTransientErrorKeptWithoutReply(t *testing.T) {
	m := newQueueTestModel()
	m.width, m.height = 80, 24
	m.messages = []ChatMessage{{Role: "error", Content: "tool selection failed: no content", Transient: true}}

	// An empty final message is not a reply: the error stays.
	next, _ := m.Update(responseMsg{content: "   "})
	nm := next.(Model)
	if len(nm.messages) != 1 || nm.messages[0].Role != "error" {
		t.Fatalf("error should persist without a reply, got %+v", nm.messages)
	}

	// An interrupt also clears the stale error (the run is no longer stuck).
	next, _ = nm.Update(responseMsg{err: context.Canceled})
	nm = next.(Model)
	for _, msg := range nm.messages {
		if msg.Role == "error" {
			t.Fatalf("stale error should be dropped after an interrupt, got %+v", msg)
		}
	}
	last := nm.messages[len(nm.messages)-1]
	if last.Role != "agent" || last.Content != "interrupted." {
		t.Fatalf("last message = %+v, want agent/interrupted.", last)
	}
}

func TestNonTransientErrorSurvivesReply(t *testing.T) {
	m := newQueueTestModel()
	m.width, m.height = 80, 24
	// A compaction failure is recorded as a plain (non-transient) error line.
	m.messages = []ChatMessage{{Role: "error", Content: "compaction failed: oops"}}

	next, _ := m.Update(responseMsg{content: "ok"})
	nm := next.(Model)
	if len(nm.messages) != 2 || nm.messages[0].Role != "error" || nm.messages[0].Content != "compaction failed: oops" {
		t.Fatalf("non-transient error must survive a reply, got %+v", nm.messages)
	}
}

func TestTransientErrorDroppedOnParkedReply(t *testing.T) {
	m := newQueueTestModel()
	m.width, m.height = 80, 24
	m.lastParkedReply = "parked answer"
	m.messages = []ChatMessage{
		{Role: "error", Content: "tool selection failed: no content", Transient: true},
		{Role: "assistant", Content: "parked answer"},
	}

	// The final reply duplicates the parked text and is skipped, but it is
	// still a reply: the stale error must be dropped.
	next, _ := m.Update(responseMsg{content: "parked answer"})
	nm := next.(Model)
	for _, msg := range nm.messages {
		if msg.Role == "error" {
			t.Fatalf("stale transient error should be dropped on a parked reply, got %+v", msg)
		}
	}
	if len(nm.messages) != 1 || nm.messages[0].Content != "parked answer" {
		t.Fatalf("parked reply should not be duplicated, got %+v", nm.messages)
	}
}

func TestParkedReplyDedupClearsAfterTerminalReply(t *testing.T) {
	m := newQueueTestModel()
	m.lastParkedReply = "same answer"
	m.messages = []ChatMessage{{Role: "assistant", Content: "same answer"}}

	next, _ := m.Update(responseMsg{content: "same answer"})
	nm := next.(Model)
	if len(nm.messages) != 1 {
		t.Fatalf("terminal parked reply should be deduped once, got %+v", nm.messages)
	}
	if nm.lastParkedReply != "" {
		t.Fatalf("parked reply marker should clear after response, got %q", nm.lastParkedReply)
	}

	next, _ = nm.Update(responseMsg{content: "same answer"})
	nm = next.(Model)
	if len(nm.messages) != 2 {
		t.Fatalf("later identical reply should not be skipped, got %+v", nm.messages)
	}
	last := nm.messages[len(nm.messages)-1]
	if last.Role != "assistant" || last.Content != "same answer" {
		t.Fatalf("last message = %+v, want assistant/same answer", last)
	}
}

func TestDropTransientErrorsIsNoopWithoutErrors(t *testing.T) {
	m := newQueueTestModel()
	m.messages = []ChatMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	m.dropTransientErrors()
	if len(m.messages) != 2 || m.messages[1].Content != "hello" {
		t.Fatalf("dropTransientErrors must not touch non-error lines, got %+v", m.messages)
	}
}
