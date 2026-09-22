package theme

import (
	"math"
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

// Motion: the few animated inks. Each is a blend between two palette inks,
// computed in Lab space so the midpoints do not go muddy. The result is a
// hex color; lipgloss degrades it to the nearest ANSI 256 or 16 color on a
// terminal without true color, so the animation still steps there.

// Blend returns the ink t of the way from a to b, with t clamped to [0, 1].
// a and b are palette inks (ANSI 256 codes, as in the Inks block).
func Blend(a, b lipgloss.Color, t float64) lipgloss.Color {
	switch {
	case t <= 0:
		return a
	case t >= 1:
		return b
	}
	ca, okA := inkRGB(a)
	cb, okB := inkRGB(b)
	if !okA || !okB {
		return b
	}
	return lipgloss.Color(ca.BlendLab(cb, t).Clamped().Hex())
}

// inkRGB converts a palette ink (an ANSI 256 code) to RGB.
func inkRGB(c lipgloss.Color) (colorful.Color, bool) {
	n, err := strconv.Atoi(string(c))
	if err != nil || n < 0 || n > 255 {
		return colorful.Color{}, false
	}
	return termenv.ConvertToRGB(termenv.ANSI256Color(n)), true
}

// CursorPulsePeriod is one full breath of the streaming cursor.
const CursorPulsePeriod = 1200 * time.Millisecond

// StreamCursorAt renders the streaming cursor at elapsed time since the reply
// started. It breathes between Faint and Accent on a sine, so it reads as
// alive without blinking hard.
func StreamCursorAt(elapsed time.Duration) string {
	phase := float64(elapsed%CursorPulsePeriod) / float64(CursorPulsePeriod)
	t := (1 - math.Cos(2*math.Pi*phase)) / 2
	return lipgloss.NewStyle().Foreground(Blend(Faint, Accent, 0.35+0.65*t)).Render(StreamCursor)
}

// FadeDuration is how long a new transcript entry takes to reach its full ink.
const FadeDuration = 240 * time.Millisecond

// FadeIn returns ink as seen at elapsed time since its entry arrived: it
// eases out from Faint to ink over FadeDuration.
func FadeIn(ink lipgloss.Color, elapsed time.Duration) lipgloss.Color {
	if elapsed >= FadeDuration {
		return ink
	}
	t := float64(elapsed) / float64(FadeDuration)
	t = 1 - (1-t)*(1-t)*(1-t) // ease-out cubic
	return Blend(Faint, ink, t)
}
