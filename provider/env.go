package provider

import "os"

// EffectiveClientID returns the OAuth client ID, resolving from the
// environment variable named by EnvClientID when set. This keeps OAuth
// client credentials out of source-controlled code.
func (d Definition) EffectiveClientID() string {
	if d.EnvClientID != "" {
		if v := os.Getenv(d.EnvClientID); v != "" {
			return v
		}
	}
	return d.ClientID
}

// EffectiveClientSecret returns the OAuth client secret, resolving from
// the environment variable named by EnvClientSecret when set.
func (d Definition) EffectiveClientSecret() string {
	if d.EnvClientSecret != "" {
		if v := os.Getenv(d.EnvClientSecret); v != "" {
			return v
		}
	}
	return d.ClientSecret
}
