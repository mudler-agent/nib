package auth

import (
	"strings"
	"testing"
	"time"
)

func TestNeedsRefreshNonOAuth(t *testing.T) {
	c := Credential{Kind: CredentialAPIKey}
	if c.NeedsRefresh() {
		t.Error("API key credential should not need refresh")
	}
}

func TestNeedsRefreshOAuthNoRefreshToken(t *testing.T) {
	c := Credential{Kind: CredentialOAuth, AccessToken: "tok"}
	if c.NeedsRefresh() {
		t.Error("OAuth without refresh token should not need refresh")
	}
}

func TestNeedsRefreshZeroExpiresAt(t *testing.T) {
	c := Credential{Kind: CredentialOAuth, RefreshToken: "ref", ExpiresAt: time.Time{}}
	if !c.NeedsRefresh() {
		t.Error("OAuth with zero ExpiresAt should need refresh")
	}
}

func TestNeedsRefreshWithinSkew(t *testing.T) {
	// Token expires in 3 minutes — within the 5-minute skew.
	c := Credential{
		Kind:         CredentialOAuth,
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(3 * time.Minute),
	}
	if !c.NeedsRefresh() {
		t.Error("token within skew window should need refresh")
	}
}

func TestNeedsRefreshWellBeforeExpiry(t *testing.T) {
	c := Credential{
		Kind:         CredentialOAuth,
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}
	if c.NeedsRefresh() {
		t.Error("token well before expiry should not need refresh")
	}
}

func TestIsExpiredNonOAuth(t *testing.T) {
	c := Credential{Kind: CredentialAPIKey}
	if c.IsExpired() {
		t.Error("API key should never be expired")
	}
}

func TestIsExpiredZeroExpiresAt(t *testing.T) {
	c := Credential{Kind: CredentialOAuth, ExpiresAt: time.Time{}}
	if !c.IsExpired() {
		t.Error("OAuth with zero ExpiresAt should be expired")
	}
}

func TestIsExpiredBeforeExpiry(t *testing.T) {
	c := Credential{
		Kind:      CredentialOAuth,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if c.IsExpired() {
		t.Error("token before expiry should not be expired")
	}
}

func TestIsExpiredAfterExpiry(t *testing.T) {
	c := Credential{
		Kind:      CredentialOAuth,
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	if !c.IsExpired() {
		t.Error("token after expiry should be expired")
	}
}

func TestDisplayLabelOAuthWithEmail(t *testing.T) {
	c := Credential{Kind: CredentialOAuth, Email: "user@example.com"}
	if got := c.DisplayLabel(); got != "user@example.com" {
		t.Errorf("DisplayLabel = %q, want user@example.com", got)
	}
}

func TestDisplayLabelOAuthWithAccountID(t *testing.T) {
	c := Credential{Kind: CredentialOAuth, AccountID: "acc-123"}
	if got := c.DisplayLabel(); got != "acc-123" {
		t.Errorf("DisplayLabel = %q, want acc-123", got)
	}
}

func TestDisplayLabelOAuthNoEmailOrAccountID(t *testing.T) {
	c := Credential{Kind: CredentialOAuth}
	if got := c.DisplayLabel(); got != "OAuth account" {
		t.Errorf("DisplayLabel = %q, want 'OAuth account'", got)
	}
}

func TestDisplayLabelAPIKey(t *testing.T) {
	c := Credential{Kind: CredentialAPIKey, APIKey: "sk-abcdefghijklmnop"}
	got := c.DisplayLabel()
	if !strings.Contains(got, "mnop") {
		t.Errorf("DisplayLabel = %q, should end with last 4 chars", got)
	}
	if !strings.HasPrefix(got, "API key") {
		t.Errorf("DisplayLabel = %q, should start with 'API key'", got)
	}
}

func TestDisplayLabelShortAPIKey(t *testing.T) {
	c := Credential{Kind: CredentialAPIKey, APIKey: "ab"}
	if got := c.DisplayLabel(); got != "API key" {
		t.Errorf("DisplayLabel = %q, want 'API key' for short key", got)
	}
}

func TestDisplayLabelEmptyAPIKey(t *testing.T) {
	c := Credential{Kind: CredentialAPIKey, APIKey: ""}
	if got := c.DisplayLabel(); got != "empty key" {
		t.Errorf("DisplayLabel = %q, want 'empty key'", got)
	}
}

func TestDisplayLabelExactlyFourChars(t *testing.T) {
	c := Credential{Kind: CredentialAPIKey, APIKey: "abcd"}
	if got := c.DisplayLabel(); got != "API key" {
		t.Errorf("DisplayLabel = %q, want 'API key' for 4-char key", got)
	}
}

func TestDisplayLabelUnknownKind(t *testing.T) {
	c := Credential{Kind: "unknown"}
	if got := c.DisplayLabel(); got != "unknown" {
		t.Errorf("DisplayLabel = %q, want 'unknown'", got)
	}
}
