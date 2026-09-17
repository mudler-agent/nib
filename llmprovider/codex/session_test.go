package codex

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// makeHeader builds an http.Header using Set (which canonicalizes keys),
// matching how the real HTTP client populates response headers.
func makeHeader(kv ...string) http.Header {
	h := make(http.Header)
	for i := 0; i+1 < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

func TestIsWithinTurnContinuation(t *testing.T) {
	tests := []struct {
		name     string
		messages []openai.ChatCompletionMessage
		want     bool
	}{
		{
			name: "empty messages",
			want: false,
		},
		{
			name: "single user message",
			messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "hello"},
			},
			want: false,
		},
		{
			name: "assistant last",
			messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "hello"},
				{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
			},
			want: true,
		},
		{
			name: "tool result after assistant",
			messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "hello"},
				{Role: openai.ChatMessageRoleAssistant, Content: ""},
				{Role: openai.ChatMessageRoleTool, Content: "result"},
			},
			want: true,
		},
		{
			name: "multiple tool results after assistant",
			messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "hello"},
				{Role: openai.ChatMessageRoleAssistant, Content: ""},
				{Role: openai.ChatMessageRoleTool, Content: "r1"},
				{Role: openai.ChatMessageRoleTool, Content: "r2"},
			},
			want: true,
		},
		{
			name: "user after tool results",
			messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "hello"},
				{Role: openai.ChatMessageRoleAssistant, Content: ""},
				{Role: openai.ChatMessageRoleTool, Content: "result"},
				{Role: openai.ChatMessageRoleUser, Content: "thanks"},
			},
			want: false,
		},
		{
			name: "only tool results",
			messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleTool, Content: "r1"},
				{Role: openai.ChatMessageRoleTool, Content: "r2"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWithinTurnContinuation(tt.messages)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrepareTurnNewTurnClearsState(t *testing.T) {
	s := newSessionState()
	oldTurnID := s.turnID
	oldTurnState := s.turnState

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "new turn"},
	}
	meta := s.prepareTurn(msgs)

	if meta.TurnID == oldTurnID {
		t.Fatal("new turn should generate a new turn ID")
	}
	if meta.HasTurnState {
		t.Fatal("new turn should not have turn state")
	}
	if meta.TurnState != "" {
		t.Fatalf("new turn should have empty turn state, got %q", meta.TurnState)
	}
	if oldTurnState != "" && meta.TurnState != "" {
		t.Fatal("new turn should clear previous turn state")
	}
}

func TestPrepareTurnContinuationReusesTurnID(t *testing.T) {
	s := newSessionState()

	// First request — new turn
	msgs1 := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
	}
	meta1 := s.prepareTurn(msgs1)

	// Simulate turn state capture
	s.captureTurnState(makeHeader(headerTurnState, "turn-state-token"))

	// Second request — continuation (assistant + tool result)
	msgs2 := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "calling tool"},
		{Role: openai.ChatMessageRoleTool, Content: "result"},
	}
	meta2 := s.prepareTurn(msgs2)

	if meta2.TurnID != meta1.TurnID {
		t.Fatalf("continuation should reuse turn ID: got %q, want %q", meta2.TurnID, meta1.TurnID)
	}
	if !meta2.HasTurnState {
		t.Fatal("continuation should preserve captured turn state")
	}
	if meta2.TurnState != "turn-state-token" {
		t.Fatalf("continuation should echo captured turn state: got %q", meta2.TurnState)
	}
}

func TestCaptureTurnStateFirstWriteWins(t *testing.T) {
	s := newSessionState()

	// First capture
	s.captureTurnState(makeHeader(headerTurnState, "first"))
	if s.turnState != "first" || !s.hasTurnState {
		t.Fatalf("first capture failed: state=%q hasState=%v", s.turnState, s.hasTurnState)
	}

	// Second capture — should be ignored
	s.captureTurnState(makeHeader(headerTurnState, "second"))
	if s.turnState != "first" {
		t.Fatalf("first-write-wins violated: got %q, want first", s.turnState)
	}
}

func TestCaptureTurnStateEmptyHeader(t *testing.T) {
	s := newSessionState()
	s.captureTurnState(http.Header{})
	if s.hasTurnState {
		t.Fatal("should not capture from empty header")
	}
}

func TestPrepareTurnNewTurnAfterContinuation(t *testing.T) {
	s := newSessionState()

	// Turn 1
	meta1 := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "turn 1"},
	})
	s.captureTurnState(makeHeader(headerTurnState, "ts1"))

	// Continuation
	meta2 := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "turn 1"},
		{Role: openai.ChatMessageRoleAssistant, Content: "resp"},
		{Role: openai.ChatMessageRoleTool, Content: "r"},
	})
	if !meta2.HasTurnState || meta2.TurnState != "ts1" {
		t.Fatalf("continuation should carry turn state: %+v", meta2)
	}

	// Turn 2 — new user message
	meta3 := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "turn 2"},
	})
	if meta3.HasTurnState {
		t.Fatal("new turn should clear turn state")
	}
	if meta3.TurnID == meta1.TurnID {
		t.Fatal("new turn should generate new turn ID")
	}
}

func TestClientMetadataKeys(t *testing.T) {
	s := newSessionState()
	meta := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hi"},
	})
	cm := meta.clientMetadata()

	required := []string{
		metaInstallationID,
		metaSessionID,
		metaThreadID,
		metaWindowID,
		metaTurnID,
		metaTurnMetadata,
	}
	for _, k := range required {
		v, ok := cm[k]
		if !ok {
			t.Errorf("client_metadata missing key %q", k)
		}
		if v == "" {
			t.Errorf("client_metadata key %q is empty", k)
		}
	}
}

func TestClientMetadataTurnMetadataIsAsciiJSON(t *testing.T) {
	s := newSessionState()
	meta := s.buildMetadata()
	cm := meta.clientMetadata()

	raw := cm[metaTurnMetadata]
	if raw == "" {
		t.Fatal("turn metadata is empty")
	}

	// Parse the ASCII-safe JSON back
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("turn metadata is not valid JSON: %v", err)
	}

	required := []string{
		"installation_id",
		"session_id",
		"thread_id",
		"turn_id",
		"window_id",
		"request_kind",
	}
	for _, k := range required {
		if _, ok := parsed[k]; !ok {
			t.Errorf("turn metadata missing field %q", k)
		}
	}
	if parsed["request_kind"] != "turn" {
		t.Errorf("request_kind = %v, want turn", parsed["request_kind"])
	}
}

func TestToAsciiJSON(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"simple ascii", map[string]any{"a": "b"}, `{"a":"b"}`},
		{"empty map", map[string]any{}, `{}`},
		{"unicode 0x7f", map[string]any{"k": "\x7f"}, `{"k":"\u007f"}`},
		{"emoji", map[string]any{"k": "😀"}, `{"k":"\ud83d\ude00"}`},
		{"mixed", map[string]any{"k": "café"}, `{"k":"caf\u00e9"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toAsciiJSON(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToAsciiJSONNoNonAsciiBytes(t *testing.T) {
	in := map[string]any{
		"name":  "日本語テスト",
		"emoji": "🎉🚀",
		"mix":   "hello世界",
	}
	out, err := toAsciiJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range []byte(out) {
		if b > 0x7f {
			t.Fatalf("non-ASCII byte 0x%02x at position %d in output: %s", b, i, out)
		}
	}

	// Verify it round-trips through json.Unmarshal
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestBuildMetadataTurnStartedMs(t *testing.T) {
	s := newSessionState()
	// Before any turn, turnStartedMs is 0
	meta := s.buildMetadata()
	if meta.TurnStartedMs != 0 {
		t.Fatalf("expected 0 before first turn, got %d", meta.TurnStartedMs)
	}

	// After prepareTurn, turnStartedMs should be set
	meta2 := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hi"},
	})
	if meta2.TurnStartedMs <= 0 {
		t.Fatal("turnStartedMs should be positive after prepareTurn")
	}

	// Turn metadata JSON should include turn_started_at_unix_ms
	var parsed map[string]any
	if err := json.Unmarshal([]byte(meta2.TurnMetadataJSON), &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["turn_started_at_unix_ms"]; !ok {
		t.Fatal("turn metadata should include turn_started_at_unix_ms after prepareTurn")
	}
}

func TestGetOrCreateInstallIDIdempotent(t *testing.T) {
	id1 := getOrCreateInstallID()
	id2 := getOrCreateInstallID()
	if id1 != id2 {
		t.Fatalf("install ID not idempotent: first=%q second=%q", id1, id2)
	}
	if !isValidUUID(id1) {
		t.Fatalf("install ID is not a valid UUID: %q", id1)
	}
}

func TestPrepareTurnPreservesSessionThreadWindow(t *testing.T) {
	s := newSessionState()
	sessionID := s.sessionID
	threadID := s.threadID
	windowID := s.windowID

	// Multiple turns should keep session/thread/window stable
	for i := 0; i < 3; i++ {
		meta := s.prepareTurn([]openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "turn"},
		})
		if meta.SessionID != sessionID {
			t.Fatalf("session ID changed on turn %d", i)
		}
		if meta.ThreadID != threadID {
			t.Fatalf("thread ID changed on turn %d", i)
		}
		if meta.WindowID != windowID {
			t.Fatalf("window ID changed on turn %d", i)
		}
	}
}

func TestTurnMetadataJSONIsAsciiSafe(t *testing.T) {
	s := newSessionState()
	meta := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hi"},
	})
	if meta.TurnMetadataJSON == "" {
		t.Fatal("turn metadata JSON is empty")
	}
	for i, b := range []byte(meta.TurnMetadataJSON) {
		if b > 0x7f {
			t.Fatalf("non-ASCII byte 0x%02x at position %d in turn metadata JSON", b, i)
		}
	}
}

func TestCaptureTurnStateFromSSEHeaders(t *testing.T) {
	// Simulate the flow: prepareTurn → capture from response headers → continuation
	s := newSessionState()

	// First turn — no turn state
	meta1 := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
	})
	if meta1.HasTurnState {
		t.Fatal("first turn should not have turn state")
	}

	// Capture from response headers (as done in CreateChatCompletion)
	s.captureTurnState(makeHeader(
		headerTurnState, "opaque-turn-token-abc123",
		"x-models-etag", "etag-xyz",
		"content-type", "text/event-stream",
	))

	// Continuation — should carry captured turn state
	meta2 := s.prepareTurn([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "resp"},
		{Role: openai.ChatMessageRoleTool, Content: "output"},
	})
	if !meta2.HasTurnState {
		t.Fatal("continuation should have captured turn state")
	}
	if meta2.TurnState != "opaque-turn-token-abc123" {
		t.Fatalf("wrong turn state: got %q", meta2.TurnState)
	}
}

// Ensure the full string output of turn metadata doesn't contain
// literal non-ASCII (this catches regression where toAsciiJSON is
// bypassed).
func TestTurnMetadataJSONNoLiteralUnicode(t *testing.T) {
	s := newSessionState()
	// Force a non-ASCII value by setting installation ID to something unicode
	s.installationID = "ünïcödé-test"

	meta := s.buildMetadata()
	cm := meta.clientMetadata()
	raw := cm[metaTurnMetadata]

	if strings.Contains(raw, "ünïcödé") {
		t.Fatalf("turn metadata contains literal non-ASCII: %s", raw)
	}
	if !strings.Contains(raw, `\u00fc`) {
		t.Fatalf("turn metadata should contain escaped ü: %s", raw)
	}
}
