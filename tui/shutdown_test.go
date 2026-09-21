package tui

import (
	"os"
	"testing"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/chat"
)

// An external SIGINT or SIGTERM unwinds the program without quit(), and the
// session used to go unsaved. Shutdown saves it.
func TestShutdownSavesASessionThatDidNotQuit(t *testing.T) {
	dir := t.TempDir()
	m := newQueueTestModel()
	m.session = newSpentSession(t)
	m.store = chat.NewSessionStore(dir)
	m.sessionID = "sig"

	m.Shutdown()

	cwd, _ := os.Getwd()
	recs, err := m.store.List(cwd)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 || recs[0].ID != "sig" {
		t.Fatalf("saved sessions = %+v, want the one Shutdown recorded", recs)
	}
}

// Detached sub-agents do not stop with a turn, and they used to die only with
// the process, mid-call. Shutdown cancels them.
func TestShutdownStopsSubAgents(t *testing.T) {
	m := newQueueTestModel()
	m.session = newSpentSession(t)
	m.quitting = true // quit() already saved and closed
	cancelled := false
	m.session.AgentManager().Register(&cogito.AgentState{ID: "bg", Status: cogito.AgentStatusRunning, Cancel: func() { cancelled = true }})

	m.Shutdown()
	if !cancelled {
		t.Fatal("Shutdown did not cancel the sub-agent")
	}
}
