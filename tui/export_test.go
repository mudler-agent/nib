package tui

import (
	"github.com/mudler/nib/tui/render"
	"github.com/mudler/nib/tui/render/inline"
)

// testPresenter returns the inline Presenter for tests that need a non-nil
// Presenter but do not care which surface renders — the same default
// newTestModel (model_test.go) falls back to for a nil fields.presenter.
func testPresenter() render.Presenter {
	return inline.New()
}
