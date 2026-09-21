package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestMergeMetadata(t *testing.T) {
	cases := []struct {
		name     string
		global   map[string]string
		override map[string]string
		want     map[string]string
	}{
		{"both empty -> nil", nil, nil, nil},
		{"global only", map[string]string{"enable_thinking": "false"}, nil, map[string]string{"enable_thinking": "false"}},
		{"override only", nil, map[string]string{"enable_thinking": "true"}, map[string]string{"enable_thinking": "true"}},
		{
			"override wins per key, global-only inherited",
			map[string]string{"enable_thinking": "false", "tier": "low"},
			map[string]string{"enable_thinking": "true"},
			map[string]string{"enable_thinking": "true", "tier": "low"},
		},
	}
	for _, c := range cases {
		got := mergeMetadata(c.global, c.override)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: mergeMetadata(%v, %v) = %v, want %v", c.name, c.global, c.override, got, c.want)
		}
	}
}

func TestMergeMetadataDoesNotMutateInputs(t *testing.T) {
	global := map[string]string{"enable_thinking": "false"}
	override := map[string]string{"enable_thinking": "true"}
	_ = mergeMetadata(global, override)
	if global["enable_thinking"] != "false" {
		t.Errorf("global was mutated: %v", global)
	}
	if override["enable_thinking"] != "true" {
		t.Errorf("override was mutated: %v", override)
	}
}

func TestResolveAgentModel(t *testing.T) {
	main := "qwen-main"
	configured := map[string]bool{"qwen-big": true, "qwen-small": true}

	cases := []struct {
		name      string
		requested string
		want      string
	}{
		{"empty falls back to main", "", main},
		{"main is honored", main, main},
		{"configured agent model is honored", "qwen-big", "qwen-big"},
		{"another configured model is honored", "qwen-small", "qwen-small"},
		{"invented model falls back to main", "sonar", main},
		{"unknown model falls back to main", "gpt-4", main},
	}
	for _, c := range cases {
		if got := resolveAgentModel(c.requested, main, configured); got != c.want {
			t.Errorf("%s: resolveAgentModel(%q) = %q, want %q", c.name, c.requested, got, c.want)
		}
	}
}

func TestAllowedAgentModelsMergesConfigAndEndpoint(t *testing.T) {
	s := &Session{
		agentModels:    map[string]bool{"configured-a": true, "configured-b": true},
		endpointModels: []string{"endpoint-x", "configured-a"}, // overlap is fine
	}
	got := s.allowedAgentModels()
	want := map[string]bool{"configured-a": true, "configured-b": true, "endpoint-x": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("allowedAgentModels() = %v, want %v", got, want)
	}
}

func TestResolveAgentModelHonorsEndpointModel(t *testing.T) {
	main := "main-model"
	allowed := map[string]bool{"endpoint-x": true}
	if got := resolveAgentModel("endpoint-x", main, allowed); got != "endpoint-x" {
		t.Fatalf("endpoint-served model not honored: got %q, want endpoint-x", got)
	}
}

func TestAgentModelGuidanceEmptyWhenNoEndpointModels(t *testing.T) {
	s := &Session{endpointModels: nil}
	if got := s.agentModelGuidance(); got != "" {
		t.Fatalf("agentModelGuidance() = %q, want empty", got)
	}
}

func TestAgentModelGuidanceEmptyWhenOnlyMainModel(t *testing.T) {
	s := &Session{llmModel: "main", endpointModels: []string{"main"}}
	if got := s.agentModelGuidance(); got != "" {
		t.Fatalf("agentModelGuidance() = %q, want empty (only the main model)", got)
	}
}

func TestAgentModelGuidanceListsOthers(t *testing.T) {
	s := &Session{llmModel: "main", endpointModels: []string{"main", "big", "small"}}
	got := s.agentModelGuidance()
	if got == "" {
		t.Fatal("agentModelGuidance() = empty, want a non-empty guidance string")
	}
	if !strings.Contains(got, "big") || !strings.Contains(got, "small") {
		t.Fatalf("agentModelGuidance() = %q, want it to list big and small", got)
	}
	// The main model is mentioned only as the default ("current model main"),
	// never as a choosable entry in the comma-separated list after the colon.
	listPart := got[strings.LastIndex(got, ": ")+2:]
	if strings.Contains(listPart, "main") {
		t.Fatalf("agentModelGuidance() model list = %q, should not include the main model", listPart)
	}
}

// TestNewAgentLLMHonorsEndpointModel: a model the endpoint serves (but that is
// neither the main model nor configured for an agent type) must reach the wire
// when the LLM requests it via spawn_agent's model argument.
func TestNewAgentLLMHonorsEndpointModel(t *testing.T) {
	srv, requests := newLLMServer(t)
	s := &Session{
		llmModel:       "main-model",
		baseURL:        srv.URL + "/v1",
		endpointModels: []string{"main-model", "endpoint-model"},
	}
	askOnce(t, s.newAgentLLM(s.Model(), "endpoint-model", 0, nil))
	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(reqs))
	}
	if reqs[0].Model != "endpoint-model" {
		t.Fatalf("sub-agent model = %q, want endpoint-model", reqs[0].Model)
	}
}

// TestNewAgentLLMFallsBackForInventedModel: a model the endpoint does not serve
// and that is not configured must fall back to the main model even when
// endpointModels is populated — the endpoint list does not open the door to
// arbitrary names.
func TestNewAgentLLMFallsBackForInventedModel(t *testing.T) {
	srv, requests := newLLMServer(t)
	s := &Session{
		llmModel:       "main-model",
		baseURL:        srv.URL + "/v1",
		endpointModels: []string{"main-model", "endpoint-model"},
	}
	askOnce(t, s.newAgentLLM(s.Model(), "invented-model", 0, nil))
	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(reqs))
	}
	if reqs[0].Model != "main-model" {
		t.Fatalf("sub-agent model = %q, want main-model (fallback for invented name)", reqs[0].Model)
	}
}

func TestAgentModelGuidanceHiddenWhenSpawnAgentDisabled(t *testing.T) {
	s := &Session{
		llmModel:       "main",
		endpointModels: []string{"main", "big"},
		toolAllow:      map[string]bool{"bash": true}, // BuiltinTools allowlist without spawn_agent
	}
	if got := s.agentModelGuidance(); got != "" {
		t.Fatalf("agentModelGuidance() = %q, want empty when spawn_agent is not exposed", got)
	}
}

func TestFetchEndpointModelsPopulatesFromEndpoint(t *testing.T) {
	srv := newModelsServer(t, "alpha", "beta")
	defer srv.Close()
	s := &Session{baseURL: srv.URL + "/v1"}
	s.fetchEndpointModels(context.Background())
	if !reflect.DeepEqual(s.endpointModels, []string{"alpha", "beta"}) {
		t.Fatalf("endpointModels = %v, want [alpha beta]", s.endpointModels)
	}
}

func TestFetchEndpointModelsLeavesEmptyOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"nope"}}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	s := &Session{baseURL: srv.URL + "/v1"}
	s.fetchEndpointModels(context.Background())
	if len(s.endpointModels) != 0 {
		t.Fatalf("endpointModels = %v, want empty on endpoint failure", s.endpointModels)
	}
}

func TestAgentModelGuidanceCapsLongList(t *testing.T) {
	models := []string{"main"}
	for i := 0; i < 50; i++ {
		models = append(models, "m"+string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	s := &Session{llmModel: "main", endpointModels: models}
	got := s.agentModelGuidance()
	if !strings.Contains(got, "more (use /models to see the full list)") {
		t.Fatalf("agentModelGuidance() should indicate truncation for >30 models, got: %q", got)
	}
}

// TestAllowedAgentModelsLazyFetchesOnFirstCall: when endpointModels is nil and a
// baseURL is set, the first allowedAgentModels() call fetches the endpoint's
// model list so the returned set includes endpoint-served models.
func TestAllowedAgentModelsLazyFetchesOnFirstCall(t *testing.T) {
	srv := newModelsServer(t, "alpha", "beta")
	s := &Session{
		baseURL:     srv.URL + "/v1",
		agentModels: map[string]bool{"configured": true},
	}
	if s.endpointModels != nil {
		t.Fatalf("precondition: endpointModels should be nil before first call")
	}
	got := s.allowedAgentModels()
	if !got["alpha"] || !got["beta"] || !got["configured"] {
		t.Fatalf("allowedAgentModels() = %v, want alpha, beta, and configured", got)
	}
	if len(s.endpointModels) == 0 {
		t.Fatal("lazy fetch did not populate endpointModels")
	}
}

// TestAllowedAgentModelsSkipsFetchWhenNoBaseURL: with no baseURL and nil
// endpointModels, allowedAgentModels() must not attempt a network call — it
// returns only the config-configured agent models.
func TestAllowedAgentModelsSkipsFetchWhenNoBaseURL(t *testing.T) {
	s := &Session{
		agentModels: map[string]bool{"configured": true},
	}
	got := s.allowedAgentModels()
	if len(got) != 1 || !got["configured"] {
		t.Fatalf("allowedAgentModels() = %v, want only {configured}", got)
	}
	if s.endpointModels != nil {
		t.Fatalf("endpointModels = %v, should be nil (no fetch without baseURL)", s.endpointModels)
	}
}
