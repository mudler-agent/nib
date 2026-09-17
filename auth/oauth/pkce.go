// Package oauth implements the OAuth authorization-code flow with PKCE for
// remote providers that support it (currently Anthropic). It provides the
// PKCE primitives, the loopback callback server, and the per-provider flow
// driver (authorize URL, token exchange, refresh, identity bootstrap).
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// verifierLen is the number of random bytes in the PKCE code verifier.
// Anthropic/Claude Code uses 96 bytes (128 base64url chars), well above the
// RFC 7636 minimum of 32. The spec caps the verifier at 128 bytes.
const verifierLen = 96

// GenerateVerifier returns a high-entropy PKCE code verifier: verifierLen
// random bytes, base64url-encoded (no padding). The verifier is sent in the
// token exchange; the challenge derived from it is sent in the authorize
// request.
func GenerateVerifier() (string, error) {
	b := make([]byte, verifierLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateChallenge returns the S256 PKCE code challenge for verifier:
// BASE64URL(SHA256(verifier)). S256 is the only method Anthropic accepts.
func GenerateChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateState returns a random hex-encoded CSRF state parameter. The state
// ties the callback to the authorize request that started the flow; the
// callback server rejects any callback whose state does not match.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ChallengeMethod is the PKCE challenge method, always "S256".
const ChallengeMethod = "S256"
