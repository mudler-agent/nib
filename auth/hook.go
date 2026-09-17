package auth

import (
	"context"

	"github.com/mudler/nib/provider"
)

// PostExchangeHook is called after a successful OAuth token exchange but
// before the credential is saved. It can enrich the credential (e.g.,
// discover a Cloud Code Assist project ID) or validate the exchange (e.g.,
// require a refresh token). Returning an error aborts the login.
//
// Hooks are registered per provider ID via RegisterPostExchange, typically
// from an adapter package's init().
type PostExchangeHook func(ctx context.Context, cred Credential, def provider.Definition) (Credential, error)

var postExchangeHooks = map[string]PostExchangeHook{}

// RegisterPostExchange registers a post-exchange hook for a provider ID.
// Intended to be called from init() in adapter packages.
func RegisterPostExchange(providerID string, hook PostExchangeHook) {
	postExchangeHooks[providerID] = hook
}

// GetPostExchange returns the post-exchange hook for a provider, or nil.
func GetPostExchange(providerID string) PostExchangeHook {
	return postExchangeHooks[providerID]
}
