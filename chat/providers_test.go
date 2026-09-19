package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider"
	"github.com/mudler/nib/types"
)

func newProviderSession(t *testing.T, cfg types.ModelProviderConfig) *Session {
	t.Helper()
	return &Session{
		ctx:            context.Background(),
		llmModel:       cfg.Model,
		mainProvider:   cfg,
		configProvider: cfg,
		providerID:     ConfigProviderID,
		credStore:      auth.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}
}

func TestProvidersReportLoginState(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "gsk-env")
	s := newProviderSession(t, types.ModelProviderConfig{Provider: "openai", Model: "local", BaseURL: "http://localhost:8080/v1"})
	if _, err := s.SaveAPIKey("regolo", "  rg-secret-1234 ", "ignored"); err != nil {
		t.Fatal(err)
	}

	byID := map[string]ProviderEntry{}
	for _, e := range s.Providers() {
		byID[e.ID] = e
	}
	if e := byID[ConfigProviderID]; !e.Current || !e.Ready || e.Status != "local @ http://localhost:8080/v1" {
		t.Fatalf("config entry = %+v", e)
	}
	if e := byID["regolo"]; !e.Stored || !e.Ready || e.Current {
		t.Fatalf("regolo entry = %+v", e)
	}
	if e := byID["groq"]; e.Stored || !e.Ready {
		t.Fatalf("groq (env key) entry = %+v", e)
	}
	if e := byID["mistral"]; e.Ready {
		t.Fatalf("mistral (no key) entry = %+v", e)
	}
	if !byID["azure"].NeedsBaseURL || byID["regolo"].NeedsBaseURL {
		t.Fatal("only providers without a default endpoint should ask for a base URL")
	}

	// The base URL is kept only where the provider needs one.
	c, _, _ := s.credStore.Get("regolo")
	if c.APIKey != "rg-secret-1234" || c.BaseURL != "" {
		t.Fatalf("stored regolo credential = %+v", c)
	}
}

func TestSwitchProviderAndBack(t *testing.T) {
	srv, requests := newLLMServer(t)
	s := newProviderSession(t, types.ModelProviderConfig{Provider: "openai", Model: "local", BaseURL: srv.URL + "/v1", APIKey: "local-key"})
	if _, err := s.SaveAPIKey("regolo", "rg-key", ""); err != nil {
		t.Fatal(err)
	}

	if err := s.SwitchProvider("regolo", ""); err == nil {
		t.Fatal("switching without a model must fail rather than keep a model the new provider does not serve")
	}
	if err := s.SwitchProvider("regolo", "Llama-3.3-70B-Instruct"); err != nil {
		t.Fatal(err)
	}
	if s.ProviderID() != "regolo" || s.Model() != "Llama-3.3-70B-Instruct" {
		t.Fatalf("after switch: provider=%q model=%q", s.ProviderID(), s.Model())
	}
	p := s.resolvedSessionProvider()
	if p.Provider != "regolo" || p.BaseURL != "" || p.APIKey != "" {
		t.Fatalf("regolo config = %+v, want the registry endpoint with the stored key", p)
	}
	if base, key, _ := llmprovider.ModelsEndpoint(p, s.credStore); key != "rg-key" || base != "https://api.regolo.ai/v1" {
		t.Fatalf("regolo endpoint = %q key %q", base, key)
	}

	// Back to config.yaml: the configured endpoint and key, and its model.
	if err := s.SwitchProvider(ConfigProviderID, ""); err != nil {
		t.Fatal(err)
	}
	if s.ProviderID() != ConfigProviderID || s.Model() != "local" {
		t.Fatalf("after switching back: provider=%q model=%q", s.ProviderID(), s.Model())
	}
	askOnce(t, s.llm)
	if reqs := requests(); len(reqs) != 1 || reqs[0].Model != "local" {
		t.Fatalf("requests after switching back = %+v", reqs)
	}
}

func TestListProviderModelsWithoutListing(t *testing.T) {
	s := newProviderSession(t, types.ModelProviderConfig{Provider: "openai", Model: "local", BaseURL: "http://unused.invalid/v1"})
	// Azure's adapter has no model listing (deployments are per account).
	t.Setenv("AZURE_OPENAI_API_KEY", "az-key")
	t.Setenv("AZURE_OPENAI_BASE_URL", "https://example.openai.azure.com/openai/v1")
	if _, err := s.ListProviderModels(context.Background(), "azure"); !errors.Is(err, llmprovider.ErrNoModelList) {
		t.Fatalf("azure listing err = %v, want ErrNoModelList", err)
	}
	if _, err := s.ListProviderModels(context.Background(), "nope"); err == nil {
		t.Fatal("unknown provider was accepted")
	}
}

func TestSwitchProviderPersistsTheDefault(t *testing.T) {
	cfg := types.ModelProviderConfig{Provider: "openai", Model: "local", BaseURL: "http://unused.invalid/v1"}
	s := newProviderSession(t, cfg)
	s.providerStatePath = filepath.Join(t.TempDir(), ProviderStateFile)
	if _, err := s.SaveAPIKey("regolo", "rg-key", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SwitchProvider("regolo", "model-one"); err != nil {
		t.Fatal(err)
	}
	s.SetModel("model-two") // /model on a /login provider updates the default

	// A fresh session over the same state starts where the last one left off.
	next := newProviderSession(t, cfg)
	next.credStore, next.providerStatePath = s.credStore, s.providerStatePath
	next.restoreDefaultProvider()
	if next.ProviderID() != "regolo" || next.Model() != "model-two" {
		t.Fatalf("restored provider=%q model=%q, want regolo/model-two", next.ProviderID(), next.Model())
	}

	// Picking config.yaml again forgets the default.
	if err := next.SwitchProvider(ConfigProviderID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.providerStatePath); !os.IsNotExist(err) {
		t.Fatalf("provider.json should be removed after switching back, stat err=%v", err)
	}
	fresh := newProviderSession(t, cfg)
	fresh.providerStatePath = s.providerStatePath
	fresh.restoreDefaultProvider()
	if fresh.ProviderID() != ConfigProviderID || fresh.Model() != "local" {
		t.Fatalf("with no saved default: provider=%q model=%q", fresh.ProviderID(), fresh.Model())
	}
}

func TestRestoreIgnoresUnknownProvider(t *testing.T) {
	cfg := types.ModelProviderConfig{Provider: "openai", Model: "local", BaseURL: "http://unused.invalid/v1"}
	s := newProviderSession(t, cfg)
	s.providerStatePath = filepath.Join(t.TempDir(), ProviderStateFile)
	if err := writeSavedProvider(s.providerStatePath, savedProvider{Provider: "gone-provider", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	s.restoreDefaultProvider()
	if s.ProviderID() != ConfigProviderID || s.Model() != "local" {
		t.Fatalf("an unknown saved provider must leave config.yaml in charge: %q/%q", s.ProviderID(), s.Model())
	}
}

func TestSwitchModelHonoursPartialLists(t *testing.T) {
	t.Setenv("AZURE_OPENAI_API_KEY", "az-key")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT_NAME_MAP", "gpt-4.1=prod")
	s := newProviderSession(t, types.ModelProviderConfig{Provider: "azure", Model: "gpt-4.1", BaseURL: "https://example.openai.azure.com/openai/v1"})

	// A deployment the map does not name is still accepted: the list is a
	// suggestion, and refusing it would lock the user out of it.
	notice, err := s.SwitchModel(context.Background(), "other-deployment")
	if err != nil || s.Model() != "other-deployment" || !strings.Contains(notice, "suggested") {
		t.Fatalf("partial list: notice=%q err=%v model=%q", notice, err, s.Model())
	}

	ids, partial, err := s.ModelChoices(context.Background(), "")
	if err != nil || !partial || len(ids) != 1 {
		t.Fatalf("ModelChoices = %v, %v, %v", ids, partial, err)
	}
}

// The TUI's boot log, header and /model picker all name the provider the
// session really talks to. A saved /login pick overrides config.yaml's model
// at startup, so reading config.yaml there showed a model no request used.
func TestActiveProviderNameFollowsTheLogin(t *testing.T) {
	s := newProviderSession(t, types.ModelProviderConfig{Provider: "openai", Model: "uncensored", BaseURL: "http://localhost:8080/v1"})
	s.providerStatePath = filepath.Join(t.TempDir(), ProviderStateFile)

	if got := s.ActiveProviderName(); got != "config.yaml" {
		t.Fatalf("ActiveProviderName on config.yaml = %q, want config.yaml", got)
	}
	if s.SavesModelAsDefault() {
		t.Fatal("config.yaml owns its model: a /model pick there is not saved as the default")
	}
	if got := s.ConfigModel(); got != "uncensored" {
		t.Fatalf("ConfigModel = %q, want uncensored", got)
	}

	if _, err := s.SaveAPIKey("regolo", "rg-secret", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SwitchProvider("regolo", "glm5.2"); err != nil {
		t.Fatal(err)
	}
	if got := s.ActiveProviderName(); got != "Regolo" {
		t.Fatalf("ActiveProviderName after /login = %q, want Regolo", got)
	}
	if !s.SavesModelAsDefault() {
		t.Fatal("a /model pick on a /login provider is saved to provider.json")
	}
	if got := s.ConfigModel(); got != "uncensored" {
		t.Fatalf("ConfigModel after /login = %q, want config.yaml's model unchanged", got)
	}
	if got := s.Model(); got != "glm5.2" {
		t.Fatalf("Model = %q, want glm5.2", got)
	}
}

func TestFormatProviderModelListNamesTheProvider(t *testing.T) {
	got := FormatProviderModelList("Regolo", []string{"a", "b"}, "b")
	want := "Regolo models:\n  a\n* b\n"
	if got != want {
		t.Fatalf("FormatProviderModelList = %q, want %q", got, want)
	}
}
