package render

import (
	"github.com/mudler/nib/theme"
)

// Loader composes the working indicator: spinner frame and status verb.
// spinner is the already-chosen animation frame (theme.SpinnerFrames picks
// that; Loader does not reimplement frame selection).
func Loader(spinner, status string) string {
	left := spinner
	if status != "" {
		if left != "" {
			left += " "
		}
		left += theme.Reasoning.Render(status)
	}
	return left
}
