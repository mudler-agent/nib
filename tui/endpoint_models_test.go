package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/types"
)

// fakeEndpointModelsServer serves an OpenAI-compatible /v1/models listing of
// ids, for exercising a named endpoint's own model list.
func fakeEndpointModelsServer(t *testing.T, ids ...string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			data = append(data, map[string]string{"id": id, "object": "model"})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/v1"
}

// newNamedEndpointTestModel is newModelSwitchTestModel ("model-a"/"model-b"
// on the default endpoint) plus one named config.yaml endpoint ("@other")
// pointed at endpointURL, so /models, /model and /endpoint can be exercised
// against a NON-current endpoint without touching the default's fake server.
func newNamedEndpointTestModel(t *testing.T, endpointURL string) Model {
	t.Helper()
	m := newModelSwitchTestModel(t, "model-a", "model-b")
	cfg := m.cfg
	cfg.BaseDir = t.TempDir()
	cfg.Endpoints = types.Endpoints{{
		Name:                "other",
		ModelProviderConfig: types.ModelProviderConfig{BaseURL: endpointURL},
	}}
	s, err := chat.NewSession(context.Background(), cfg, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	m.cfg, m.session = cfg, s
	return m
}

// /models @other lists another endpoint's models without switching to it.
func TestModelsListsAnotherEndpointWithoutSwitching(t *testing.T) {
	url := fakeEndpointModelsServer(t, "llama-3.3-70b", "qwen3-coder-30b")
	m := newNamedEndpointTestModel(t, url)

	if cmd := m.dispatchResolved("/models @other"); cmd != nil {
		t.Fatal("/models <endpoint> must not start a turn")
	}
	msg := lastMessage(t, m)
	if msg.Role != "agent" {
		t.Fatalf("listing posted as %q, want agent", msg.Role)
	}
	if !strings.Contains(msg.Content, "llama-3.3-70b") || !strings.Contains(msg.Content, "qwen3-coder-30b") {
		t.Fatalf("listing missing the endpoint's models: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "@other") {
		t.Fatalf("listing header does not name the endpoint it listed: %q", msg.Content)
	}
	if got := m.session.EndpointID(); got != chat.ConfigProviderID {
		t.Fatalf("EndpointID = %q, want the session left on the default", got)
	}
	if got := m.session.Model(); got != "model-a" {
		t.Fatalf("Model = %q, want the default's model unchanged", got)
	}
}

// A bare /models still lists the current endpoint, unchanged from before
// Endpoint existed on the action.
func TestModelsWithNoEndpointListsTheCurrentOne(t *testing.T) {
	m := newModelSwitchTestModel(t, "model-a", "model-b")

	if cmd := m.dispatchResolved("/models"); cmd != nil {
		t.Fatal("/models must not start a turn")
	}
	msg := lastMessage(t, m)
	if !strings.Contains(msg.Content, "* model-a") || !strings.Contains(msg.Content, "  model-b") {
		t.Fatalf("listing = %q, want both models with the current one marked", msg.Content)
	}
}

// An unknown endpoint reports the lookup error rather than crashing or
// silently listing the current endpoint instead.
func TestModelsUnknownEndpointReportsError(t *testing.T) {
	m := newModelSwitchTestModel(t, "model-a")

	if cmd := m.dispatchResolved("/models @nope"); cmd != nil {
		t.Fatal("/models <unknown> must not start a turn")
	}
	msg := lastMessage(t, m)
	if msg.Role != "error" || !strings.Contains(msg.Content, "@nope") {
		t.Fatalf("message = %+v, want an error naming @nope", msg)
	}
}

// /model reset drops the sticky per-endpoint override and restores the
// endpoint's own model.
func TestModelResetRestoresTheEndpointModel(t *testing.T) {
	m := newModelSwitchTestModel(t, "model-a", "model-b")

	if cmd := m.dispatchResolved("/model model-b"); cmd != nil {
		t.Fatal("/model <name> must not start a turn")
	}
	if got := m.session.Model(); got != "model-b" {
		t.Fatalf("Model after /model model-b = %q, want model-b", got)
	}

	if cmd := m.dispatchResolved("/model reset"); cmd != nil {
		t.Fatal("/model reset must not start a turn")
	}
	if got := m.session.Model(); got != "model-a" {
		t.Fatalf("Model after /model reset = %q, want model-a (the endpoint's own)", got)
	}
	msg := lastMessage(t, m)
	if msg.Role != "agent" || !strings.Contains(msg.Content, "model-a") {
		t.Fatalf("reset notice = %+v, want it to name the model now in use", msg)
	}
}

// /model reset on an endpoint that names no model of its own reports the
// error instead of a false success.
func TestModelResetErrorsWhenEndpointNamesNoModelOfItsOwn(t *testing.T) {
	url := fakeEndpointModelsServer(t, "picked-model")
	m := newNamedEndpointTestModel(t, url)

	// Switch onto the model-less named endpoint with an explicitly typed
	// model, the way useEndpoint's picker chain would after Enter.
	if err := m.session.SwitchProvider("@other", "picked-model"); err != nil {
		t.Fatalf("SwitchProvider: %v", err)
	}

	if cmd := m.dispatchResolved("/model reset"); cmd != nil {
		t.Fatal("/model reset must not start a turn")
	}
	msg := lastMessage(t, m)
	if msg.Role != "error" || !strings.Contains(msg.Content, "@other") {
		t.Fatalf("reset error = %+v, want an error naming @other and no model of its own", msg)
	}
	if got := m.session.Model(); got != "picked-model" {
		t.Fatalf("Model = %q, want the failed reset to leave the pick untouched", got)
	}
}

// Selecting a named endpoint that names no model of its own chains straight
// into its model picker, populated from a real listing (Task 10's
// useEndpoint -> openProviderModelPicker, exercised here against a fake
// endpoint that actually answers /v1/models).
func TestSelectingAModellessEndpointOpensItsPopulatedModelPicker(t *testing.T) {
	url := fakeEndpointModelsServer(t, "llama-3.3-70b")
	m := newNamedEndpointTestModel(t, url)

	cmd := m.dispatchResolved("/endpoint @other")
	if cmd == nil {
		t.Fatal("a model-less endpoint must chain into the (asynchronous) model picker")
	}
	if m.session.EndpointID() != chat.ConfigProviderID {
		t.Fatal("the session must not switch before the picker's Enter")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if !m.modelPicker.active || m.modelPicker.loading {
		t.Fatalf("picker state = %+v, want active and loaded", m.modelPicker)
	}
	if len(m.modelPicker.all) != 1 || m.modelPicker.all[0] != "llama-3.3-70b" {
		t.Fatalf("picker models = %v, want the endpoint's own listing", m.modelPicker.all)
	}

	m, _ = press(t, m, keyEnter)
	if m.modelPicker.active {
		t.Fatal("Enter should switch and close the picker")
	}
	if got := m.session.EndpointID(); got != "@other" {
		t.Fatalf("EndpointID = %q, want @other", got)
	}
	if got := m.session.Model(); got != "llama-3.3-70b" {
		t.Fatalf("Model = %q, want llama-3.3-70b", got)
	}
}

// An unreachable endpoint's model listing must not dead-end the picker: the
// error shows, and a typed model name still switches.
func TestUnreachableEndpointShowsItsErrorAndAcceptsATypedModel(t *testing.T) {
	m := newNamedEndpointTestModel(t, "http://127.0.0.1:1/v1")

	cmd := m.dispatchResolved("/endpoint @other")
	if cmd == nil {
		t.Fatal("/endpoint @other must chain into the model picker")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if !m.modelPicker.active {
		t.Fatal("the picker closed silently on a listing failure")
	}
	if !m.modelPicker.typed {
		t.Fatal("a listing failure must still let a model name be typed")
	}

	dlg := m.buildModelPickerDialog()
	if !strings.Contains(strings.ToLower(dlg.Hint), "could not list") {
		t.Fatalf("hint = %q, want it to explain the listing failed", dlg.Hint)
	}
	if !strings.Contains(dlg.Hint, "@other") {
		t.Fatalf("hint = %q, want it to name the endpoint that failed", dlg.Hint)
	}
	if !strings.Contains(dlg.Hint, theme.ModelPickerTypeName) {
		t.Fatalf("hint = %q, want it to still say a model name can be typed", dlg.Hint)
	}

	m, _ = press(t, m, typeText("typed-model"), keyEnter)
	if m.modelPicker.active {
		t.Fatal("Enter on the typed name should switch and close the picker")
	}
	if got := m.session.EndpointID(); got != "@other" {
		t.Fatalf("EndpointID = %q, want @other", got)
	}
	if got := m.session.Model(); got != "typed-model" {
		t.Fatalf("Model = %q, want the typed fallback to switch", got)
	}
}

// A failed provider switch from the model picker (Enter with target set)
// must report the failure instead of a false success notice — the same
// discard-the-error bug this task fixes for the plain /model pick
// (target == nil, tui/model.go's other SetModel call site), pinned here on
// its sibling branch since SwitchProvider's own build failure is reachable
// through a public API (an unknown provider ID) where a bare SetModel's is
// not: every registered protocol's factory tolerates an unreachable or
// misconfigured endpoint and only errors on a provider ID the registry does
// not recognize, which target-less /model has no way to produce.
func TestModelPickerSwitchFailureReportsErrorNotSuccess(t *testing.T) {
	m := newModelSwitchTestModel(t, "model-a", "model-b")
	m.modelPicker.open(1)
	m.modelPicker.target = &chat.ProviderEntry{ID: "not-a-real-provider", Name: "broken"}
	m.modelPicker.setModels([]string{"model-x"}, "")
	m.modelPicker.selected = 0

	m, _ = press(t, m, keyEnter)
	if m.modelPicker.active {
		t.Fatal("Enter should still close the picker on a failed switch")
	}
	msg := lastMessage(t, m)
	if msg.Role != "error" {
		t.Fatalf("message = %+v, want an error, not a false success notice", msg)
	}
}
