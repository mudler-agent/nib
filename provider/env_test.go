package provider

import "testing"

func TestEffectiveClientIDNoEnv(t *testing.T) {
	d := Definition{ClientID: "hardcoded-id"}
	if got := d.EffectiveClientID(); got != "hardcoded-id" {
		t.Errorf("EffectiveClientID = %q, want 'hardcoded-id'", got)
	}
}

func TestEffectiveClientIDFromEnv(t *testing.T) {
	t.Setenv("TEST_OAUTH_CLIENT_ID", "env-client-id")
	d := Definition{
		ClientID:    "hardcoded-id",
		EnvClientID: "TEST_OAUTH_CLIENT_ID",
	}
	if got := d.EffectiveClientID(); got != "env-client-id" {
		t.Errorf("EffectiveClientID = %q, want 'env-client-id'", got)
	}
}

func TestEffectiveClientIDEnvEmptyFallsBack(t *testing.T) {
	t.Setenv("TEST_OAUTH_CLIENT_ID_EMPTY", "")
	d := Definition{
		ClientID:    "fallback-id",
		EnvClientID: "TEST_OAUTH_CLIENT_ID_EMPTY",
	}
	if got := d.EffectiveClientID(); got != "fallback-id" {
		t.Errorf("EffectiveClientID = %q, want 'fallback-id'", got)
	}
}

func TestEffectiveClientIDNoEnvVarName(t *testing.T) {
	d := Definition{ClientID: "direct-id"}
	if got := d.EffectiveClientID(); got != "direct-id" {
		t.Errorf("EffectiveClientID = %q, want 'direct-id'", got)
	}
}

func TestEffectiveClientSecretNoEnv(t *testing.T) {
	d := Definition{ClientSecret: "hardcoded-secret"}
	if got := d.EffectiveClientSecret(); got != "hardcoded-secret" {
		t.Errorf("EffectiveClientSecret = %q, want 'hardcoded-secret'", got)
	}
}

func TestEffectiveClientSecretFromEnv(t *testing.T) {
	t.Setenv("TEST_OAUTH_CLIENT_SECRET", "env-secret")
	d := Definition{
		ClientSecret:    "hardcoded-secret",
		EnvClientSecret: "TEST_OAUTH_CLIENT_SECRET",
	}
	if got := d.EffectiveClientSecret(); got != "env-secret" {
		t.Errorf("EffectiveClientSecret = %q, want 'env-secret'", got)
	}
}

func TestEffectiveClientSecretEnvEmptyFallsBack(t *testing.T) {
	t.Setenv("TEST_OAUTH_CLIENT_SECRET_EMPTY", "")
	d := Definition{
		ClientSecret:    "fallback-secret",
		EnvClientSecret: "TEST_OAUTH_CLIENT_SECRET_EMPTY",
	}
	if got := d.EffectiveClientSecret(); got != "fallback-secret" {
		t.Errorf("EffectiveClientSecret = %q, want 'fallback-secret'", got)
	}
}

func TestEffectiveClientSecretNoEnvVarName(t *testing.T) {
	d := Definition{ClientSecret: "direct-secret"}
	if got := d.EffectiveClientSecret(); got != "direct-secret" {
		t.Errorf("EffectiveClientSecret = %q, want 'direct-secret'", got)
	}
}

func TestEffectiveClientIDEmptyBoth(t *testing.T) {
	d := Definition{}
	if got := d.EffectiveClientID(); got != "" {
		t.Errorf("EffectiveClientID = %q, want empty", got)
	}
}

func TestEffectiveClientSecretEmptyBoth(t *testing.T) {
	d := Definition{}
	if got := d.EffectiveClientSecret(); got != "" {
		t.Errorf("EffectiveClientSecret = %q, want empty", got)
	}
}
