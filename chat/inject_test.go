package chat

import (
	"context"
	"strings"
	"testing"

	wizmcp "github.com/mudler/nib/mcp"
	"github.com/mudler/nib/types"
)

func TestInjectRequiresLiveRun(t *testing.T) {
	s, err := NewSession(context.Background(), types.Config{}, Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	// No live run: Inject is a no-op and reports false.
	if s.Inject("hello") {
		t.Fatal("Inject should return false when no run is live")
	}

	// Empty/whitespace input is always rejected.
	if s.Inject("   ") {
		t.Fatal("Inject should reject blank input")
	}

	// With a live run it succeeds and delivers the message into the channel.
	s.runMu.Lock()
	s.runLive = true
	s.runMu.Unlock()
	if !s.Inject("hello") {
		t.Fatal("Inject should succeed against a live run")
	}
	select {
	case msg := <-s.inject:
		if msg.Content != "hello" || msg.Role != "user" {
			t.Fatalf("injected message = %+v, want user/hello", msg)
		}
	default:
		t.Fatal("expected the injected message on the channel")
	}
}

func TestRunLiveReflectsRunState(t *testing.T) {
	s, err := NewSession(context.Background(), types.Config{}, Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	if s.RunLive() {
		t.Fatal("RunLive should be false before any run")
	}
	s.runMu.Lock()
	s.runLive = true
	s.runMu.Unlock()
	if !s.RunLive() {
		t.Fatal("RunLive should be true while a run is live")
	}
	s.runMu.Lock()
	s.runLive = false
	s.runMu.Unlock()
	if s.RunLive() {
		t.Fatal("RunLive should be false after the run ends")
	}
}

func TestSetShellJobsNilSafe(t *testing.T) {
	s, err := NewSession(context.Background(), types.Config{}, Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	// nil registry must not panic and leaves the pending-work predicate harmless.
	s.SetShellJobs(nil)

	// A real registry wires the completion hook; with no live run the injected
	// notice is simply dropped (Inject returns false), which must not panic.
	jobs := wizmcp.NewShellJobs()
	s.SetShellJobs(jobs)
	if s.shellJobs == nil {
		t.Fatal("SetShellJobs should store the registry")
	}
}

// A background job can finish while no run is live: after an interrupt ended
// the run, or between turns. Inject has no run to deliver to, and the model
// used to never learn the job finished. The notice now waits for the next turn.
func TestNoticeWithNoLiveRunReachesTheNextTurn(t *testing.T) {
	s := newWarmTestSession(t)
	llm := s.llm.(*captureLLM)

	s.deliverNotice("shell job sh-1 done:\nbuild ok")

	if _, err := s.SendMessage("what happened?"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	var sawNotice bool
	var userAfterNotice bool
	for _, m := range llm.lastRequest().Messages {
		if strings.Contains(m.Content, "shell job sh-1 done") {
			sawNotice = true
			continue
		}
		if sawNotice && m.Role == "user" && m.Content == "what happened?" {
			userAfterNotice = true
		}
	}
	if !sawNotice {
		t.Fatal("the buffered notice never reached the model")
	}
	if !userAfterNotice {
		t.Fatal("the notice must come before the user's message")
	}

	// Delivered once: the next turn does not repeat it.
	if _, err := s.SendMessage("again"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	n := 0
	for _, m := range llm.lastRequest().Messages {
		if strings.Contains(m.Content, "shell job sh-1 done") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("notice appears %d times in the second turn's history, want 1", n)
	}
}

// A notice that arrives while a run is live goes straight into that run.
func TestNoticeWithALiveRunIsInjected(t *testing.T) {
	s, err := NewSession(context.Background(), types.Config{}, Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()
	s.runMu.Lock()
	s.runLive = true
	s.runMu.Unlock()

	s.deliverNotice("shell job sh-2 done")

	select {
	case msg := <-s.inject:
		if msg.Content != "shell job sh-2 done" {
			t.Fatalf("injected %q", msg.Content)
		}
	default:
		t.Fatal("a notice for a live run was not injected")
	}
	if n := len(s.takePendingNotices()); n != 0 {
		t.Fatalf("%d notices buffered although the run was live", n)
	}
}
