package types

import "testing"

func TestResolvedModelProvidersAreIndependent(t *testing.T) {
	cfg := Config{
		Provider: "codex", Model: "main-model",
		PromptInjectionProtection: PromptInjectionProtectionConfig{
			Enabled: true,
			Classifier: ModelProviderConfig{
				Provider: "openai", Model: "classifier-model",
				BaseURL: "http://classifier.invalid/v1", APIKey: "classifier-key",
			},
		},
	}
	main := cfg.ResolvedMainModel()
	classifier := cfg.ResolvedClassifierModel()
	if main.Provider != "codex" || main.Model != "main-model" {
		t.Fatalf("main = %#v", main)
	}
	if classifier.Provider != "openai" || classifier.Model != "classifier-model" ||
		classifier.BaseURL != "http://classifier.invalid/v1" || classifier.APIKey != "classifier-key" {
		t.Fatalf("classifier = %#v", classifier)
	}
}

func TestResolvedMainModelPreservesLegacyOpenAIConfig(t *testing.T) {
	cfg := Config{Model: "legacy", APIKey: "key", BaseURL: "http://legacy.invalid/v1"}
	main := cfg.ResolvedMainModel()
	if main.Provider != "openai" || main.Model != cfg.Model || main.APIKey != cfg.APIKey || main.BaseURL != cfg.BaseURL {
		t.Fatalf("main = %#v", main)
	}
	classifier := cfg.ResolvedClassifierModel()
	if classifier.Provider != main.Provider || classifier.Model != main.Model ||
		classifier.APIKey != main.APIKey || classifier.BaseURL != main.BaseURL {
		t.Fatalf("classifier = %#v, want inherited %#v", classifier, main)
	}
}

func TestClassifierPartialOverrideInheritsTopLevelEndpoint(t *testing.T) {
	cfg := Config{
		Model: "main", APIKey: "key", BaseURL: "https://api.example/v1",
		PromptInjectionProtection: PromptInjectionProtectionConfig{
			Classifier: ModelProviderConfig{Model: "classifier"},
		},
	}
	classifier := cfg.ResolvedClassifierModel()
	if classifier.Provider != "openai" || classifier.Model != "classifier" ||
		classifier.APIKey != "key" || classifier.BaseURL != "https://api.example/v1" {
		t.Fatalf("classifier = %#v", classifier)
	}
}

func TestResolvedClassifierSupportsCodexIndependentOfOpenAIMain(t *testing.T) {
	cfg := Config{
		Model: "remote-main", BaseURL: "https://api.example/v1",
		PromptInjectionProtection: PromptInjectionProtectionConfig{
			Classifier: ModelProviderConfig{Provider: "codex", Model: "classifier"},
		},
	}
	if got := cfg.ResolvedMainModel().Provider; got != "openai" {
		t.Fatalf("main provider = %q", got)
	}
	classifier := cfg.ResolvedClassifierModel()
	if classifier.Provider != "codex" || classifier.Model != "classifier" {
		t.Fatalf("classifier = %#v", classifier)
	}
}

func TestResolvedAPIKeyPrefersInlineKey(t *testing.T) {
	t.Setenv("NIB_TEST_KEY", "from-env")
	c := ModelProviderConfig{APIKey: "inline", APIKeyEnv: "NIB_TEST_KEY"}
	if got := c.ResolvedAPIKey(); got != "inline" {
		t.Fatalf("ResolvedAPIKey = %q, want \"inline\"", got)
	}
}

func TestResolvedAPIKeyFallsBackToEnv(t *testing.T) {
	t.Setenv("NIB_TEST_KEY", "from-env")
	c := ModelProviderConfig{APIKeyEnv: "NIB_TEST_KEY"}
	if got := c.ResolvedAPIKey(); got != "from-env" {
		t.Fatalf("ResolvedAPIKey = %q, want \"from-env\"", got)
	}
}

// Finding 6: ResolvedAPIKey() had exactly one production caller
// (endpoint/set.go), so api_key_env on the top-level block was accepted and
// silently ignored. ResolvedMainModel must resolve it too.
func TestResolvedMainModelResolvesAPIKeyEnv(t *testing.T) {
	t.Setenv("NIB_TEST_MAIN_KEY", "from-env-main")
	cfg := Config{Model: "m", APIKeyEnv: "NIB_TEST_MAIN_KEY"}
	main := cfg.ResolvedMainModel()
	if main.APIKey != "from-env-main" {
		t.Fatalf("main.APIKey = %q, want the env-resolved key", main.APIKey)
	}
	if main.APIKeyEnv != "" {
		t.Fatalf("main.APIKeyEnv = %q, want it cleared once resolved", main.APIKeyEnv)
	}
}

// The classifier's own api_key_env, the concrete bug the review reported
// (prompt_injection_protection.classifier.api_key_env silently ignored),
// must resolve too, and must win over whatever the inherited main key
// resolved to — not be shadowed by it.
func TestResolvedClassifierModelResolvesItsOwnAPIKeyEnv(t *testing.T) {
	t.Setenv("NIB_TEST_CLASSIFIER_KEY", "from-env-classifier")
	cfg := Config{
		Model: "main", APIKey: "main-key",
		PromptInjectionProtection: PromptInjectionProtectionConfig{
			Classifier: ModelProviderConfig{Model: "classifier", APIKeyEnv: "NIB_TEST_CLASSIFIER_KEY"},
		},
	}
	classifier := cfg.ResolvedClassifierModel()
	if classifier.APIKey != "from-env-classifier" {
		t.Fatalf("classifier.APIKey = %q, want the classifier's own env-resolved key, not the inherited main key", classifier.APIKey)
	}
	if classifier.APIKeyEnv != "" {
		t.Fatalf("classifier.APIKeyEnv = %q, want it cleared once resolved", classifier.APIKeyEnv)
	}
}

func TestResolvedAPIKeyEmptyWhenEnvUnset(t *testing.T) {
	c := ModelProviderConfig{APIKeyEnv: "NIB_TEST_KEY_ABSENT"}
	if got := c.ResolvedAPIKey(); got != "" {
		t.Fatalf("ResolvedAPIKey = %q, want empty", got)
	}
}
