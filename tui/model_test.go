package tui

import "github.com/mudler/nib/tui/render/inline"

// newTestModel builds a Model for tests that need more than the zero value a
// bare Model{} literal gives, while still guaranteeing a non-nil presenter —
// the same guarantee every production construction path (NewModel, always
// called with a Presenter from app.go) carries. Model.presenter used to have
// a nil fallback in renderer() that quietly rendered the inline surface for
// any Model built without one; that fallback is gone, so a test that reaches
// View() or updateViewport() must build its Model through here instead of a
// bare Model{} literal.
//
// fields is applied on top of a Model whose only preset value is presenter;
// set fields.presenter explicitly to exercise a different Presenter.
func newTestModel(fields Model) Model {
	if fields.presenter == nil {
		fields.presenter = inline.New()
	}
	return fields
}

// withMessages appends transcript entries and returns the Model, so a test can
// seed a transcript in one expression.
func withMessages(m Model, msgs ...ChatMessage) Model {
	m.appendMessage(msgs...)
	return m
}
