package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mudler/xlog"

	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

// ConfigProviderID names the provider-picker entry for the endpoint
// config.yaml describes, as opposed to a registry provider reached via /login.
const ConfigProviderID = "config"

// ProviderEntry is one row of the provider picker: a registry provider (or the
// config.yaml endpoint) with its login state.
type ProviderEntry struct {
	ID        string
	Name      string
	LoginKind provider.LoginKind
	// Ready means requests can authenticate right now: a stored login, the
	// provider's environment variable, or no credential needed at all.
	Ready bool
	// Stored means a /login credential exists (so /logout can remove it).
	Stored bool
	// Status is a short human description of Ready/Stored.
	Status string
	// Current marks the provider the session is talking to.
	Current bool
	// NeedsBaseURL means logging in must also collect an endpoint.
	NeedsBaseURL bool
	// EnvVar is the environment variable that can hold the key instead.
	EnvVar string
}

// ProviderID returns the provider-picker ID of the provider the session is
// currently using: ConfigProviderID until a /login provider is selected.
func (s *Session) ProviderID() string {
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()
	return s.providerID
}

// ConfigProviderName is how the UI names the config.yaml endpoint: the same
// label its /login picker row carries, so the header, boot log and pickers
// all agree on where the model comes from.
const ConfigProviderName = "config.yaml"

// ActiveProviderName is the display name of the provider the session is
// talking to right now: the registry name of a /login provider ("Regolo"), or
// ConfigProviderName while config.yaml's endpoint is in use.
//
// It is the single source of truth for "which provider" in the UI. A saved
// /login pick overrides config.yaml at startup (restoreDefaultProvider), so a
// front end that read types.Config.Provider/Model instead would name an
// endpoint and model that no request goes to.
func (s *Session) ActiveProviderName() string {
	id := s.ProviderID()
	if id == "" || id == ConfigProviderID {
		return ConfigProviderName
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
// startup default (provider.json). That only happens on a /login provider:
// config.yaml owns the model for its own endpoint, and nib never edits it.
func (s *Session) SavesModelAsDefault() bool {
	id := s.ProviderID()
	return s.providerStatePath != "" && id != "" && id != ConfigProviderID
}

// Providers lists every provider the picker offers: the config.yaml endpoint
// first, then the registry in its display order.
func (s *Session) Providers() []ProviderEntry {
	stored := map[string]auth.Credential{}
	if creds, err := s.credStore.All(); err == nil {
		for _, c := range creds {
			stored[c.ProviderID] = c
		}
	}
	current := s.ProviderID()

	out := []ProviderEntry{{
		ID:      ConfigProviderID,
		Name:    ConfigProviderName,
		Ready:   true,
		Status:  describeConfigEndpoint(s.configProvider),
		Current: current == ConfigProviderID,
	}}
	for _, d := range provider.All() {
		e := ProviderEntry{
			ID:           d.ID,
			Name:         d.Name,
			LoginKind:    d.LoginKind,
			Current:      current == d.ID,
			NeedsBaseURL: d.NeedsBaseURL(),
			EnvVar:       d.EnvVar,
		}
		c, hasCred := stored[d.ID]
		switch {
		case hasCred:
			e.Ready, e.Stored = true, true
			e.Status = "logged in · " + c.StatusLine()
		case d.EnvVar != "" && os.Getenv(d.EnvVar) != "":
			e.Ready = true
			e.Status = "key from $" + d.EnvVar
		case d.LoginKind == provider.LoginNone:
			// Local servers need nothing; cloud SDK providers (Bedrock,
			// Vertex) read their own environment at request time.
			e.Ready = true
			e.Status = "no login needed"
		default:
			e.Status = "not logged in"
		}
		out = append(out, e)
	}
	return out
}

func describeConfigEndpoint(p types.ModelProviderConfig) string {
	where := p.BaseURL
	if where == "" {
		where = p.Provider
	}
	if p.Model == "" {
		return where
	}
	return p.Model + " @ " + where
}

// providerConfig builds the connection config for a picker entry without
// switching to it. Metadata and reasoning effort carry over from the session.
func (s *Session) providerConfig(id string) (types.ModelProviderConfig, error) {
	if id == ConfigProviderID {
		return s.configProvider, nil
	}
	def, ok := provider.Get(id)
	if !ok {
		return types.ModelProviderConfig{}, fmt.Errorf("unknown provider %q", id)
	}
	cur := s.resolvedSessionProvider()
	p := types.ModelProviderConfig{
		Provider:        def.ID,
		Metadata:        cur.Metadata,
		ReasoningEffort: cur.ReasoningEffort,
	}
	// config.yaml may already point at this provider with its own key: keep
	// it, unless its base_url sends the provider somewhere else entirely.
	if cfg := s.configProvider; cfg.Provider == def.ID &&
		(cfg.BaseURL == "" || strings.TrimRight(cfg.BaseURL, "/") == strings.TrimRight(def.BaseURL, "/")) {
		p.APIKey, p.BaseURL = cfg.APIKey, cfg.BaseURL
	}
	if c, ok, err := s.credStore.Get(def.ID); err == nil && ok && c.BaseURL != "" {
		p.BaseURL = c.BaseURL
	}
	return p, nil
}

// ListProviderModels lists the models the given picker entry serves, so a UI
// can offer them before switching. llmprovider.ErrNoModelList means the
// provider has no listing nib can query: ask for a model name instead.
func (s *Session) ListProviderModels(ctx context.Context, id string) ([]string, error) {
	p, err := s.providerConfig(id)
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
	p, err := s.providerConfig(id)
	if err != nil {
		return nil, false, err
	}
	return s.modelChoices(ctx, p)
}

// SwitchProvider points the session at a picker entry and model, rebuilding
// the LLM the way SetModel does. Conversation history is kept.
func (s *Session) SwitchProvider(id, model string) error {
	p, err := s.providerConfig(id)
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
	s.saveDefaultProvider(id, p.Model)
	return nil
}

// ProviderStateFile holds the provider picked with /login, next to
// credentials.json, so the next session starts on it. It is nib-managed state
// rather than a config.yaml edit: config.yaml keeps describing its own
// endpoint, which the picker's config.yaml entry switches back to (and
// picking that entry removes this file).
const ProviderStateFile = "provider.json"

type savedProvider struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// saveDefaultProvider records id/model as the startup default. Best-effort:
// the switch itself already happened, so a write failure is only logged.
func (s *Session) saveDefaultProvider(id, model string) {
	if s.providerStatePath == "" {
		return
	}
	if id == ConfigProviderID {
		if err := os.Remove(s.providerStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			xlog.Warn("could not clear the default provider", "error", err)
		}
		return
	}
	if err := writeSavedProvider(s.providerStatePath, savedProvider{Provider: id, Model: model}); err != nil {
		xlog.Warn("could not save the default provider", "provider", id, "error", err)
	}
}

func writeSavedProvider(path string, v savedProvider) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// restoreDefaultProvider switches a new session to the provider saved by a
// previous /login pick. An unknown provider or a failing client leaves the
// session on config.yaml's endpoint.
func (s *Session) restoreDefaultProvider() {
	if s.providerStatePath == "" {
		return
	}
	data, err := os.ReadFile(s.providerStatePath)
	if err != nil {
		return
	}
	var v savedProvider
	if err := json.Unmarshal(data, &v); err != nil || v.Provider == "" || v.Provider == ConfigProviderID {
		return
	}
	p, err := s.providerConfig(v.Provider)
	if err == nil {
		p.Model = v.Model
		if p.Model == "" {
			err = fmt.Errorf("no model saved")
		} else {
			err = s.applyProvider(p, v.Provider)
		}
	}
	if err != nil {
		xlog.Warn("could not restore the default provider; using config.yaml", "provider", v.Provider, "error", err)
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
