package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateVerifier(t *testing.T) {
	v, err := GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}
	if v == "" {
		t.Fatal("verifier is empty")
	}
	// 96 random bytes → 128 base64url chars (no padding)
	if len(v) != 128 {
		t.Errorf("verifier length = %d, want 128", len(v))
	}
	// must be base64url decodable
	dec, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		t.Fatalf("verifier not valid base64url: %v", err)
	}
	if len(dec) != verifierLen {
		t.Errorf("decoded verifier = %d bytes, want %d", len(dec), verifierLen)
	}
}

func TestGenerateVerifierUniqueness(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		v, err := GenerateVerifier()
		if err != nil {
			t.Fatalf("GenerateVerifier: %v", err)
	}
		if seen[v] {
			t.Fatal("duplicate verifier generated")
	}
		seen[v] = true
	}
}

func TestGenerateChallenge(t *testing.T) {
	verifier := "test-verifier-12345"
	challenge := GenerateChallenge(verifier)

	// S256 = BASE64URL(SHA256(verifier))
	h := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(h[:])
	if challenge != want {
		t.Errorf("challenge = %q, want %q", challenge, want)
	}
}

func TestGenerateChallengeDeterministic(t *testing.T) {
	v := "deterministic-verifier"
	c1 := GenerateChallenge(v)
	c2 := GenerateChallenge(v)
	if c1 != c2 {
		t.Fatal("challenge not deterministic for same verifier")
	}
}

func TestGenerateChallengeFromRandomVerifier(t *testing.T) {
	v, err := GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}
	c := GenerateChallenge(v)
	if c == "" {
		t.Fatal("challenge is empty")
	}
	// challenge is 43 chars (SHA256 = 32 bytes → 43 base64url chars)
	if len(c) != 43 {
		t.Errorf("challenge length = %d, want 43", len(c))
	}
}

func TestGenerateState(t *testing.T) {
	s, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	if s == "" {
		t.Fatal("state is empty")
	}
	// 16 random bytes → 32 hex chars
	if len(s) != 32 {
		t.Errorf("state length = %d, want 32", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Errorf("state not valid hex: %v", err)
	}
}

func TestGenerateStateUniqueness(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		s, err := GenerateState()
		if err != nil {
			t.Fatalf("GenerateState: %v", err)
	}
		if seen[s] {
			t.Fatal("duplicate state generated")
	}
		seen[s] = true
	}
}

func TestChallengeMethodConstant(t *testing.T) {
	if ChallengeMethod != "S256" {
		t.Errorf("ChallengeMethod = %q, want S256", ChallengeMethod)
	}
}

func TestGenerateVerifierNoPadding(t *testing.T) {
	v, err := GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}
	if strings.Contains(v, "=") {
		t.Error("verifier contains padding character")
	}
}
