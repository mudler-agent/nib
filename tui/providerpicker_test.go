package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/llmprovider"
)

// newLoginTestModel is newModelSwitchTestModel with its credential store in a
// temp dir, so /login never touches the real ~/.config/nib.
func newLoginTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("REGOLO_API_KEY", "")
	m := newModelSwitchTestModel(t, "local-a", "local-b")
	cfg := m.cfg
	cfg.BaseDir = t.TempDir()
	s, err := chat.NewSession(context.Background(), cfg, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	m.cfg, m.session = cfg, s
	return m
}

func press(t *testing.T, m Model, keys ...tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var next tea.Model
		next, cmd = m.Update(k)
		m = next.(Model)
	}
	return m, cmd
}

func typeText(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
)

func TestLoginOpensProviderPicker(t *testing.T) {
	m := newLoginTestModel(t)
	if cmd := m.dispatchResolved("/login"); cmd != nil {
		t.Fatal("/login must not start a turn")
	}
	if !m.providerPicker.active || len(m.messages) != 0 {
		t.Fatalf("/login should open the picker, not print: picker=%v messages=%+v", m.providerPicker.active, m.messages)
	}
	// /login is authentication-only: it never lists config.yaml's default
	// endpoint (that's /endpoint's job), so nothing in its list can be marked
	// Current while the session sits on that default.
	if e, _ := m.providerPicker.choice(); e.ID == chat.ConfigProviderID || e.Current {
		t.Fatalf("initial selection = %+v, want a registry provider, not config.yaml's default", e)
	}
	m, _ = press(t, m, typeText("regolo"))
	if e, ok := m.providerPicker.choice(); !ok || e.ID != "regolo" {
		t.Fatalf("filtering by 'regolo' selected %+v", e)
	}
	view := m.View()
	if !strings.Contains(view, "Regolo") || !strings.Contains(view, "not logged in") {
		t.Fatalf("picker view lacks the provider row and its state:\n%s", view)
	}
	m, _ = press(t, m, keyEsc)
	if m.providerPicker.active || m.quitting {
		t.Fatal("Esc should close the picker, not quit")
	}
}

func TestLoginAPIKeyInTUIThenPickModel(t *testing.T) {
	m := newLoginTestModel(t)
	m.dispatchResolved("/login")
	m, _ = press(t, m, typeText("regolo"), keyEnter)
	if !m.loginForm.active {
		t.Fatal("Enter on a provider without a key should open the key form")
	}

	m, _ = press(t, m, typeText("rg-super-secret-key"))
	if view := m.View(); strings.Contains(view, "rg-super-secret-key") || !strings.Contains(view, "•") {
		t.Fatalf("the key must be masked on screen:\n%s", view)
	}

	m, cmd := press(t, m, keyEnter)
	if m.loginForm.active || cmd == nil {
		t.Fatal("submitting the form should save the key and load the provider's models")
	}
	if !m.modelPicker.active || m.modelPicker.target == nil || m.modelPicker.target.ID != "regolo" {
		t.Fatalf("model picker after login = %+v", m.modelPicker)
	}
	if !strings.Contains(lastMessage(t, m).Content, "Logged in to Regolo") {
		t.Fatalf("login notice = %q", lastMessage(t, m).Content)
	}
	if m.session.ProviderID() != chat.ConfigProviderID {
		t.Fatal("the session must not switch provider before a model is picked")
	}

	// A provider whose list cannot be fetched still gets a model: typed.
	next, _ := m.Update(modelListMsg{requestID: m.modelPicker.requestID, err: errors.New("401")})
	m = next.(Model)
	if !m.modelPicker.active || !m.modelPicker.typed {
		t.Fatal("a failing model list must fall back to typing the name")
	}
	m, _ = press(t, m, typeText("Llama-3.3-70B-Instruct"), keyEnter)
	if m.session.ProviderID() != "regolo" || m.session.Model() != "Llama-3.3-70B-Instruct" {
		t.Fatalf("after picking: provider=%q model=%q", m.session.ProviderID(), m.session.Model())
	}

	// Logged in now: the picker goes straight to the model list.
	m.dispatchResolved("/login")
	m, _ = press(t, m, typeText("regolo"), keyEnter)
	if m.loginForm.active || !m.modelPicker.active {
		t.Fatal("a logged-in provider should skip the key form")
	}
	m, _ = press(t, m, keyEsc)

	// And back to config.yaml — via /endpoint, since /login no longer lists
	// (or switches to) the config.yaml default; it is authentication-only.
	m.dispatchResolved("/endpoint config")
	if m.session.ProviderID() != chat.ConfigProviderID || m.session.Model() != "local-a" {
		t.Fatalf("switching back: provider=%q model=%q", m.session.ProviderID(), m.session.Model())
	}
}

func TestLoginFormRejectsEmptyKeyAndCancels(t *testing.T) {
	m := newLoginTestModel(t)
	m.dispatchResolved("/login regolo")
	if !m.loginForm.active {
		t.Fatal("/login <id> should open the key form directly")
	}
	m, _ = press(t, m, keyEnter)
	if !m.loginForm.active || m.loginForm.err == "" {
		t.Fatal("an empty key must be refused in place")
	}
	m, _ = press(t, m, keyEsc)
	if m.loginForm.active || m.quitting {
		t.Fatal("Esc should cancel the form, not quit")
	}
	for _, e := range m.session.Providers() {
		if e.ID == "regolo" && e.Stored {
			t.Fatal("a cancelled form stored a credential")
		}
	}
}

func TestLogoutPicker(t *testing.T) {
	m := newLoginTestModel(t)
	m.dispatchResolved("/logout")
	if m.providerPicker.active || !strings.Contains(lastMessage(t, m).Content, "not logged in") {
		t.Fatal("/logout with no logins should say so, not open an empty picker")
	}
	if _, err := m.session.SaveAPIKey("regolo", "k", ""); err != nil {
		t.Fatal(err)
	}
	m.dispatchResolved("/logout")
	if !m.providerPicker.active || len(m.providerPicker.all) != 1 {
		t.Fatalf("/logout picker = %+v, want only the stored login", m.providerPicker.all)
	}
	m, _ = press(t, m, keyEnter)
	if !strings.Contains(lastMessage(t, m).Content, "Logged out of Regolo") {
		t.Fatalf("logout notice = %q", lastMessage(t, m).Content)
	}
}

func TestProviderModelPickerNoListTypesName(t *testing.T) {
	m := newLoginTestModel(t)
	m.openProviderModelPicker(chat.ProviderEntry{ID: "anthropic", Name: "Anthropic"})
	next, _ := m.Update(modelListMsg{requestID: m.modelPicker.requestID, err: llmprovider.ErrNoModelList})
	m = next.(Model)
	if !m.modelPicker.typed {
		t.Fatal("no model list should switch the picker to typing")
	}
	if view := m.View(); !strings.Contains(view, "type the model name") {
		t.Fatalf("typed-mode hint missing:\n%s", view)
	}
}

func TestModelPickerAcceptsTypedNameForPartialList(t *testing.T) {
	m := newLoginTestModel(t)
	m.openModelPicker()
	next, _ := m.Update(modelListMsg{requestID: m.modelPicker.requestID, models: []string{"gemini-2.5-pro"}, partial: true})
	m = next.(Model)
	m, _ = press(t, m, typeText("gemini-9-ultra"))
	if view := m.View(); !strings.Contains(view, "enter uses the typed name") {
		t.Fatalf("partial list should say a typed name works:\n%s", view)
	}
	m, _ = press(t, m, keyEnter)
	if m.session.Model() != "gemini-9-ultra" {
		t.Fatalf("model = %q, want the typed name", m.session.Model())
	}
}
