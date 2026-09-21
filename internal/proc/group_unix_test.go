//go:build unix

package proc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// startGroup runs script under sh in its own group and returns once the
// script has written ready, so its signal traps are installed.
func startGroup(t *testing.T, script string) (string, context.CancelFunc, chan error) {
	t.Helper()
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sh", "-c", script+"\nwait")
	cmd.Env = append(os.Environ(), "DIR="+dir)
	Group(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("script never became ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return dir, cancel, done
}

// A cancelled command gets SIGTERM first, so it can clean up. Before, the
// group got SIGKILL at once and a command's cleanup never ran.
func TestGroupCancelSendsSIGTERMFirst(t *testing.T) {
	dir, cancel, done := startGroup(t, `trap 'touch "$DIR/cleaned"; exit 0' TERM; touch "$DIR/ready"; sleep 30 &`)
	cancel()
	select {
	case <-done:
	case <-time.After(KillGrace + 2*time.Second):
		t.Fatal("command did not exit after cancel")
	}
	if _, err := os.Stat(filepath.Join(dir, "cleaned")); err != nil {
		t.Fatal("the TERM trap never ran: the command got no chance to clean up")
	}
}

// A command that ignores SIGTERM is still stopped: the group gets SIGKILL
// after KillGrace.
func TestGroupCancelKillsACommandThatIgnoresSIGTERM(t *testing.T) {
	_, cancel, done := startGroup(t, `trap '' TERM; touch "$DIR/ready"; sleep 30 &`)
	start := time.Now()
	cancel()
	select {
	case <-done:
		if el := time.Since(start); el < KillGrace/2 {
			t.Fatalf("exited after %v; SIGTERM alone should not stop it", el)
		}
	case <-time.After(KillGrace + 3*time.Second):
		t.Fatal("a command that ignores SIGTERM was never killed")
	}
}
