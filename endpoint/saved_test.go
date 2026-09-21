package endpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mudler/nib/types"
)

func TestLoadSavedReadsTheLegacyProviderShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.json")
	if err := os.WriteFile(path, []byte(`{"provider":"regolo","model":"x"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	sv := LoadSaved(path)
	if sv.ID != "regolo" || sv.Model != "x" {
		t.Fatalf("Saved = %#v", sv)
	}
}

func TestWriteSavedThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "provider.json")
	if err := WriteSaved(path, Saved{ID: "@work", Model: "llama"}); err != nil {
		t.Fatalf("WriteSaved: %v", err)
	}
	if sv := LoadSaved(path); sv.ID != "@work" || sv.Model != "llama" {
		t.Fatalf("Saved = %#v", sv)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"id"`) {
		t.Fatalf("file = %s, want the new id shape", data)
	}
}

func TestLoadSavedMissingFile(t *testing.T) {
	if sv := LoadSaved(filepath.Join(t.TempDir(), "absent.json")); sv.ID != "" {
		t.Fatalf("Saved = %#v, want zero", sv)
	}
}

func startupConfig(t *testing.T) types.Config {
	t.Helper()
	return types.Config{
		Model: "qwen3-coder", BaseURL: "http://localhost:8080/v1",
		Endpoints: types.Endpoints{{
			Name:                "work",
			ModelProviderConfig: types.ModelProviderConfig{BaseURL: "https://vllm.corp/v1", Model: "llama"},
		}},
	}
}

func TestStartupHonorsTheSavedPick(t *testing.T) {
	e, cfg, note := testSet(t, startupConfig(t)).Startup(Saved{ID: "@work", Model: "llama-70b"})
	if e.ID != "@work" {
		t.Fatalf("entry = %#v", e)
	}
	if cfg.Model != "llama-70b" {
		t.Fatalf("Model = %q, want the saved model to win", cfg.Model)
	}
	if note != "" {
		t.Fatalf("note = %q, want none", note)
	}
}

func TestStartupSavedWithoutModelUsesTheEntryModel(t *testing.T) {
	_, cfg, _ := testSet(t, startupConfig(t)).Startup(Saved{ID: "@work"})
	if cfg.Model != "llama" {
		t.Fatalf("Model = %q, want the entry's own model", cfg.Model)
	}
}

func TestStartupFallsBackToTheDefaultWithANote(t *testing.T) {
	e, cfg, note := testSet(t, startupConfig(t)).Startup(Saved{ID: "@gone", Model: "m"})
	if e.ID != DefaultID {
		t.Fatalf("entry = %#v, want the default", e)
	}
	if cfg.Model != "qwen3-coder" {
		t.Fatalf("Model = %q", cfg.Model)
	}
	for _, want := range []string{"@gone", "config.yaml", "qwen3-coder"} {
		if !strings.Contains(note, want) {
			t.Fatalf("note = %q, missing %q", note, want)
		}
	}
}

func TestStartupWithNothingSavedUsesTheDefaultSilently(t *testing.T) {
	e, _, note := testSet(t, startupConfig(t)).Startup(Saved{})
	if e.ID != DefaultID || note != "" {
		t.Fatalf("entry = %#v note = %q", e, note)
	}
}

func TestStartupSavedModelAppliesOnTheDefaultEndpoint(t *testing.T) {
	cfg := startupConfig(t)
	set := testSet(t, cfg)

	// Test with explicit DefaultID
	e, cfg1, note := set.Startup(Saved{ID: DefaultID, Model: "picked-model"})
	if e.ID != DefaultID {
		t.Fatalf("entry.ID = %q, want %q", e.ID, DefaultID)
	}
	if cfg1.Model != "picked-model" {
		t.Fatalf("config.Model = %q, want picked-model", cfg1.Model)
	}
	if e.Model != "picked-model" {
		t.Fatalf("entry.Model = %q, want picked-model", e.Model)
	}
	if note != "" {
		t.Fatalf("note = %q, want empty", note)
	}

	// Test with empty ID (also uses default)
	e, cfg2, note := set.Startup(Saved{ID: "", Model: "picked-model"})
	if e.ID != DefaultID {
		t.Fatalf("entry.ID = %q, want %q", e.ID, DefaultID)
	}
	if cfg2.Model != "picked-model" {
		t.Fatalf("config.Model = %q, want picked-model", cfg2.Model)
	}
	if e.Model != "picked-model" {
		t.Fatalf("entry.Model = %q, want picked-model", e.Model)
	}
	if note != "" {
		t.Fatalf("note = %q, want empty", note)
	}
}
