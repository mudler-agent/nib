package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/mudler/xlog"

	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/endpoint"
	"github.com/mudler/nib/provider"
)

// ConfigProviderID is endpoint.DefaultID under its pre-endpoints name, kept
// so existing call sites and embedders keep compiling.
const ConfigProviderID = endpoint.DefaultID

// ConfigProviderName is how the UI names the default endpoint.
const ConfigProviderName = endpoint.DefaultName

// ProviderEntry is one row of the provider picker: a registry provider, a
// config.yaml named endpoint, or the config.yaml default endpoint, with its
// login state.
type ProviderEntry struct {
	ID        string
	Name      string
	Kind      endpoint.Kind
	LoginKind provider.LoginKind
	// Ready means requests can authenticate right now: a stored login, the
	// provider's environment variable, or no credential needed at all.
	Ready bool
	// Stored means a /login credential exists (so /logout can remove it).
	Stored bool
	// Status is a short human description of Ready/Stored.
	Status string
	// Current marks the endpoint the session is talking to.
	Current bool
	// NeedsBaseURL means logging in must also collect an endpoint.
	NeedsBaseURL bool
	// EnvVar is the environment variable that can hold the key instead.
	EnvVar string
	// Model is what this entry would run, empty when it must be picked.
	Model string
}

// EndpointID returns the picker ID of the endpoint the session is currently
// talking to: ConfigProviderID until a named endpoint or /login provider is
// selected.
func (s *Session) EndpointID() string {
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()
	return s.endpointID
}

// ProviderID is EndpointID under its pre-endpoints name.
func (s *Session) ProviderID() string { return s.EndpointID() }

// StartupNote is the non-empty explanation when a saved pick could not be
// honored at startup, for the boot log. Empty otherwise.
func (s *Session) StartupNote() string { return s.startupNote }

// ConfigErrors are the config.yaml endpoints that were rejected at load.
func (s *Session) ConfigErrors() []error { return s.configErrs }

// ActiveProviderName is the display name of the endpoint the session is
// talking to right now: the registry name of a /login provider ("Regolo"),
// a config.yaml named endpoint ("@work"), or ConfigProviderName while
// config.yaml's default endpoint is in use.
//
// It is the single source of truth for "which provider" in the UI. A saved
// pick overrides config.yaml at startup (restoreStartupEndpoint), so a front
// end that read types.Config.Provider/Model instead would name an endpoint
// and model that no request goes to.
func (s *Session) ActiveProviderName() string {
	id := s.EndpointID()
	if id == "" || id == ConfigProviderID {
		return ConfigProviderName
	}
	if e, ok := s.endpoints.Lookup(id); ok {
		return e.Name
	}
	if def, ok := provider.Get(id); ok && def.Name != "" {
		return def.Name
	}
	return id
}

// ConfigModel is the model config.yaml names for its own endpoint, whichever
// provider is active. A UI compares it with Model() to explain that a /login
// selection overrides it, instead of silently showing one or the other.
func (s *Session) ConfigModel() string {
	return s.configProvider.Model
}

// SavesModelAsDefault reports whether SetModel also records the model as the
// startup default (provider.json).
func (s *Session) SavesModelAsDefault() bool {
	return s.savedPath != ""
}

// Providers lists every endpoint the pickers offer: the config.yaml default
// endpoint, its named endpoints, then the registry in its display order.
func (s *Session) Providers() []ProviderEntry {
	current := s.EndpointID()
	var out []ProviderEntry
	for _, e := range s.endpoints.List() {
		out = append(out, ProviderEntry{
			ID:           e.ID,
			Name:         e.Name,
			Kind:         e.Kind,
			LoginKind:    e.Def.LoginKind,
			Ready:        e.Ready,
			Stored:       e.Stored,
			Status:       e.Status,
			Current:      e.ID == current,
			NeedsBaseURL: e.NeedsBaseURL,
			EnvVar:       e.EnvVar,
			Model:        e.Model,
		})
	}
	return out
}

// ListProviderModels lists the models the given picker entry serves, so a UI
// can offer them before switching. llmprovider.ErrNoModelList means the
// provider has no listing nib can query: ask for a model name instead.
func (s *Session) ListProviderModels(ctx context.Context, id string) ([]string, error) {
	p, err := s.endpoints.Config(id)
	if err != nil {
		return nil, err
	}
	return s.listModels(ctx, p)
}

// ModelChoices lists the models of a picker entry ("" = the current provider)
// and whether the list is partial, in which case a UI should also accept a
// typed model name.
func (s *Session) ModelChoices(ctx context.Context, id string) ([]string, bool, error) {
	if id == "" {
		return s.modelChoices(ctx, s.resolvedSessionProvider())
	}
	p, err := s.endpoints.Config(id)
	if err != nil {
		return nil, false, err
	}
	return s.modelChoices(ctx, p)
}

// SwitchProvider points the session at an endpoint and model, rebuilding the
// LLM the way SetModel does. Conversation history is kept.
func (s *Session) SwitchProvider(id, model string) error {
	p, err := s.endpoints.Config(id)
	if err != nil {
		return err
	}
	if model = strings.TrimSpace(model); model != "" {
		p.Model = model
	}
	if p.Model == "" {
		return fmt.Errorf("no model selected for %s", id)
	}
	if err := s.applyProvider(p, id); err != nil {
		return err
	}
	return endpoint.WriteSaved(s.savedPath, endpoint.Saved{ID: id, Model: p.Model})
}

// ProviderStateFile holds the endpoint picked via the picker or /login, next
// to credentials.json, so the next session starts on it. It is nib-managed
// state rather than a config.yaml edit: config.yaml keeps describing its own
// endpoints, which the picker's config.yaml entry switches back to.
const ProviderStateFile = "provider.json"

// restoreStartupEndpoint puts a new session on the endpoint the last one
// picked. A pick that no longer resolves leaves the session on the default
// and records why, for the boot log.
func (s *Session) restoreStartupEndpoint() {
	e, p, note := s.endpoints.Startup(endpoint.LoadSaved(s.savedPath))
	s.startupNote = note
	if e.ID == endpoint.DefaultID && note == "" {
		return // already built from the default
	}
	if p.Model == "" {
		return
	}
	if err := s.applyProvider(p, e.ID); err != nil {
		xlog.Warn("could not start on the saved endpoint; using config.yaml", "endpoint", e.ID, "error", err)
	}
}

// SaveAPIKey stores an API-key login for a provider (the TUI's key form).
// baseURL is only kept for providers that need one (ProviderEntry.NeedsBaseURL).
func (s *Session) SaveAPIKey(id, key, baseURL string) (auth.Credential, error) {
	def, ok := provider.Get(id)
	if !ok {
		return auth.Credential{}, fmt.Errorf("unknown provider %q", id)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return auth.Credential{}, fmt.Errorf("the API key is empty")
	}
	if !def.NeedsBaseURL() {
		baseURL = ""
	}
	return auth.LoginAPIKeyAt(s.credStore, def, key, baseURL)
}
