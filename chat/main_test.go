package chat

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// isModelProbe reports whether r is one of the requests a session makes to
// learn about its model — the context window and the output cap — rather than
// a chat completion. Both are GETs under /v1/models.
//
// A fake server that scripts its replies by request number has to skip these,
// or the probe is handed the answer the first chat call was meant to get.
func isModelProbe(r *http.Request) bool {
	return r.Method != http.MethodPost
}

// serveEmptyModels answers a probe with a listing that carries no model, which
// leaves the session on its defaults.
func serveEmptyModels(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{}})
}

// TestMain points the default config root at a throwaway directory, so a
// session built with an empty BaseDir never reads the developer's real
// credentials.json or the provider saved by /login (provider.json).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nib-chat-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
