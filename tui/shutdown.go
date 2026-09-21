package tui

import (
	"time"

	"github.com/mudler/nib/internal/proc"
)

// shellShutdownTimeout bounds how long Shutdown waits for shell jobs. It is
// past proc.KillGrace, so a job that ignores SIGTERM gets its SIGKILL before
// nib exits.
const shellShutdownTimeout = proc.KillGrace + time.Second

// Shutdown stops the work that would otherwise outlive the program, and saves
// a session the program did not save itself. RunTUI calls it once the program
// has exited.
//
// quit() saves and closes the session, but an external SIGINT or SIGTERM
// unwinds the program without it, and the session went unsaved. Detached
// sub-agents and shell jobs do not stop with a turn: they used to die only
// with the process, and a shell job's children could outlive nib.
func (m Model) Shutdown() {
	if m.session != nil {
		m.session.StopAgents()
		if !m.quitting {
			m.recordSession()
			_ = m.session.Close()
		}
	}
	m.shellJobs.Shutdown(shellShutdownTimeout)
}
