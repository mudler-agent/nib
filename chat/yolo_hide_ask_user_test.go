package chat

import (
	"context"
	"slices"
	"testing"
)

// TestNormalSessionAdvertisesAskUser: a session without auto-approval must
// advertise ask_user when the built-in tool allowlist permits it.
func TestNormalSessionAdvertisesAskUser(t *testing.T) {
	s := newWarmTestSession(t)
	llm := s.llm.(*captureLLM)

	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); !slices.Contains(got, "ask_user") {
		t.Fatalf("normal session must advertise ask_user, got: %v", got)
	}
}

// TestAutoApproveSessionHidesAskUser: an auto-approval (yolo) session must
// not advertise ask_user — yolo mode must run without blocking prompts.
func TestAutoApproveSessionHidesAskUser(t *testing.T) {
	s := newWarmTestSession(t)
	s.SetAutoApprove(true)
	llm := s.llm.(*captureLLM)

	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); slices.Contains(got, "ask_user") {
		t.Fatalf("yolo session must not advertise ask_user, got: %v", got)
	}
}

// TestEnablingAutoApproveRemovesAskUser: turning auto-approval on must
// remove ask_user from the next tool set.
func TestEnablingAutoApproveRemovesAskUser(t *testing.T) {
	s := newWarmTestSession(t)
	llm := s.llm.(*captureLLM)

	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); !slices.Contains(got, "ask_user") {
		t.Fatalf("precondition: normal session must advertise ask_user, got: %v", got)
	}

	s.SetAutoApprove(true)
	llm.reset()
	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); slices.Contains(got, "ask_user") {
		t.Fatalf("enabling yolo must remove ask_user, got: %v", got)
	}
}

// TestDisablingAutoApproveRestoresAskUser: turning auto-approval off must
// restore ask_user in the next tool set.
func TestDisablingAutoApproveRestoresAskUser(t *testing.T) {
	s := newWarmTestSession(t)
	s.SetAutoApprove(true)
	llm := s.llm.(*captureLLM)

	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); slices.Contains(got, "ask_user") {
		t.Fatalf("precondition: yolo session must not advertise ask_user, got: %v", got)
	}

	s.SetAutoApprove(false)
	llm.reset()
	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); !slices.Contains(got, "ask_user") {
		t.Fatalf("disabling yolo must restore ask_user, got: %v", got)
	}
}

// TestAskUserExcludedByAllowlistStaysExcluded: an allowlist that excludes
// ask_user must continue to exclude it after yolo is disabled.
func TestAskUserExcludedByAllowlistStaysExcluded(t *testing.T) {
	s := newWarmTestSession(t)
	// Overwrite the allowlist to exclude ask_user but keep other tools.
	s.toolAllow = map[string]bool{
		"agent_logs":  true,
		"spawn_agent": true,
	}
	s.SetAutoApprove(true)
	llm := s.llm.(*captureLLM)

	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); slices.Contains(got, "ask_user") {
		t.Fatalf("allowlist-excluded ask_user must not appear in yolo, got: %v", got)
	}

	s.SetAutoApprove(false)
	llm.reset()
	if err := s.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if got := toolNames(llm.lastRequest().Tools); slices.Contains(got, "ask_user") {
		t.Fatalf("allowlist-excluded ask_user must stay excluded after yolo off, got: %v", got)
	}
}
