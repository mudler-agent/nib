package endpoint

import (
	"path/filepath"
	"testing"

	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/types"
)

func testSet(t *testing.T, cfg types.Config) *Set {
	t.Helper()
	store := auth.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
	s, errs := New(cfg, store)
	if len(errs) != 0 {
		t.Fatalf("New returned errors: %v", errs)
	}
	return s
}

func TestListStartsWithDefaultThenNamedInFileOrder(t *testing.T) {
	cfg := types.Config{
		Model: "qwen3-coder", BaseURL: "http://localhost:8080/v1",
		Endpoints: types.Endpoints{
			{Name: "zeta", ModelProviderConfig: types.ModelProviderConfig{BaseURL: "http://zeta/v1"}},
			{Name: "alpha", ModelProviderConfig: types.ModelProviderConfig{BaseURL: "http://alpha/v1", Model: "m"}},
		},
	}
	got := testSet(t, cfg).List()
	if len(got) < 3 {
		t.Fatalf("got %d entries", len(got))
	}
	if got[0].ID != DefaultID || got[0].Kind != KindDefault {
		t.Fatalf("entry 0 = %#v", got[0])
	}
	if got[1].ID != "@zeta" || got[1].Kind != KindNamed {
		t.Fatalf("entry 1 = %#v", got[1])
	}
	if got[2].ID != "@alpha" || got[2].Model != "m" {
		t.Fatalf("entry 2 = %#v", got[2])
	}
	if got[3].Kind != KindProvider {
		t.Fatalf("entry 3 = %#v, want the registry to follow", got[3])
	}
}

func TestNamedEntryWithoutModelSaysModelsAreListedOnOpen(t *testing.T) {
	cfg := types.Config{Endpoints: types.Endpoints{
		{Name: "scratch", ModelProviderConfig: types.ModelProviderConfig{BaseURL: "http://gpu-box:8080/v1"}},
	}}
	e, ok := testSet(t, cfg).Lookup("@scratch")
	if !ok {
		t.Fatal("@scratch not found")
	}
	if e.Model != "" {
		t.Fatalf("Model = %q, want empty", e.Model)
	}
	if e.Status != "gpu-box:8080 · models listed on open" {
		t.Fatalf("Status = %q", e.Status)
	}
}

func TestNewReportsInvalidEndpointsAndKeepsTheRest(t *testing.T) {
	cfg := types.Config{Endpoints: types.Endpoints{
		{Name: "ok", ModelProviderConfig: types.ModelProviderConfig{BaseURL: "http://ok/v1"}},
		{Name: "bad", ModelProviderConfig: types.ModelProviderConfig{Model: "m"}},
	}}
	store := auth.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
	s, errs := New(cfg, store)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want 1", errs)
	}
	if _, ok := s.Lookup("@bad"); ok {
		t.Fatal("@bad should have been dropped")
	}
	if _, ok := s.Lookup("@ok"); !ok {
		t.Fatal("@ok should have been kept")
	}
}

func TestLookupUnknownID(t *testing.T) {
	if _, ok := testSet(t, types.Config{}).Lookup("@nope"); ok {
		t.Fatal("unknown id resolved")
	}
}
