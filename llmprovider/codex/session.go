package codex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
)

const (
	headerTurnState       = "x-codex-turn-state"
	headerScopedSessionID = "session-id"
	headerThreadID        = "thread-id"
	headerWindowID        = "x-codex-window-id"
	headerTurnMetadata    = "x-codex-turn-metadata"
	headerSessionID       = "session_id"
	headerConversationID  = "conversation_id"
	headerClientRequestID = "x-client-request-id"

	metaInstallationID = "x-codex-installation-id"
	metaSessionID      = "session_id"
	metaThreadID       = "thread_id"
	metaWindowID       = "x-codex-window-id"
	metaTurnID         = "turn_id"
	metaTurnMetadata   = "x-codex-turn-metadata"
)

// sessionState holds Codex turn state and client metadata, persisted across
// CreateChatCompletion calls on the same LLM instance. The x-codex-turn-state
// header is captured from the first response in a turn (first-write-wins) and
// echoed on subsequent requests for sticky routing. A new user turn clears it.
type sessionState struct {
	mu             sync.Mutex
	sessionID      string
	threadID       string
	windowID       string
	turnID         string
	turnStartedMs  int64
	turnState      string
	hasTurnState   bool
	installationID string
}

func newSessionState() *sessionState {
	return &sessionState{
		sessionID:      uuid.NewString(),
		threadID:       uuid.NewString(),
		windowID:       uuid.NewString(),
		installationID: getOrCreateInstallID(),
	}
}

// requestMetadata is a snapshot of session state used to build headers and body.
type requestMetadata struct {
	SessionID        string
	ThreadID         string
	WindowID         string
	TurnID           string
	TurnStartedMs    int64
	InstallationID   string
	TurnState        string
	HasTurnState     bool
	TurnMetadataJSON string
}

// prepareTurn detects whether this request continues the current turn or
// starts a new one, updates session state accordingly, and returns a snapshot.
func (s *sessionState) prepareTurn(messages []openai.ChatCompletionMessage) requestMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isWithinTurnContinuation(messages) {
		s.turnID = uuid.NewString()
		s.turnStartedMs = time.Now().UnixMilli()
		s.turnState = ""
		s.hasTurnState = false
	}

	return s.buildMetadata()
}

// captureTurnState captures x-codex-turn-state from response headers.
// First-write-wins: only set when not already captured.
func (s *sessionState) captureTurnState(headers http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hasTurnState {
		return
	}
	if v := headers.Get(headerTurnState); v != "" {
		s.turnState = v
		s.hasTurnState = true
	}
}

func (s *sessionState) buildMetadata() requestMetadata {
	turnMeta := map[string]any{
		"installation_id": s.installationID,
		"session_id":      s.sessionID,
		"thread_id":       s.threadID,
		"turn_id":         s.turnID,
		"window_id":       s.windowID,
		"request_kind":    "turn",
	}
	if s.turnStartedMs > 0 {
		turnMeta["turn_started_at_unix_ms"] = s.turnStartedMs
	}

	turnJSON, _ := toAsciiJSON(turnMeta)

	return requestMetadata{
		SessionID:        s.sessionID,
		ThreadID:         s.threadID,
		WindowID:         s.windowID,
		TurnID:           s.turnID,
		TurnStartedMs:    s.turnStartedMs,
		InstallationID:   s.installationID,
		TurnState:        s.turnState,
		HasTurnState:     s.hasTurnState,
		TurnMetadataJSON:  turnJSON,
	}
}

// clientMetadata builds the body client_metadata map.
func (m requestMetadata) clientMetadata() map[string]string {
	return map[string]string{
		metaInstallationID: m.InstallationID,
		metaSessionID:      m.SessionID,
		metaThreadID:       m.ThreadID,
		metaWindowID:       m.WindowID,
		metaTurnID:         m.TurnID,
		metaTurnMetadata:   m.TurnMetadataJSON,
	}
}

// isWithinTurnContinuation returns true when the request continues the current
// turn (trailing messages are tool results after an assistant message), false
// when a new user turn starts. Mirrors codex-rs turn-state scoping.
func isWithinTurnContinuation(messages []openai.ChatCompletionMessage) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		role := messages[i].Role
		if role == openai.ChatMessageRoleTool {
			continue
		}
		return role == openai.ChatMessageRoleAssistant
	}
	return false
}

// toAsciiJSON marshals v to JSON and escapes every byte 0x7f-0xffff as
// \uXXXX, producing ASCII-safe JSON. Mirrors omp's toAsciiJsonString.
func toAsciiJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	var buf []byte
	for i := 0; i < len(raw); {
		b := raw[i]
		if b < 0x7f {
			buf = append(buf, b)
			i++
			continue
		}

		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			buf = append(buf, []byte(fmt.Sprintf(`\u%04x`, b))...)
			i++
			continue
		}
		if r > 0xffff {
			r -= 0x10000
			hi := 0xD800 + (r >> 10)
			lo := 0xDC00 + (r & 0x3FF)
			buf = append(buf, []byte(fmt.Sprintf(`\u%04x\u%04x`, hi, lo))...)
		} else {
			buf = append(buf, []byte(fmt.Sprintf(`\u%04x`, r))...)
		}
		i += size
	}
	return string(buf), nil
}

// getOrCreateInstallID reads or creates a persistent installation UUID at
// ~/.config/nib/install-id. Atomic create with O_CREAT|O_EXCL, mode 0600.
// On any error, a random UUID is returned in-memory.
func getOrCreateInstallID() string {
	dir := configDir()
	path := filepath.Join(dir, "install-id")

	if existing, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(existing))
		if isValidUUID(id) {
			return id
		}
	}

	id := uuid.NewString()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return id
	}
	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		// Lost the race or other error — try reading what the winner wrote.
		if existing, err := os.ReadFile(path); err == nil {
			if e := strings.TrimSpace(string(existing)); isValidUUID(e) {
				return e
			}
		}
		return id
	}
	fmt.Fprintf(fd, "%s\n", id)
	fd.Close()
	return id
}

func configDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "nib")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "nib")
	}
	return ".nib"
}

func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
