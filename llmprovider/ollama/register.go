package ollama

import (
	"github.com/mudler/cogito"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider/registry"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

func init() {
	registry.Register(provider.ProtocolOllamaChat, factory)
}

func factory(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, _ float32) (cogito.LLM, error) {
	resolved, _ := auth.Resolve(store, def, config.APIKey)
	return New(Config{
		Model:   config.Model,
		BaseURL: orDefault(config.BaseURL, def.BaseURL),
		APIKey:  resolved.APIKey,
	}), nil
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
