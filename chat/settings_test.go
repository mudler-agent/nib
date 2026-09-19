package chat

import (
	"testing"

	"github.com/mudler/nib/types"
)

func TestSetApprovalModeSwitchesAutoApproveAndReadOnlyRule(t *testing.T) {
	s := newDecideSession("", func(ToolCallRequest) ToolCallResponse { return ToolCallResponse{Approved: false} })

	s.SetApprovalMode("auto")
	if !s.AutoApprove() {
		t.Fatal("auto mode must turn auto-approve on")
	}
	s.SetApprovalMode("strict")
	if s.AutoApprove() {
		t.Fatal("leaving auto mode must turn auto-approve off")
	}
	// strict prompts even for a read-only call, which prompt mode approves.
	if d := s.decideToolCall(ToolCallRequest{Name: "read", Arguments: `{"path":"x"}`}); d.Approved {
		t.Fatal("strict mode auto-approved a read-only call")
	}
	s.SetApprovalMode("prompt")
	if d := s.decideToolCall(ToolCallRequest{Name: "read", Arguments: `{"path":"x"}`}); !d.Approved {
		t.Fatal("prompt mode must auto-approve a read-only call")
	}
}

// A zero max_context_tokens means "auto-detect", so applying a compaction
// block that leaves it unset must keep the window the session already
// detected rather than zeroing it.
func TestSetCompactionKeepsDetectedWindow(t *testing.T) {
	s := &Session{compaction: types.CompactionConfig{MaxContextTokens: 32000, Threshold: 0.8}, compactionAutoDetected: true}
	s.SetCompaction(types.CompactionConfig{Threshold: 0.6, KeepRecent: 4})
	if got := s.MaxContextTokens(); got != 32000 {
		t.Fatalf("window = %d, want the detected 32000 kept", got)
	}
	if c := s.compactionConfig(); c.Threshold != 0.6 || c.KeepRecent != 4 {
		t.Fatalf("compaction = %+v", c)
	}

	// An explicit window is an override and replaces the detected one.
	s.SetCompaction(types.CompactionConfig{Threshold: 0.6, MaxContextTokens: 64000})
	if got := s.MaxContextTokens(); got != 64000 || s.compactionAutoDetected {
		t.Fatalf("window = %d autodetected %v, want the explicit 64000", got, s.compactionAutoDetected)
	}
}

func TestSetToolOutputPruning(t *testing.T) {
	s := &Session{}
	s.SetToolOutputPruning(types.ToolOutputPruningConfig{HighWaterTokens: 1000, LowWaterTokens: 10})
	s.prunedMu.Lock()
	defer s.prunedMu.Unlock()
	if s.pruning.HighWaterTokens != 1000 {
		t.Fatalf("pruning = %+v", s.pruning)
	}
}
