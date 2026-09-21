package mcp

import "time"

// Shutdown cancels every running shell job and waits up to timeout for them to
// exit. It returns how many were still running when it gave up.
//
// nib calls it on quit, before the process exits. Cancelling the app context
// alone was not enough: the jobs were signalled, but nib exited before they
// could stop or be sent the follow-up SIGKILL, and some of them kept running.
func (s *ShellJobs) Shutdown(timeout time.Duration) int {
	if s == nil {
		return 0
	}
	s.mgr.mu.Lock()
	var running []*bgJob
	for _, j := range s.mgr.jobs {
		if done, _, _ := j.snapshot(); !done {
			running = append(running, j)
		}
	}
	s.mgr.mu.Unlock()

	for _, j := range running {
		j.cancel()
	}
	deadline := time.After(timeout)
	left := len(running)
	for _, j := range running {
		select {
		case <-j.doneCh:
			left--
		case <-deadline:
			return left
		}
	}
	return 0
}
