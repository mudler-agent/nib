package types

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEndpointsKeepFileOrder(t *testing.T) {
	const src = `
endpoints:
  zeta:
    base_url: http://zeta/v1
  alpha:
    base_url: http://alpha/v1
  middle:
    base_url: http://middle/v1
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []string{"zeta", "alpha", "middle"}
	if len(cfg.Endpoints) != len(want) {
		t.Fatalf("got %d endpoints, want %d", len(cfg.Endpoints), len(want))
	}
	for i, name := range want {
		if cfg.Endpoints[i].Name != name {
			t.Fatalf("endpoint %d = %q, want %q", i, cfg.Endpoints[i].Name, name)
		}
	}
}

func TestEndpointFieldsInlineFromMapping(t *testing.T) {
	const src = `
endpoints:
  work-vllm:
    base_url: https://vllm.corp/v1
    api_key_env: VLLM_KEY
    model: llama-3.3-70b
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	e := cfg.Endpoints[0]
	if e.BaseURL != "https://vllm.corp/v1" || e.APIKeyEnv != "VLLM_KEY" || e.Model != "llama-3.3-70b" {
		t.Fatalf("endpoint = %#v", e)
	}
}

func TestValidateDropsBadEntriesAndKeepsGoodOnes(t *testing.T) {
	in := Endpoints{
		{Name: "good", ModelProviderConfig: ModelProviderConfig{BaseURL: "http://good/v1"}},
		{Name: "model only", ModelProviderConfig: ModelProviderConfig{Model: "m"}},
		{Name: "good", ModelProviderConfig: ModelProviderConfig{BaseURL: "http://dup/v1"}},
		{Name: "", ModelProviderConfig: ModelProviderConfig{BaseURL: "http://noname/v1"}},
		{Name: "@at", ModelProviderConfig: ModelProviderConfig{BaseURL: "http://at/v1"}},
		{Name: "byprovider", ModelProviderConfig: ModelProviderConfig{Provider: "anthropic"}},
	}
	out, errs := in.Validate()
	if len(out) != 2 || out[0].Name != "good" || out[1].Name != "byprovider" {
		t.Fatalf("kept = %#v", out)
	}
	if len(errs) != 4 {
		t.Fatalf("errs = %v, want 4", errs)
	}
	joined := ""
	for _, err := range errs {
		joined += err.Error() + "\n"
	}
	for _, want := range []string{"model only", "duplicate", "name", "@"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("errors %q missing %q", joined, want)
		}
	}
}
