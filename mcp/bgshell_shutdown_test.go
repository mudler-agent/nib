//go:build unix

package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/mudler/nib/internal/proc"
)

func TestShellJobsShutdownStopsRunningJobs(t *testing.T) {
	t.Setenv("SHELL_CMD", "")
	jobs := &ShellJobs{mgr: newBgJobManager()}
	a := jobs.mgr.launch(context.Background(), "sleep 30", false)
	b := jobs.mgr.launch(context.Background(), "trap '' TERM; sleep 30", false)
	done := jobs.mgr.launch(context.Background(), "true", false)
	waitJobWithin(t, done, 2*time.Second)
	time.Sleep(200 * time.Millisecond) // past the fork, see bgshell_kill_test.go

	if left := jobs.Shutdown(proc.KillGrace + 2*time.Second); left != 0 {
		t.Fatalf("%d jobs still running after Shutdown", left)
	}
	for _, j := range []*bgJob{a, b} {
		if d, _, _ := j.snapshot(); !d {
			t.Fatalf("job %s not done after Shutdown", j.id)
		}
	}
}

func TestShellJobsShutdownNilSafe(t *testing.T) {
	var jobs *ShellJobs
	if left := jobs.Shutdown(time.Second); left != 0 {
		t.Fatalf("nil Shutdown = %d", left)
	}
}
