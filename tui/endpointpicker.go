package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/endpoint"
	"github.com/mudler/nib/theme"
)

// openEndpointPicker opens /endpoint's picker: every entry the session can
// talk to — the config.yaml default, its named endpoints, then the registry
// — as one homogeneous list. /login stays authentication-only (pickerLogin
// lists the registry alone); this is the picker that makes named endpoints
// visible.
func (m *Model) openEndpointPicker() {
	m.openProviderPicker(pickerEndpoint)
}

// useEndpoint is Enter in the /endpoint picker, and what a direct
// `/endpoint <id>` resolves to.
//
// A registry entry behaves exactly like /login: useProvider decides between
// authenticating and going straight to model selection, unchanged. The
// default endpoint or a named one that already names a model switches
// immediately — SwitchProvider keeps conversation history, the way /login's
// config.yaml entry always has. A named endpoint with no model chains into
// the model picker instead of failing: a model-less endpoint is the normal
// way to describe a local server whose models are listed on open, not typed
// into config.yaml.
func (m *Model) useEndpoint(e chat.ProviderEntry) tea.Cmd {
	if e.Kind == endpoint.KindProvider {
		return m.useProvider(e, false)
	}
	if e.Model == "" {
		return m.openProviderModelPicker(e)
	}
	if err := m.session.SwitchProvider(e.ID, ""); err != nil {
		m.appendMessage(ChatMessage{Role: "error", Content: err.Error()})
		return nil
	}
	m.appendMessage(ChatMessage{Role: "agent", Content: fmt.Sprintf(theme.EndpointSwitched, e.Name, m.session.Model())})
	return nil
}
