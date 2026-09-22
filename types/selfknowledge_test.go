package types

import (
	"strings"
	"testing"
)

func TestSelfKnowledgeSuffix(t *testing.T) {
	got := selfKnowledgeSuffix("nib")

	if !strings.Contains(got, "You are nib") {
		t.Errorf("suffix missing identity: %q", got)
	}
	if !strings.Contains(got, "self_read") {
		t.Errorf("suffix missing self_read tool mention: %q", got)
	}
	if !strings.Contains(got, "documentation") {
		t.Errorf("suffix missing documentation mention: %q", got)
	}
}

func TestSelfKnowledgeSuffixCustomProgramName(t *testing.T) {
	got := selfKnowledgeSuffix("my-agent")
	if !strings.Contains(got, "You are my-agent") {
		t.Errorf("suffix missing custom identity: %q", got)
	}
	if !strings.Contains(got, "my-agent's embedded documentation") {
		t.Errorf("suffix missing custom possessive: %q", got)
	}
}

func TestGetPromptIncludesSelfKnowledge(t *testing.T) {
	cfg := Config{
		Prompt:     "test prompt",
		ProgramName: "nib",
	}
	got := cfg.GetPrompt()

	if !strings.Contains(got, "You are nib") {
		t.Error("GetPrompt() does not include self-knowledge suffix")
	}
	if !strings.Contains(got, "self_read") {
		t.Error("GetPrompt() does not mention self_read tool")
	}
}
