package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/mudler/cogito"
	openai "github.com/sashabaranov/go-openai"
)

func newStaleTestSession(live bool) *Session {
	s := &Session{
		agentLogs: newAgentLogStore(),
		inject:    make(chan openai.ChatCompletionMessage, 8),
	}
	s.runLive = live
	return s
}

func drainInjected(s *Session) []string {
	var out []string
	for {
		select {
		case m := <-s.inject:
			out = append(out, m.Content)
		default:
			return out
		}
	}
}

func TestStaleAgentNotifiedOncePerStall(t *testing.T) {
	s := newStaleTestSession(true)
	t0 := time.Now()
	s.agentLogs.started("a1", t0)

	s.notifyStaleAgents(t0.Add(staleAgentAfter - time.Second))
	if got := drainInjected(s); len(got) != 0 {
		t.Fatalf("notified before the threshold: %v", got)
	}

	s.notifyStaleAgents(t0.Add(staleAgentAfter + time.Second))
	got := drainInjected(s)
	if len(got) != 1 || !strings.Contains(got[0], "a1") {
		t.Fatalf("notices = %v, want one about a1", got)
	}

	s.notifyStaleAgents(t0.Add(2 * staleAgentAfter))
	if got := drainInjected(s); len(got) != 0 {
		t.Fatalf("the same stall was reported again: %v", got)
	}
}

func TestStaleAgentActivityRearmsNotice(t *testing.T) {
	s := newStaleTestSession(true)
	s.agentLogs.started("a1", time.Now().Add(-2*staleAgentAfter))
	s.notifyStaleAgents(time.Now())
	drainInjected(s)

	// A tool call is activity: the agent is no longer stale...
	s.agentLogs.recordCall("a1", &cogito.ToolChoice{ID: "c1", Name: "bash"})
	s.notifyStaleAgents(time.Now())
	if got := drainInjected(s); len(got) != 0 {
		t.Fatalf("an active agent was reported: %v", got)
	}
	// ...and a later stall is reported again.
	s.notifyStaleAgents(time.Now().Add(staleAgentAfter + time.Second))
	if got := drainInjected(s); len(got) != 1 {
		t.Fatalf("notices = %v, want the new stall reported", got)
	}
}

func TestFinishedAgentIsNeverStale(t *testing.T) {
	s := newStaleTestSession(true)
	s.agentLogs.started("a1", time.Now().Add(-2*staleAgentAfter))
	s.agentLogs.forget("a1")
	s.notifyStaleAgents(time.Now())
	if got := drainInjected(s); len(got) != 0 {
		t.Fatalf("a finished agent was reported: %v", got)
	}
}

func TestStaleNoticeRetriedWhenRunNotLive(t *testing.T) {
	s := newStaleTestSession(false)
	s.agentLogs.started("a1", time.Now().Add(-2*staleAgentAfter))
	s.notifyStaleAgents(time.Now()) // Inject fails: no live run

	s.runLive = true
	s.notifyStaleAgents(time.Now())
	if got := drainInjected(s); len(got) != 1 {
		t.Fatalf("notices = %v; an undelivered notice must be retried", got)
	}
}
