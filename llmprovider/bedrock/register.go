package bedrock

import (
	"os"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider/registry"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

func init() {
	registry.Register(provider.ProtocolBedrockConverse, factory)
}

func factory(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, _ float32) (cogito.LLM, error) {
	resolved, _ := auth.Resolve(store, def, config.APIKey)

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	profile := os.Getenv("AWS_PROFILE")

	return New(Config{
		Model:       config.Model,
		BaseURL:     orDefault(config.BaseURL, def.BaseURL),
		BearerToken: resolved.APIKey,
		Region:      region,
		Profile:     profile,
	}), nil
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
