package copilot

import (
	"fmt"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider/registry"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

func init() {
	registry.Register(provider.ProtocolCopilot, factory)
}

func factory(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, _ float32) (cogito.LLM, error) {
	// 1. Try stored credential / config API key / env var.
	resolved, _ := auth.Resolve(store, def, config.APIKey)
	token := resolved.APIKey

	// 2. Fall back to Copilot-specific token sources (env vars, gh CLI
	//    config files).
	if token == "" {
		t, err := ResolveToken()
		if err != nil {
			return nil, fmt.Errorf("copilot: no token — run 'nib login github-copilot' or set GH_COPILOT_TOKEN")
		}
		token = t
	}

	return New(Config{
		Model: config.Model,
		Token: token,
	}), nil
}
