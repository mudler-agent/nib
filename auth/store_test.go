package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStorePath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "credentials.json")
}

func TestStoreSaveGetRoundtrip(t *testing.T) {
	s := NewStore(testStorePath(t))
	cred := Credential{
		ProviderID: "openai",
		Kind:       CredentialAPIKey,
		APIKey:     "sk-test-123",
	}
	if err := s.Save(cred); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("credential not found after save")
	}
	if got.APIKey != "sk-test-123" {
		t.Errorf("APIKey = %q, want sk-test-123", got.APIKey)
	}
}

func TestStoreSaveOverwrites(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "openai", Kind: CredentialAPIKey, APIKey: "old"})
	_ = s.Save(Credential{ProviderID: "openai", Kind: CredentialAPIKey, APIKey: "new"})
	got, ok, _ := s.Get("openai")
	if !ok {
		t.Fatal("credential not found")
	}
	if got.APIKey != "new" {
		t.Errorf("APIKey = %q, want 'new'", got.APIKey)
	}
}

func TestStoreSaveSetsAuthorizedAt(t *testing.T) {
	s := NewStore(testStorePath(t))
	before := time.Now()
	_ = s.Save(Credential{ProviderID: "anthropic", Kind: CredentialOAuth, AccessToken: "tok"})
	after := time.Now()
	got, _, _ := s.Get("anthropic")
	if got.AuthorizedAt.Before(before) || got.AuthorizedAt.After(after) {
		t.Errorf("AuthorizedAt = %v, want between %v and %v", got.AuthorizedAt, before, after)
	}
}

func TestStoreSavePreservesAuthorizedAt(t *testing.T) {
	s := NewStore(testStorePath(t))
	authAt := time.Now().Add(-1 * time.Hour)
	_ = s.Save(Credential{
		ProviderID:   "anthropic",
		Kind:         CredentialOAuth,
		AccessToken:  "tok",
		AuthorizedAt: authAt,
	})
	got, _, _ := s.Get("anthropic")
	if !got.AuthorizedAt.Equal(authAt) {
		t.Errorf("AuthorizedAt = %v, want %v", got.AuthorizedAt, authAt)
	}
}

func TestStoreSaveRejectsEmptyProviderID(t *testing.T) {
	s := NewStore(testStorePath(t))
	err := s.Save(Credential{ProviderID: "", Kind: CredentialAPIKey, APIKey: "k"})
	if err == nil {
		t.Fatal("expected error for empty ProviderID")
	}
}

func TestStoreGetMissing(t *testing.T) {
	s := NewStore(testStorePath(t))
	_, ok, err := s.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing credential")
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "openai", Kind: CredentialAPIKey, APIKey: "k"})
	if err := s.Delete("openai"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, ok, _ := s.Get("openai")
	if ok {
		t.Fatal("credential still exists after delete")
	}
}

func TestStoreDeleteMissing(t *testing.T) {
	s := NewStore(testStorePath(t))
	if err := s.Delete("nonexistent"); err != nil {
		t.Errorf("Delete on missing credential should not error: %v", err)
	}
}

func TestStoreAllSorted(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "zebra", Kind: CredentialAPIKey, APIKey: "z"})
	_ = s.Save(Credential{ProviderID: "alpha", Kind: CredentialAPIKey, APIKey: "a"})
	_ = s.Save(Credential{ProviderID: "mid", Kind: CredentialAPIKey, APIKey: "m"})

	all, err := s.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len(all) = %d, want 3", len(all))
	}
	if all[0].ProviderID != "alpha" || all[1].ProviderID != "mid" || all[2].ProviderID != "zebra" {
		t.Errorf("not sorted: %s, %s, %s", all[0].ProviderID, all[1].ProviderID, all[2].ProviderID)
	}
}

func TestStoreAllEmpty(t *testing.T) {
	s := NewStore(testStorePath(t))
	all, err := s.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("len(all) = %d, want 0", len(all))
	}
}

func TestStoreMissingFile(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nonexistent", "credentials.json"))
	_, ok, err := s.Get("any")
	if err != nil {
		t.Errorf("Get on missing file should not error: %v", err)
	}
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreFilePermissions(t *testing.T) {
	p := testStorePath(t)
	s := NewStore(p)
	_ = s.Save(Credential{ProviderID: "openai", Kind: CredentialAPIKey, APIKey: "k"})

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0o600", info.Mode().Perm())
	}
}

func TestStoreDirPermissions(t *testing.T) {
	Dir := filepath.Join(t.TempDir(), "nested", "auth")
	p := filepath.Join(Dir, "credentials.json")
	s := NewStore(p)
	_ = s.Save(Credential{ProviderID: "openai", Kind: CredentialAPIKey, APIKey: "k"})

	info, err := os.Stat(Dir)
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 0o700", info.Mode().Perm())
	}
}

func TestStoreMultipleProviders(t *testing.T) {
	s := NewStore(testStorePath(t))
	_ = s.Save(Credential{ProviderID: "openai", Kind: CredentialAPIKey, APIKey: "sk-o"})
	_ = s.Save(Credential{ProviderID: "anthropic", Kind: CredentialAPIKey, APIKey: "sk-a"})
	_ = s.Save(Credential{ProviderID: "google", Kind: CredentialAPIKey, APIKey: "sk-g"})

	got, ok, _ := s.Get("anthropic")
	if !ok {
		t.Fatal("anthropic not found")
	}
	if got.APIKey != "sk-a" {
		t.Errorf("anthropic APIKey = %q, want sk-a", got.APIKey)
	}

	all, _ := s.All()
	if len(all) != 3 {
		t.Errorf("len(all) = %d, want 3", len(all))
	}
}
