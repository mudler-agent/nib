// Package llmprovider constructs cogito LLM transports from nib configuration.
//
// Native adapters (anthropic, google) self-register via llmprovider/registry
// in their init(). This file registers the OpenAI-compatible factory and
// provides the top-level New / NewWithStore constructors.
//
// Adding a new native protocol: write the adapter package, call
// registry.Register in init(), and blank-import it below. No changes to
// NewWithTemperatureAndStore are needed.
package llmprovider

import (
	"fmt"
	"strings"

	"github.com/mudler/cogito"
	"github.com/mudler/cogito/clients"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/codexapp"
	"github.com/mudler/nib/llmprovider/registry"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"

	// Blank imports trigger init() self-registration.
	_ "github.com/mudler/nib/llmprovider/anthropic"
	_ "github.com/mudler/nib/llmprovider/azureresponses"
	_ "github.com/mudler/nib/llmprovider/bedrock"
	_ "github.com/mudler/nib/llmprovider/codex"
	_ "github.com/mudler/nib/llmprovider/google"
	_ "github.com/mudler/nib/llmprovider/geminicli"
	_ "github.com/mudler/nib/llmprovider/googlevertex"
	_ "github.com/mudler/nib/llmprovider/ollama"
	_ "github.com/mudler/nib/llmprovider/openairesponses"
)

const (
	ProviderOpenAI = "openai"
	ProviderCodex  = "codex"
)

func init() {
	registry.Register(provider.ProtocolOpenAICompletions, openAIFactory)
}

// openAIFactory builds a LocalAI (OpenAI-compatible) LLM for any provider
// whose protocol is ProtocolOpenAICompletions. Credentials are resolved
// from the store / config / env ladder.
func openAIFactory(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, temperature float32) (cogito.LLM, error) {
	resolved, _ := auth.Resolve(store, def, config.APIKey)
	llm := clients.NewLocalAILLM(config.Model, resolved.APIKey, orDefault(config.BaseURL, def.BaseURL))
	llm.SetTemperature(temperature)
	llm.SetMetadata(config.Metadata)
	llm.SetReasoningEffort(config.ReasoningEffort)
	return llm, nil
}

// New returns an independent LLM transport. OpenAI means any
// OpenAI-compatible HTTP endpoint, including a local LocalAI server.
func New(config types.ModelProviderConfig) (cogito.LLM, error) {
	return NewWithTemperature(config, 0)
}

// NewWithTemperature applies agent-specific sampling to OpenAI-compatible
// providers. Codex app-server owns its reasoning configuration and ignores the
// compatibility hint.
func NewWithTemperature(config types.ModelProviderConfig, temperature float32) (cogito.LLM, error) {
	return NewWithTemperatureAndStore(config, temperature, nil)
}

// NewWithStore is like New but resolves credentials from store for providers
// that support /login (anthropic, google, openai-direct).
func NewWithStore(config types.ModelProviderConfig, store *auth.Store) (cogito.LLM, error) {
	return NewWithTemperatureAndStore(config, 0, store)
}

// NewWithTemperatureAndStore is the full constructor: temperature for
// OpenAI-compatible providers, credential store for providers with /login.
// A nil store is valid — the resolver falls through to config/env.
//
// Codex is the only special case (it uses a subprocess, not HTTP). Every
// other provider goes through the registry: the provider Definition is
// looked up, and the registered adapter Factory for its Protocol is called.
func NewWithTemperatureAndStore(config types.ModelProviderConfig, temperature float32, store *auth.Store) (cogito.LLM, error) {
	if normalize(config.Provider) == ProviderCodex {
		return codexapp.New(codexapp.Config{
			Command: config.Command,
			Args:    config.Args,
			Model:   config.Model,
		}), nil
	}

	def, ok := provider.Get(normalize(config.Provider))
	if !ok {
		return nil, fmt.Errorf("unknown LLM provider %q — see 'nib login --list' for available providers", config.Provider)
	}

	factory, ok := registry.Get(def.Protocol)
	if !ok {
		return nil, fmt.Errorf("provider %q: no adapter registered for protocol %q", config.Provider, def.Protocol)
	}

	return factory(def, config, store, temperature)
}

func normalize(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "openai", "openai-compatible", "openai_compatible":
		return ProviderOpenAI
	case "codex", "codex-app-server", "codex_app_server":
		return ProviderCodex
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func IsCodex(config types.ModelProviderConfig) bool {
	return normalize(config.Provider) == ProviderCodex
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
