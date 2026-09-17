package codex

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

const (
	codexOrphanOutputLimit     = 16000
	codexInterruptedToolOutput = "[No tool output recorded: the tool call was interrupted before it produced a result.]"
)

// toolCallKind classifies a tool call item into its call kind.
type toolCallKind string

const (
	kindFunction toolCallKind = "function"
	kindCustom   toolCallKind = "custom"
	kindComputer toolCallKind = "computer"
)

func toolCallKindOf(typ string) (toolCallKind, bool) {
	switch typ {
	case "function_call":
		return kindFunction, true
	case "custom_tool_call":
		return kindCustom, true
	case "computer_call":
		return kindComputer, true
	}
	return "", false
}

func toolOutputKindOf(typ string) (toolCallKind, bool) {
	switch typ {
	case "function_call_output":
		return kindFunction, true
	case "custom_tool_call_output":
		return kindCustom, true
	case "computer_call_output":
		return kindComputer, true
	}
	return "", false
}

// filterInput drops item_reference items and strips the id field from all
// items except computer_call. Returns a new slice; does not mutate input.
func filterInput(input []codexInput) []codexInput {
	out := make([]codexInput, 0, len(input))
	for _, item := range input {
		if item.Type == "item_reference" {
			continue
		}
		if item.Type != "computer_call" {
			item.ID = ""
		}
		out = append(out, item)
	}
	return out
}

// sanitizeCodexCallId enforces the ≤64-char limit and [a-zA-Z0-9_-] charset
// on a tool call ID. Composite IDs (containing | or \n) have the secondary
// part stripped. When transformation is needed, a hash suffix is appended for
// collision resistance.
func sanitizeCodexCallId(rawCallID string) string {
	if rawCallID == "" {
		return "call_" + fnv1aBase36("empty")
	}

	sep := strings.IndexAny(rawCallID, "|\n")
	var base string
	switch {
	case sep > 0:
		base = rawCallID[:sep]
	case sep == 0:
		base = rawCallID[1:]
	default:
		base = rawCallID
	}

	sanitized := sanitizeCallIDCharset(base)
	sanitized = strings.TrimRight(sanitized, "_")

	if len(sanitized) > 0 && len(sanitized) <= 64 && sanitized == base {
		return sanitized
	}

	hashInput := base
	if hashInput == "" {
		hashInput = rawCallID
	}
	hash := fnv1aBase36(hashInput)

	effectiveBase := sanitized
	if effectiveBase == "" {
		effectiveBase = "call"
	}

	prefixLen := 63 - len(hash)
	if prefixLen < 0 {
		prefixLen = 0
	}
	if prefixLen > len(effectiveBase) {
		prefixLen = len(effectiveBase)
	}

	result := effectiveBase[:prefixLen] + "_" + hash
	if len(result) > 64 {
		result = result[:64]
	}
	return result
}

// sanitizeInputCallIds mutates the CallID field of every item in place.
func sanitizeInputCallIds(input []codexInput) {
	for i := range input {
		if input[i].CallID != "" {
			input[i].CallID = sanitizeCodexCallId(input[i].CallID)
		}
	}
}

// repairToolCallPairs fixes unpaired tool exchanges so the Responses input
// grammar stays valid. Orphan outputs (no matching call of same kind) are
// folded into assistant messages. Orphan calls (no matching output) get a
// synthesized placeholder output, except computer calls which become a message.
func repairToolCallPairs(input []codexInput) []codexInput {
	callKinds := make(map[string]toolCallKind)
	outputKinds := make(map[string]toolCallKind)

	for i := range input {
		item := &input[i]
		if item.CallID == "" {
			continue
		}
		if k, ok := toolCallKindOf(item.Type); ok {
			callKinds[item.CallID] = k
		}
		if k, ok := toolOutputKindOf(item.Type); ok {
			outputKinds[item.CallID] = k
		}
	}

	repaired := make([]codexInput, 0, len(input))
	for i := range input {
		item := input[i]
		callID := item.CallID

		if outputKind, ok := toolOutputKindOf(item.Type); ok && callID != "" && callKinds[callID] != outputKind {
			repaired = append(repaired, orphanOutputToMessage(item, callID))
			continue
		}

		if callKind, ok := toolCallKindOf(item.Type); ok && callID != "" && outputKinds[callID] != callKind {
			if callKind == kindComputer {
				repaired = append(repaired, codexInput{
					Type:    "message",
					Role:    "assistant",
					Content: fmt.Sprintf("[Computer call interrupted before a screenshot was recorded; call_id=%s]", callID),
				})
				continue
			}
			outputType := "function_call_output"
			if callKind == kindCustom {
				outputType = "custom_tool_call_output"
			}
			repaired = append(repaired, item, codexInput{
				Type:   outputType,
				CallID: callID,
				Output: codexInterruptedToolOutput,
			})
			continue
		}

		repaired = append(repaired, item)
	}
	return repaired
}

// orphanOutputToMessage folds an orphan tool output item into an assistant
// message, truncating to codexOrphanOutputLimit chars.
func orphanOutputToMessage(item codexInput, callID string) codexInput {
	toolName := item.Name
	if toolName == "" {
		toolName = "tool"
	}
	text := item.Output
	if len(text) > codexOrphanOutputLimit {
		text = text[:codexOrphanOutputLimit] + "\n...[truncated]"
	}
	return codexInput{
		Type:    "message",
		Role:    "assistant",
		Content: fmt.Sprintf("[Previous %s result; call_id=%s]: %s", toolName, callID, text),
	}
}

func sanitizeCallIDCharset(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			b.WriteByte(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func fnv1aBase36(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return strconv.FormatUint(h.Sum64(), 36)
}
