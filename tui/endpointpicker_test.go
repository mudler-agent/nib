package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/types"
)

// newEndpointTestModel is newLoginTestModel with a named config.yaml
// endpoint ("@home") added, so /endpoint's picker has a named entry besides
// the default and the registry to list.
func newEndpointTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("REGOLO_API_KEY", "")
	m := newModelSwitchTestModel(t, "local-a", "local-b")
	cfg := m.cfg
	cfg.BaseDir = t.TempDir()
	cfg.Endpoints = types.Endpoints{{
		Name: "home",
		ModelProviderConfig: types.ModelProviderConfig{
			BaseURL: "http://nas:8080/v1",
			Model:   "qwen-30b",
		},
	}}
	s, err := chat.NewSession(context.Background(), cfg, chat.Callbacks{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	m.cfg, m.session = cfg, s
	return m
}

func TestEndpointOpensPickerWithDefaultNamedAndRegistry(t *testing.T) {
	m := newEndpointTestModel(t)
	if cmd := m.dispatchResolved("/endpoint"); cmd != nil {
		t.Fatal("/endpoint must not start a turn")
	}
	if !m.providerPicker.active {
		t.Fatal("/endpoint should open the picker")
	}
	var sawDefault, sawNamed, sawRegistry bool
	for _, e := range m.providerPicker.all {
		switch e.ID {
		case chat.ConfigProviderID:
			sawDefault = true
		case "@home":
			sawNamed = true
		case "regolo":
			sawRegistry = true
		}
	}
	if !sawDefault || !sawNamed || !sawRegistry {
		t.Fatalf("picker entries = %+v, want the default, @home and a registry provider", m.providerPicker.all)
	}
}

func TestEndpointPickerRowShowsStatus(t *testing.T) {
	m := newEndpointTestModel(t)
	m.dispatchResolved("/endpoint")
	// dispatchResolved runs outside Update's normal loop, which is what calls
	// updateViewport (see handleProviderPickerKey) to bake the active dialog
	// into the viewport's cached content that View() then just replays.
	m.updateViewport()
	view := m.View()
	if !strings.Contains(view, "@home") {
		t.Fatalf("picker view missing the named endpoint:\n%s", view)
	}
	if !strings.Contains(view, "qwen-30b @ nas:8080") {
		t.Fatalf("picker view missing the named endpoint's status (model @ host):\n%s", view)
	}
}

func TestEndpointSwitchesDirectlyToNamedEndpoint(t *testing.T) {
	m := newEndpointTestModel(t)
	if cmd := m.dispatchResolved("/endpoint @home"); cmd != nil {
		t.Fatal("/endpoint <id> must not start a turn")
	}
	if m.providerPicker.active {
		t.Fatal("/endpoint <id> switches directly, it does not open the picker")
	}
	if got := m.session.EndpointID(); got != "@home" {
		t.Fatalf("EndpointID = %q, want @home", got)
	}
	if got := m.session.Model(); got != "qwen-30b" {
		t.Fatalf("Model = %q, want qwen-30b", got)
	}
	if msg := lastMessage(t, m); !strings.Contains(msg.Content, "@home") || !strings.Contains(msg.Content, "qwen-30b") {
		t.Fatalf("confirmation message = %q, want it to name the endpoint and model", msg.Content)
	}
}

func TestEndpointPickerFiltersByTyping(t *testing.T) {
	m := newEndpointTestModel(t)
	m.dispatchResolved("/endpoint")
	m, _ = press(t, m, typeText("home"))
	if len(m.providerPicker.matches) != 1 || m.providerPicker.matches[0].ID != "@home" {
		t.Fatalf("filtering by 'home' = %+v, want only @home", m.providerPicker.matches)
	}
}

func TestLoginPickerExcludesDefaultAndNamedEndpoints(t *testing.T) {
	m := newEndpointTestModel(t)
	m.dispatchResolved("/login")
	if !m.providerPicker.active {
		t.Fatal("/login should open the picker")
	}
	if len(m.providerPicker.all) == 0 {
		t.Fatal("/login picker unexpectedly empty")
	}
	for _, e := range m.providerPicker.all {
		if e.ID == chat.ConfigProviderID || e.ID == "@home" {
			t.Fatalf("/login picker listed %+v; it must be authentication-only, no config.yaml default or named endpoints", e)
		}
	}
}

func TestEndpointUnknownIDReportsError(t *testing.T) {
	m := newEndpointTestModel(t)
	if cmd := m.dispatchResolved("/endpoint @nope"); cmd != nil {
		t.Fatal("/endpoint <unknown> must not start a turn")
	}
	if m.providerPicker.active {
		t.Fatal("an unknown endpoint must not open the picker")
	}
	msg := lastMessage(t, m)
	if msg.Role != "error" || !strings.Contains(msg.Content, "@nope") {
		t.Fatalf("unknown-endpoint message = %+v, want an error naming @nope", msg)
	}
	if m.session.EndpointID() == "@nope" {
		t.Fatal("the session must not have switched to an unknown endpoint")
	}
}
