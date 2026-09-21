package chat

import (
	"testing"

	"github.com/mudler/cogito"
)

func TestKillAgent(t *testing.T) {
	s := &Session{agentManager: cogito.NewAgentManager()}

	killed := false
	s.agentManager.Register(&cogito.AgentState{ID: "x", Cancel: func() { killed = true }})

	if !s.KillAgent("x") {
		t.Fatal("KillAgent should return true for a known agent")
	}
	if !killed {
		t.Fatal("KillAgent should call the agent's Cancel")
	}
	if s.KillAgent("nope") {
		t.Fatal("KillAgent should return false for an unknown agent")
	}
}

// On quit nothing cancelled detached sub-agents: they run on a context
// detached from the turn, so they died only when the process exited.
func TestStopAgentsCancelsEveryAgent(t *testing.T) {
	s := &Session{agentManager: cogito.NewAgentManager()}
	var a, b bool
	s.agentManager.Register(&cogito.AgentState{ID: "a", Cancel: func() { a = true }})
	s.agentManager.Register(&cogito.AgentState{ID: "b", Cancel: func() { b = true }})
	s.agentManager.Register(&cogito.AgentState{ID: "no-cancel"})

	s.StopAgents()
	if !a || !b {
		t.Fatalf("cancelled a=%v b=%v, want both", a, b)
	}
	(&Session{}).StopAgents() // no manager: must not panic
}
