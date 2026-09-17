package codex

import (
	"strings"
	"testing"
)

func TestSanitizeCodexCallId(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "call_" + fnv1aBase36("empty")},
		{"valid id", "call_abc123", "call_abc123"},
		{"valid id at boundary 64 chars", strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"too long", strings.Repeat("a", 65), ""},
		{"invalid chars", "call@abc!", ""},
		{"composite with pipe", "call_abc|extra", "call_abc"},
		{"composite with newline", "call_abc\nextra", "call_abc"},
		{"leading pipe", "|abc", ""},
		{"all invalid chars", "@@@@", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCodexCallId(tt.in)
			if tt.name == "too long" || tt.name == "invalid chars" || tt.name == "leading pipe" || tt.name == "all invalid chars" {
				if len(got) > 64 {
					t.Fatalf("result too long: %d chars", len(got))
				}
				if !isValidCallID(got) {
					t.Fatalf("result has invalid charset: %q", got)
				}
				if got == tt.in {
					t.Fatalf("expected transformation, got same: %q", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("sanitizeCodexCallId(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeCodexCallIdDeterministic(t *testing.T) {
	id := "call_with_bad@chars#here"
	a := sanitizeCodexCallId(id)
	b := sanitizeCodexCallId(id)
	if a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}
}

func TestSanitizeCodexCallIdCompositeConsistent(t *testing.T) {
	base := "call_abc"
	a := sanitizeCodexCallId(base + "|item1")
	b := sanitizeCodexCallId(base + "|item2")
	if a != b {
		t.Fatalf("composite IDs with same base should sanitize to same value: %q vs %q", a, b)
	}
}

func TestFilterInput(t *testing.T) {
	input := []codexInput{
		{ID: "1", Type: "message", Role: "user", Content: "hi"},
		{Type: "item_reference"},
		{ID: "2", Type: "computer_call"},
		{ID: "3", Type: "function_call", CallID: "c1"},
	}
	result := filterInput(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0].ID != "" {
		t.Fatalf("expected id stripped from message, got %q", result[0].ID)
	}
	if result[1].ID != "2" {
		t.Fatalf("expected id retained on computer_call, got %q", result[1].ID)
	}
	if result[2].ID != "" {
		t.Fatalf("expected id stripped from function_call, got %q", result[2].ID)
	}
}

func TestSanitizeInputCallIds(t *testing.T) {
	input := []codexInput{
		{Type: "function_call", CallID: "call_valid123"},
		{Type: "function_call_output", CallID: "call_valid123"},
		{Type: "message", Role: "user", Content: "no call id"},
		{Type: "function_call", CallID: "bad@id!"},
	}
	sanitizeInputCallIds(input)
	if input[0].CallID != "call_valid123" {
		t.Fatalf("valid id should be unchanged: %q", input[0].CallID)
	}
	if input[1].CallID != "call_valid123" {
		t.Fatalf("valid id should be unchanged: %q", input[1].CallID)
	}
	if input[3].CallID == "bad@id!" {
		t.Fatalf("invalid id should be sanitized: %q", input[3].CallID)
	}
	if !isValidCallID(input[3].CallID) {
		t.Fatalf("sanitized id has invalid charset: %q", input[3].CallID)
	}
}

func TestRepairToolCallPairs_Paired(t *testing.T) {
	input := []codexInput{
		{Type: "function_call", CallID: "c1", Name: "get_weather", Arguments: "{}"},
		{Type: "function_call_output", CallID: "c1", Output: "sunny"},
	}
	result := repairToolCallPairs(input)
	if len(result) != 2 {
		t.Fatalf("paired call+output should stay 2 items, got %d", len(result))
	}
	if result[0].Type != "function_call" || result[1].Type != "function_call_output" {
		t.Fatalf("paired items should be unchanged")
	}
}

func TestRepairToolCallPairs_OrphanCall(t *testing.T) {
	input := []codexInput{
		{Type: "function_call", CallID: "c1", Name: "get_weather", Arguments: "{}"},
		{Type: "message", Role: "user", Content: "next"},
	}
	result := repairToolCallPairs(input)
	if len(result) != 3 {
		t.Fatalf("orphan call should add synthesized output, got %d items", len(result))
	}
	if result[0].Type != "function_call" {
		t.Fatalf("first item should be original call, got %q", result[0].Type)
	}
	if result[1].Type != "function_call_output" || result[1].CallID != "c1" || result[1].Output != codexInterruptedToolOutput {
		t.Fatalf("second item should be synthesized output, got %+v", result[1])
	}
	if result[2].Type != "message" {
		t.Fatalf("third item should be original message, got %q", result[2].Type)
	}
}

func TestRepairToolCallPairs_OrphanOutput(t *testing.T) {
	input := []codexInput{
		{Type: "function_call_output", CallID: "c1", Output: "sunny"},
	}
	result := repairToolCallPairs(input)
	if len(result) != 1 {
		t.Fatalf("orphan output should fold into one message, got %d", len(result))
	}
	if result[0].Type != "message" || result[0].Role != "assistant" {
		t.Fatalf("orphan output should become assistant message, got %+v", result[0])
	}
	if !strings.Contains(result[0].Content, "call_id=c1") {
		t.Fatalf("folded message should contain call_id, got %q", result[0].Content)
	}
	if !strings.Contains(result[0].Content, "sunny") {
		t.Fatalf("folded message should contain output text, got %q", result[0].Content)
	}
}

func TestRepairToolCallPairs_OrphanOutputTruncation(t *testing.T) {
	longOutput := strings.Repeat("x", codexOrphanOutputLimit+100)
	input := []codexInput{
		{Type: "function_call_output", CallID: "c1", Output: longOutput},
	}
	result := repairToolCallPairs(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	if !strings.Contains(result[0].Content, "...[truncated]") {
		t.Fatalf("expected truncated output, got %q", result[0].Content)
	}
}

func TestRepairToolCallPairs_OrphanComputerCall(t *testing.T) {
	input := []codexInput{
		{Type: "computer_call", CallID: "c1"},
		{Type: "message", Role: "user", Content: "next"},
	}
	result := repairToolCallPairs(input)
	if len(result) != 2 {
		t.Fatalf("orphan computer call should become one message, got %d", len(result))
	}
	if result[0].Type != "message" || result[0].Role != "assistant" {
		t.Fatalf("orphan computer call should become assistant message, got %+v", result[0])
	}
	if !strings.Contains(result[0].Content, "call_id=c1") {
		t.Fatalf("message should contain call_id, got %q", result[0].Content)
	}
}

func TestRepairToolCallPairs_KindMismatch(t *testing.T) {
	input := []codexInput{
		{Type: "function_call", CallID: "c1", Name: "fn"},
		{Type: "custom_tool_call_output", CallID: "c1", Output: "result"},
	}
	result := repairToolCallPairs(input)
	// Both should be treated as orphans — call gets synthesized output,
	// output gets folded into message.
	if len(result) != 3 {
		t.Fatalf("kind mismatch should produce 3 items (call + synth output + folded msg), got %d", len(result))
	}
}

func TestRepairToolCallPairs_CustomToolOrphanCall(t *testing.T) {
	input := []codexInput{
		{Type: "custom_tool_call", CallID: "c1", Name: "my_tool"},
	}
	result := repairToolCallPairs(input)
	if len(result) != 2 {
		t.Fatalf("orphan custom call should add synthesized output, got %d", len(result))
	}
	if result[1].Type != "custom_tool_call_output" {
		t.Fatalf("expected custom_tool_call_output, got %q", result[1].Type)
	}
	if result[1].Output != codexInterruptedToolOutput {
		t.Fatalf("expected placeholder output, got %q", result[1].Output)
	}
}

func TestRepairToolCallPairs_OrphanOutputUsesName(t *testing.T) {
	input := []codexInput{
		{Type: "function_call_output", CallID: "c1", Name: "get_weather", Output: "sunny"},
	}
	result := repairToolCallPairs(input)
	if !strings.Contains(result[0].Content, "get_weather") {
		t.Fatalf("folded message should use tool name, got %q", result[0].Content)
	}
}

func TestRepairToolCallPairs_OrphanOutputNoNameDefaultsToTool(t *testing.T) {
	input := []codexInput{
		{Type: "function_call_output", CallID: "c1", Output: "sunny"},
	}
	result := repairToolCallPairs(input)
	if !strings.Contains(result[0].Content, "tool") {
		t.Fatalf("folded message should default to 'tool' name, got %q", result[0].Content)
	}
}

func TestFullRepairPipeline(t *testing.T) {
	input := []codexInput{
		{ID: "x", Type: "message", Role: "user", Content: "hi"},
		{Type: "item_reference"},
		{Type: "function_call", CallID: "call_good", Name: "fn", Arguments: "{}"},
		{Type: "function_call_output", CallID: "call_good", Output: "ok"},
		{Type: "function_call", CallID: "orphan_call", Name: "fn2", Arguments: "{}"},
		{Type: "function_call_output", CallID: "orphan_out", Output: "lost"},
	}

	filtered := filterInput(input)
	sanitizeInputCallIds(filtered)
	repaired := repairToolCallPairs(filtered)

	// item_reference should be gone, id stripped from message
	// orphan_call gets synth output → +1 item
	// orphan_out folds into message → 0 items (replaces 1 with 1)
	// Total: 6 - 1 (item_ref) + 1 (synth) = 6
	if len(repaired) != 6 {
		t.Fatalf("expected 6 items after full pipeline, got %d", len(repaired))
	}

	for _, item := range repaired {
		if item.ID != "" {
			t.Fatalf("all ids should be stripped, found %q", item.ID)
		}
		if item.CallID != "" && !isValidCallID(item.CallID) {
			t.Fatalf("call_id has invalid charset: %q", item.CallID)
		}
	}
}

func isValidCallID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
