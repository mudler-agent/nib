package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/types"
)

// newRegoloOverrideModel reproduces the reported setup: config.yaml names one
// model ("uncensored") while a /login pick (Regolo, glm5.2) is what the
// session really talks to. BaseDir is a temp dir so neither the user's real
// provider.json nor credentials.json leaks in. No request is ever sent.
func newRegoloOverrideModel(t *testing.T) Model {
	t.Helper()
	cfg := types.Config{
		Model:      "uncensored",
		BaseURL:    "http://127.0.0.1:1/v1",
		BaseDir:    t.TempDir(),
		Compaction: types.CompactionConfig{MaxContextTokens: 128000},
	}
	s, err := chat.NewSession(context.Background(), cfg, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.SaveAPIKey("regolo", "rg-secret", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SwitchProvider("regolo", "glm5.2"); err != nil {
		t.Fatal(err)
	}

	m := newQueueTestModel()
	m.ctx = context.Background()
	m.cfg = cfg
	m.session = s
	return m
}

func bootLine(t *testing.T, b *bootState, ev string) string {
	t.Helper()
	for _, e := range b.entries {
		if e.ev == ev {
			return e.dt
		}
	}
	t.Fatalf("boot log has no %q line: %+v", ev, b.entries)
	return ""
}

// The boot log's provider and model lines name what requests really use, and
// say so when a /login pick overrides config.yaml's model.
func TestBootLogNamesTheActiveProviderAndModel(t *testing.T) {
	m := newRegoloOverrideModel(t)
	m.boot = newBootState()
	for range bootScript() {
		m.boot.tick(&m)
	}

	if got := bootLine(t, m.boot, "provider"); got != "Regolo" {
		t.Fatalf("provider line = %q, want Regolo", got)
	}
	model := bootLine(t, m.boot, "model")
	if !strings.HasPrefix(model, "glm5.2") {
		t.Fatalf("model line = %q, want it to start with the active model glm5.2", model)
	}
	if !strings.Contains(model, "config.yaml: uncensored") || !strings.Contains(model, "/login") {
		t.Fatalf("model line = %q, want a note that /login overrides config.yaml's uncensored", model)
	}
}

// Without an override (config.yaml's own endpoint), the model line carries no
// note: there is nothing to reconcile.
func TestBootLogOmitsTheNoteWithoutAnOverride(t *testing.T) {
	m := newModelSwitchTestModel(t, "model-a")
	m.boot = newBootState()
	for range bootScript() {
		m.boot.tick(&m)
	}
	if got := bootLine(t, m.boot, "model"); got != "model-a" {
		t.Fatalf("model line = %q, want just model-a", got)
	}
	if got := bootLine(t, m.boot, "provider"); got != chat.ConfigProviderName {
		t.Fatalf("provider line = %q, want %q", got, chat.ConfigProviderName)
	}
}

// The session is built asynchronously, so the provider/model lines can be
// printed from config.yaml before it exists. sessionReadyMsg must correct them
// rather than leave config.yaml's model on screen.
func TestBootLogIsCorrectedWhenTheSessionArrives(t *testing.T) {
	m := newRegoloOverrideModel(t)
	s := m.session
	m.session = nil
	m.sessionReady = false
	m.boot = newBootState()
	for range 4 { // core/init, config, provider, model
		m.boot.tick(&m)
	}
	if got := bootLine(t, m.boot, "model"); got != "uncensored" {
		t.Fatalf("pre-session model line = %q, want the config.yaml fallback", got)
	}

	next, _ := m.Update(sessionReadyMsg{session: s})
	m = next.(Model)
	if got := bootLine(t, m.boot, "provider"); got != "Regolo" {
		t.Fatalf("provider line after session = %q, want Regolo", got)
	}
	if got := bootLine(t, m.boot, "model"); !strings.HasPrefix(got, "glm5.2") {
		t.Fatalf("model line after session = %q, want glm5.2", got)
	}
}

// A session that is ready before the provider/model ticks fire flushes them
// through markReady, which must fill in real details too, not blanks.
func TestBootLogFlushFillsProviderAndModel(t *testing.T) {
	m := newRegoloOverrideModel(t)
	m.boot = newBootState()
	m.boot.markReady(&m)
	if got := bootLine(t, m.boot, "provider"); got != "Regolo" {
		t.Fatalf("flushed provider line = %q, want Regolo", got)
	}
	if got := bootLine(t, m.boot, "model"); !strings.HasPrefix(got, "glm5.2") {
		t.Fatalf("flushed model line = %q, want glm5.2", got)
	}
}

func TestHeaderStatsCarryTheActiveProvider(t *testing.T) {
	m := newRegoloOverrideModel(t)
	hs := m.viewState().HeaderStats
	if hs.Provider != "regolo" || hs.Model != "glm5.2" {
		t.Fatalf("header stats = %+v, want provider regolo and model glm5.2", hs)
	}

	cfgModel := newModelSwitchTestModel(t, "model-a")
	if hs := cfgModel.viewState().HeaderStats; hs.Provider != chat.ConfigProviderName {
		t.Fatalf("config.yaml header provider = %q, want %q", hs.Provider, chat.ConfigProviderName)
	}
}

// /model lists the current provider's models only, so its picker names that
// provider and points at /login for switching provider.
func TestModelPickerNamesTheCurrentProvider(t *testing.T) {
	m := newRegoloOverrideModel(t)
	m.modelPicker.open(1)
	m.modelPicker.setModels([]string{"glm5.2", "qwen"}, "glm5.2")
	d := m.buildModelPickerDialog()
	if !strings.HasPrefix(d.Title, "Regolo ") {
		t.Fatalf("title = %q, want it to name Regolo", d.Title)
	}
	if !strings.Contains(d.Hint, theme.ModelPickerLoginHint) {
		t.Fatalf("hint = %q, want the /login hint", d.Hint)
	}
}

// Picking a model on a /login provider persists it to provider.json; the
// confirmation says so, so the next start is no surprise.
func TestModelPickerConfirmsTheSavedDefault(t *testing.T) {
	m := newRegoloOverrideModel(t)
	m.modelPicker.open(1)
	m.modelPicker.setModels([]string{"glm5.2", "qwen"}, "glm5.2")
	m.modelPicker.move(1)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	msg := lastMessage(t, m)
	if !strings.Contains(msg.Content, "Regolo") || !strings.Contains(msg.Content, "qwen") || !strings.Contains(msg.Content, theme.ProviderSavedDefault) {
		t.Fatalf("confirmation = %q, want provider, model and %q", msg.Content, theme.ProviderSavedDefault)
	}
}

func TestModelsListingStartsWithTheProvider(t *testing.T) {
	m := newModelSwitchTestModel(t, "model-a", "model-b")
	if cmd := m.dispatchResolved("/models"); cmd != nil {
		t.Fatal("/models must not start a turn")
	}
	msg := lastMessage(t, m)
	if !strings.Contains(msg.Content, chat.ConfigProviderName+" models:") {
		t.Fatalf("listing = %q, want it headed by the provider", msg.Content)
	}
}
