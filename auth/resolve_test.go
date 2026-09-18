package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mudler/nib/provider"
)

func testDef(id string) provider.Definition {
	return provider.Definition{
		ID:     id,
		EnvVar: "TEST_KEY_" + id,
	}
}

func TestResolveStoredAPIKey(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "test", Kind: CredentialAPIKey, APIKey: "stored-key"})

	def := testDef("test")
	r, err := Resolve(s, def, "config-key")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "stored-key" {
		t.Errorf("APIKey = %q, want 'stored-key'", r.APIKey)
	}
	if r.IsOAuth {
		t.Error("IsOAuth should be false for API key")
	}
}

func TestResolveStoredOAuthValid(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{
		ProviderID:   "test",
		Kind:         CredentialOAuth,
		AccessToken:  "valid-tok",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	})

	def := testDef("test")
	r, err := Resolve(s, def, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "valid-tok" {
		t.Errorf("APIKey = %q, want 'valid-tok'", r.APIKey)
	}
	if !r.IsOAuth {
		t.Error("IsOAuth should be true")
	}
}

func TestResolveStoredOAuthRefreshSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-tok",
			"refresh_token": "new-ref",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer server.Close()

	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{
		ProviderID:   "test",
		Kind:         CredentialOAuth,
		AccessToken:  "old-tok",
		RefreshToken: "old-ref",
		ExpiresAt:    time.Now().Add(-1 * time.Minute), // expired
	})

	def := testDef("test")
	def.TokenURL = server.URL + "/token"
	def.ClientID = "test-client"

	r, err := Resolve(s, def, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "refreshed-tok" {
		t.Errorf("APIKey = %q, want 'refreshed-tok'", r.APIKey)
	}
	if !r.IsOAuth {
		t.Error("IsOAuth should be true")
	}

	// Verify refreshed credential was persisted.
	got, _, _ := s.Get("test")
	if got.AccessToken != "refreshed-tok" {
		t.Errorf("stored AccessToken = %q, want 'refreshed-tok'", got.AccessToken)
	}
	if got.RefreshToken != "new-ref" {
		t.Errorf("stored RefreshToken = %q, want 'new-ref'", got.RefreshToken)
	}
}

func TestResolveStoredOAuthRefreshFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
	}))
	defer server.Close()

	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{
		ProviderID:   "test",
		Kind:         CredentialOAuth,
		AccessToken:  "old-tok",
		RefreshToken: "old-ref",
		ExpiresAt:    time.Now().Add(-1 * time.Minute), // expired
	})

	def := testDef("test")
	def.TokenURL = server.URL + "/token"
	def.ClientID = "test-client"

	r, err := Resolve(s, def, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// On refresh failure, resolveOAuth returns empty (access token cleared).
	if r.APIKey != "" {
		t.Errorf("APIKey = %q, want empty on refresh failure", r.APIKey)
	}
}

func TestResolveConfigAPIKey(t *testing.T) {
	s := NewStore(testStorePath(t)) // empty store
	def := testDef("test")
	r, err := Resolve(s, def, "config-api-key")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "config-api-key" {
		t.Errorf("APIKey = %q, want 'config-api-key'", r.APIKey)
	}
}

func TestResolveEnvVar(t *testing.T) {
	s := NewStore(testStorePath(t)) // empty store
	def := testDef("TESTPROVIDER")
	t.Setenv("TEST_KEY_TESTPROVIDER", "env-key")

	r, err := Resolve(s, def, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want 'env-key'", r.APIKey)
	}
}

func TestResolveFallbackEmpty(t *testing.T) {
	s := NewStore(testStorePath(t)) // empty store
	def := testDef("TESTPROVIDER2") // no env var set
	r, err := Resolve(s, def, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", r.APIKey)
	}
}

func TestResolveStoreTakesPriorityOverConfig(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "test", Kind: CredentialAPIKey, APIKey: "stored"})

	def := testDef("test")
	r, _ := Resolve(s, def, "config")
	if r.APIKey != "stored" {
		t.Errorf("APIKey = %q, want 'stored' (store should win)", r.APIKey)
	}
}

func TestResolveNilStore(t *testing.T) {
	def := testDef("TESTPROVIDER3")
	r, err := Resolve(nil, def, "config-key")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.APIKey != "config-key" {
		t.Errorf("APIKey = %q, want 'config-key'", r.APIKey)
	}
}

func TestResolveOrErrorNoCredentials(t *testing.T) {
	s := NewStore(testStorePath(t)) // empty store
	def := testDef("TESTPROVIDER4") // no env var
	_, err := ResolveOrError(s, def, "")
	if err == nil {
		t.Fatal("expected error when no credentials found")
	}
}

func TestResolveOrErrorWithConfigKey(t *testing.T) {
	s := NewStore(testStorePath(t))
	def := testDef("test")
	r, err := ResolveOrError(s, def, "config-key")
	if err != nil {
		t.Fatalf("ResolveOrError: %v", err)
	}
	if r.APIKey != "config-key" {
		t.Errorf("APIKey = %q, want 'config-key'", r.APIKey)
	}
}

func TestResolveOrErrorWithStoredCred(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "test", Kind: CredentialAPIKey, APIKey: "stored"})
	def := testDef("test")
	r, err := ResolveOrError(s, def, "")
	if err != nil {
		t.Fatalf("ResolveOrError: %v", err)
	}
	if r.APIKey != "stored" {
		t.Errorf("APIKey = %q, want 'stored'", r.APIKey)
	}
}
