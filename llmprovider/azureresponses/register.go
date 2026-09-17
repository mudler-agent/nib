package azureresponses

import (
	"fmt"
	"os"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider/registry"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

func init() {
	registry.Register(provider.ProtocolAzureResponses, factory)
}

func factory(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, _ float32) (cogito.LLM, error) {
	resolved, _ := auth.Resolve(store, def, config.APIKey)

	apiKey := resolved.APIKey
	if apiKey == "" {
		return nil, fmt.Errorf("azure-responses: no API key — set %s or run 'nib login %s'", def.EnvVar, def.ID)
	}

	baseURL := resolveBaseURL(orDefault(config.BaseURL, def.BaseURL))
	if baseURL == "" {
		return nil, fmt.Errorf("azure-responses: no base URL — set AZURE_OPENAI_BASE_URL or AZURE_OPENAI_RESOURCE_NAME")
	}

	apiVersion := os.Getenv("AZURE_OPENAI_API_VERSION")

	return New(Config{
		Model:          config.Model,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		APIVersion:     apiVersion, // New() defaults to "v1" when empty
		DeploymentName: resolveDeploymentName(config.Model),
	}), nil
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
