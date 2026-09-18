package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/mudler/nib/auth/oauth"
	"github.com/mudler/nib/provider"
)

// LoginAPIKey creates and stores an API-key credential for a provider.
// Returns the credential (with AuthorizedAt set).
func LoginAPIKey(store *Store, def provider.Definition, apiKey string) (Credential, error) {
	if apiKey == "" {
		return Credential{}, fmt.Errorf("auth: empty API key for %s", def.ID)
	}
	cred := Credential{
		ProviderID: def.ID,
		Kind:       CredentialAPIKey,
		APIKey:     apiKey,
	}
	if err := store.Save(cred); err != nil {
		return Credential{}, fmt.Errorf("auth: save %s credential: %w", def.ID, err)
	}
	return cred, nil
}

// OAuthFlow holds the state of an in-progress OAuth authorization-code flow:
// the PKCE verifier, the CSRF state, and the running callback server. Start
// with StartOAuthFlow, display AuthorizeURL to the user, then call Complete
// to wait for the callback and exchange the code.
type OAuthFlow struct {
	def         provider.Definition
	verifier    string
	state       string
	server      *oauth.CallbackServer
	redirectURI string
}

// StartOAuthFlow generates PKCE parameters, starts the loopback callback
// server, and returns a flow ready to display the authorize URL.
func StartOAuthFlow(def provider.Definition) (*OAuthFlow, error) {
	verifier, err := oauth.GenerateVerifier()
	if err != nil {
		return nil, err
	}
	state, err := oauth.GenerateState()
	if err != nil {
		return nil, err
	}
	server := oauth.NewCallbackServer(def.CallbackPort, def.CallbackPath, def.CallbackHost, state)
	redirectURI, err := server.Start()
	if err != nil {
		return nil, fmt.Errorf("auth: start callback server: %w", err)
	}
	return &OAuthFlow{
		def:         def,
		verifier:    verifier,
		state:       state,
		server:      server,
		redirectURI: redirectURI,
	}, nil
}

// AuthorizeURL returns the URL the user must open to authorize the app.
func (f *OAuthFlow) AuthorizeURL() string {
	challenge := oauth.GenerateChallenge(f.verifier)
	return oauth.AuthorizeURL(f.def, challenge, f.state, f.redirectURI)
}

// Complete waits for the OAuth callback, exchanges the code for tokens,
// bootstraps identity, and returns a credential ready to save. The caller
// is responsible for persisting it via Store.Save.
func (f *OAuthFlow) Complete(ctx context.Context) (Credential, error) {
	result := f.server.Wait(ctx)
	if result.Err != nil {
		return Credential{}, fmt.Errorf("auth: oauth callback: %w", result.Err)
	}

	tr, err := oauth.Exchange(ctx, f.def, result.Code, f.verifier, f.redirectURI, result.State)
	if err != nil {
		return Credential{}, fmt.Errorf("auth: exchange code: %w", err)
	}

	// Identity recovery is best-effort: the token is valid even if we
	// cannot recover the account/org info. omp does the same.
	cred := Credential{
		ProviderID:   f.def.ID,
		Kind:         CredentialOAuth,
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    oauth.ExpiresAt(tr.ExpiresIn),
	}
	if id, err := oauth.FetchIdentity(ctx, f.def, tr.AccessToken); err == nil {
		cred.AccountID = id.AccountID
		cred.Email = id.Email
		cred.OrgID = id.OrgID
		cred.OrgName = id.OrgName
	}

	// Post-exchange hook (e.g., Cloud Code Assist project discovery).
	if hook := GetPostExchange(f.def.ID); hook != nil {
		var err error
		cred, err = hook(ctx, cred, f.def)
		if err != nil {
			return Credential{}, fmt.Errorf("auth: post-exchange hook for %s: %w", f.def.ID, err)
		}
	}

	return cred, nil
}

// LoginOAuth runs the full OAuth flow (start + complete) and saves the
// credential. onURL is called with the authorize URL before waiting for the
// callback, so the caller can display it or open a browser.
func LoginOAuth(ctx context.Context, store *Store, def provider.Definition, onURL func(string)) (Credential, error) {
	flow, err := StartOAuthFlow(def)
	if err != nil {
		return Credential{}, err
	}
	onURL(flow.AuthorizeURL())
	cred, err := flow.Complete(ctx)
	if err != nil {
		return Credential{}, err
	}
	if err := store.Save(cred); err != nil {
		return Credential{}, fmt.Errorf("auth: save %s credential: %w", def.ID, err)
	}
	return cred, nil
}

// LoginDeviceCode runs the RFC 8628 device-code flow and saves the credential.
// onInstructions is called with a human-readable instruction string (containing
// the user code and verification URL) before polling begins, so the caller can
// display it and optionally open the verification URL in a browser.
func LoginDeviceCode(ctx context.Context, store *Store, def provider.Definition, onInstructions func(string, string)) (Credential, error) {
	dr, err := oauth.RequestDeviceCode(ctx, def)
	if err != nil {
		return Credential{}, fmt.Errorf("auth: device code: %w", err)
	}

	// Build the instruction string and the URL to open.
	verificationURL := dr.VerificationURIComplete
	if verificationURL == "" {
		verificationURL = dr.VerificationURI
	}
	instructions := fmt.Sprintf("Go to %s and enter code: %s", dr.VerificationURI, dr.UserCode)
	onInstructions(instructions, verificationURL)

	tr, err := oauth.PollDeviceToken(ctx, def, dr)
	if err != nil {
		return Credential{}, fmt.Errorf("auth: device flow: %w", err)
	}

	cred := Credential{
		ProviderID:   def.ID,
		Kind:         CredentialOAuth,
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    oauth.ExpiresAt(tr.ExpiresIn),
	}
	if id, err := oauth.FetchIdentity(ctx, def, tr.AccessToken); err == nil {
		cred.AccountID = id.AccountID
		cred.Email = id.Email
		cred.OrgID = id.OrgID
		cred.OrgName = id.OrgName
	}

	// Post-exchange hook (e.g., Cloud Code Assist project discovery).
	if hook := GetPostExchange(def.ID); hook != nil {
		var err error
		cred, err = hook(ctx, cred, def)
		if err != nil {
			return Credential{}, fmt.Errorf("auth: post-exchange hook for %s: %w", def.ID, err)
		}
	}

	if err := store.Save(cred); err != nil {
		return Credential{}, fmt.Errorf("auth: save %s credential: %w", def.ID, err)
	}
	return cred, nil
}

// LoginFlow represents an in-progress login flow. The caller displays Prompt
// to the user (and optionally opens URL in a browser), then calls Complete to
// finish the flow asynchronously.
type LoginFlow struct {
	ProviderID string
	Prompt     string // authorize URL or device-code instructions
	URL        string // URL to open in a browser (may differ from Prompt)
	complete   func(context.Context) (Credential, error)
}

// Complete finishes the login flow, returning the saved credential. It blocks
// until the OAuth callback arrives, the device-code poll succeeds, or the
// context is cancelled.
func (f *LoginFlow) Complete(ctx context.Context) (Credential, error) {
	return f.complete(ctx)
}

// NewLoginFlow constructs a LoginFlow with the given display fields and
// completion function. It is intended for providers that finish the flow
// synchronously (e.g. Copilot token import) and need a no-op Complete.
func NewLoginFlow(providerID, prompt, url string, complete func(context.Context) (Credential, error)) *LoginFlow {
	return &LoginFlow{
		ProviderID: providerID,
		Prompt:     prompt,
		URL:        url,
		complete:   complete,
	}
}

// StartLogin begins an OAuth-code or device-code login flow for def. The
// returned LoginFlow's Prompt should be displayed to the user and URL opened
// in a browser; then Complete should be called to finish the flow.
// For LoginOAuthCode and LoginDeviceCode only. For other login kinds, use
// LoginAPIKey or the provider-specific importer.
func StartLogin(ctx context.Context, store *Store, def provider.Definition) (*LoginFlow, error) {
	switch def.LoginKind {
	case provider.LoginOAuthCode:
		flow, err := StartOAuthFlow(def)
		if err != nil {
			return nil, err
		}
		url := flow.AuthorizeURL()
		return &LoginFlow{
			ProviderID: def.ID,
			Prompt:     "Open this URL to log in:\n" + url,
			URL:        url,
			complete: func(ctx context.Context) (Credential, error) {
				cred, err := flow.Complete(ctx)
				if err != nil {
					return Credential{}, err
				}
				if err := store.Save(cred); err != nil {
					return Credential{}, fmt.Errorf("auth: save %s credential: %w", def.ID, err)
				}
				return cred, nil
			},
		}, nil

	case provider.LoginDeviceCode:
		dr, err := oauth.RequestDeviceCode(ctx, def)
		if err != nil {
			return nil, fmt.Errorf("auth: device code: %w", err)
		}
		verificationURL := dr.VerificationURIComplete
		if verificationURL == "" {
			verificationURL = dr.VerificationURI
		}
		instructions := fmt.Sprintf("Go to %s and enter code: %s", dr.VerificationURI, dr.UserCode)
		return &LoginFlow{
			ProviderID: def.ID,
			Prompt:     instructions,
			URL:        verificationURL,
			complete: func(ctx context.Context) (Credential, error) {
				tr, err := oauth.PollDeviceToken(ctx, def, dr)
				if err != nil {
					return Credential{}, fmt.Errorf("auth: device flow: %w", err)
				}
				cred := Credential{
					ProviderID:   def.ID,
					Kind:         CredentialOAuth,
					AccessToken:  tr.AccessToken,
					RefreshToken: tr.RefreshToken,
					ExpiresAt:    oauth.ExpiresAt(tr.ExpiresIn),
				}
				if id, err := oauth.FetchIdentity(ctx, def, tr.AccessToken); err == nil {
					cred.AccountID = id.AccountID
					cred.Email = id.Email
					cred.OrgID = id.OrgID
					cred.OrgName = id.OrgName
				}
				if hook := GetPostExchange(def.ID); hook != nil {
					var err error
					cred, err = hook(ctx, cred, def)
					if err != nil {
						return Credential{}, fmt.Errorf("auth: post-exchange hook for %s: %w", def.ID, err)
					}
				}
				if err := store.Save(cred); err != nil {
					return Credential{}, fmt.Errorf("auth: save %s credential: %w", def.ID, err)
				}
				return cred, nil
			},
		}, nil

	default:
		return nil, fmt.Errorf("auth: StartLogin does not support login kind %q for %s", def.LoginKind, def.ID)
	}
}

// StatusLine returns a one-line summary of a stored credential for display
// in `nib login --list`. Example: "user@example.com (OAuth, expires in 3d)".
func (c Credential) StatusLine() string {
	switch c.Kind {
	case CredentialOAuth:
		label := c.Email
		if label == "" {
			label = c.AccountID
		}
		if label == "" {
			label = "OAuth account"
		}
		if c.ExpiresAt.IsZero() {
			return fmt.Sprintf("%s (OAuth)", label)
		}
		remaining := time.Until(c.ExpiresAt)
		if remaining < 0 {
			return fmt.Sprintf("%s (OAuth, expired)", label)
		}
		return fmt.Sprintf("%s (OAuth, expires in %s)", label, roundDuration(remaining))
	case CredentialAPIKey:
		return c.DisplayLabel()
	default:
		return "unknown"
	}
}

func roundDuration(d time.Duration) string {
	if d > 24*time.Hour {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	if d > time.Hour {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d > time.Minute {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return fmt.Sprintf("%ds", int(d/time.Second))
}
