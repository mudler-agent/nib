package tui

import (
	"fmt"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// goalFooterRow returns the plain {Glyph, Text, Kind} data for the goal
// footer, and whether there is one to show. The presenter styles it
// (render.FooterGoal gets the original theme.Subtle, unfilled treatment —
// see inline.Footer).
//
// A paused goal (an interrupt stopped its pursuit) still shows, marked paused,
// so the user can see it is kept and how to pick it up again.
func goalFooterRow(goal string, paused bool) (render.FooterRow, bool) {
	if goal == "" {
		return render.FooterRow{}, false
	}
	text := fmt.Sprintf("goal: %s  (/goal clear)", render.TruncateRunes(goal, 48))
	if paused {
		text = fmt.Sprintf("goal (paused): %s  (/goal resume · /goal clear)", render.TruncateRunes(goal, 48))
	}
	return render.FooterRow{Glyph: theme.Goal, Text: text, Kind: render.FooterGoal}, true
}
