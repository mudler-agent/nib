// Package auth holds the credential store and resolution ladder for remote
// LLM providers. It is the runtime side of package provider: provider knows
// *which* providers exist and how to reach them; auth knows *how to
// authenticate* to them — API keys stored by /login, OAuth tokens with refresh,
// and the fallback chain that picks a credential at call time.
//
// The store is a single JSON file (~/.config/nib/credentials.json), written
// atomically with 0o600 perms under a 0o700 directory — the same discipline
// as chat/sessionstore.go. No SQLite, no external deps.
package auth

import (
	"fmt"
	"time"
)

// CredentialKind classifies how a credential authenticates.
type CredentialKind string

const (
	// CredentialAPIKey is a plain API key entered via /login or stored from
	// config. The key is sent as-is in the Authorization header (OpenAI) or
	// x-api-key header (Anthropic).
	CredentialAPIKey CredentialKind = "api-key"
	// CredentialOAuth is an OAuth access/refresh token pair obtained via the
	// authorization-code flow with PKCE. The access token is sent as a Bearer
	// token; when it expires, the refresh token is used to get a new one.
	CredentialOAuth CredentialKind = "oauth"
)

// Credential is one stored login for one provider. The store keeps at most one
// credential per provider (single-account); multi-account is a Phase 3 goal.
// Fields not relevant to the Kind are left zero-valued and omitted from JSON.
type Credential struct {
	ProviderID string         `json:"provider_id"`
	Kind       CredentialKind `json:"kind"`

	// CredentialAPIKey fields.
	APIKey string `json:"api_key,omitempty"`

	// CredentialOAuth fields.
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Email        string    `json:"email,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	OrgID        string    `json:"org_id,omitempty"`
	OrgName      string    `json:"org_name,omitempty"`
	// ProjectID is used by Cloud Code Assist (google-gemini-cli). Discovered
	// via loadCodeAssist/onboardUser after the OAuth exchange.
	ProjectID    string    `json:"project_id,omitempty"`
	AuthorizedAt time.Time `json:"authorized_at,omitempty"`
}

// refreshSkew is how far before ExpiresAt a token is considered expirable.
// Matching omp's 300-second skew: refresh early so a slow network round-trip
// does not race the real expiry.
const refreshSkew = 5 * time.Minute

// NeedsRefresh reports whether an OAuth credential's access token is expired
// or about to expire. Always false for non-OAuth credentials.
func (c Credential) NeedsRefresh() bool {
	if c.Kind != CredentialOAuth || c.RefreshToken == "" {
		return false
	}
	if c.ExpiresAt.IsZero() {
		return true // no expiry recorded — assume stale
	}
	return time.Now().Add(refreshSkew).After(c.ExpiresAt)
}

// IsExpired reports whether the access token has fully expired (no skew).
// Used to decide whether to attempt a refresh vs. demand re-login.
func (c Credential) IsExpired() bool {
	if c.Kind != CredentialOAuth {
		return false
	}
	if c.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().After(c.ExpiresAt)
}

// DisplayLabel returns a human-readable label for the account picker
// ("user@example.com" for OAuth, "API key" for api-key creds).
func (c Credential) DisplayLabel() string {
	switch c.Kind {
	case CredentialOAuth:
		if c.Email != "" {
			return c.Email
		}
		if c.AccountID != "" {
			return c.AccountID
		}
		return "OAuth account"
	case CredentialAPIKey:
		if c.APIKey == "" {
			return "empty key"
		}
		// Show the last 4 chars only — enough to tell keys apart, not enough
		// to reconstruct. Matches how omp displays stored keys.
		if len(c.APIKey) <= 4 {
			return "API key"
		}
		return fmt.Sprintf("API key …%s", c.APIKey[len(c.APIKey)-4:])
	default:
		return "unknown"
	}
}
