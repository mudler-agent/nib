package geminicli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider/registry"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

func init() {
	registry.Register(provider.ProtocolGeminiCLI, factory)
	auth.RegisterPostExchange("google-gemini-cli", postExchangeHook)
}

func factory(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, _ float32) (cogito.LLM, error) {
	resolved, err := auth.ResolveOrError(store, def, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("gemini-cli: %w", err)
	}

	// Build the credential JSON blob the adapter expects. For OAuth
	// credentials we include projectId, refreshToken, expiry, and email
	// from the stored credential.
	credBlob := map[string]any{
		"token": resolved.APIKey,
	}
	if store != nil {
		if cred, ok, err := store.Get(def.ID); err == nil && ok {
			if cred.ProjectID != "" {
				credBlob["projectId"] = cred.ProjectID
			}
			if cred.RefreshToken != "" {
				credBlob["refreshToken"] = cred.RefreshToken
			}
			if !cred.ExpiresAt.IsZero() {
				credBlob["expiresAt"] = cred.ExpiresAt.UnixMilli()
			}
			if cred.Email != "" {
				credBlob["email"] = cred.Email
			}
		}
	}

	credJSON, err := json.Marshal(credBlob)
	if err != nil {
		return nil, fmt.Errorf("gemini-cli: marshal credentials: %w", err)
	}

	return New(Config{
		Model:      config.Model,
		BaseURL:    orDefault(config.BaseURL, def.BaseURL),
		Credential: string(credJSON),
	}), nil
}

// postExchangeHook runs after the OAuth token exchange for google-gemini-cli.
// It discovers (or provisions) a Cloud Code Assist project ID and attaches
// it to the credential. It also validates that a refresh token was received.
func postExchangeHook(ctx context.Context, cred auth.Credential, def provider.Definition) (auth.Credential, error) {
	if cred.RefreshToken == "" {
		return cred, fmt.Errorf("gemini-cli: no refresh token received — please try again")
	}
	projectID, err := discoverProject(ctx, cred.AccessToken)
	if err != nil {
		return cred, fmt.Errorf("gemini-cli: discover project: %w", err)
	}
	cred.ProjectID = projectID
	return cred, nil
}


