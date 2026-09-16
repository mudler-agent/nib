package inline

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR escape sequences so column measurements count only
// visible runes.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func TestMessageRendersRolePrefixes(t *testing.T) {
	p := New()
	cases := []struct {
		name string
		msg  render.Message
		want string
	}{
		{"user", render.Message{Role: render.RoleUser, Content: "hi"}, "you"},
		{"assistant", render.Message{Role: render.RoleAssistant, Content: "hi"}, theme.BrandName},
		{"error", render.Message{Role: render.RoleError, Content: "boom"}, theme.Cross},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := p.Message(c.msg, render.RoleNone, 80)
			if !strings.Contains(got, c.want) {
				t.Errorf("Message(%v) = %q, want it to contain %q", c.msg, got, c.want)
			}
		})
	}
}

func TestContinuationLinesAreIndentedToPrefixWidth(t *testing.T) {
	p := New()
	out := p.Message(render.Message{
		Role:    render.RoleUser,
		Content: strings.Repeat("word ", 40),
	}, render.RoleNone, 30)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output, got %d line(s)", len(lines))
	}
	if !strings.HasPrefix(lines[1], "      ") {
		t.Errorf("continuation line not indented to the prefix width: %q", lines[1])
	}
}

// TestInlineOmitsPrefixOnConsecutiveSameRole: repeating "you ·" down a run of
// messages costs six columns on every line and tells the reader nothing new.
func TestInlineOmitsPrefixOnConsecutiveSameRole(t *testing.T) {
	p := New()
	msg := render.Message{Role: render.RoleUser, Content: "second message"}

	first := p.Message(msg, render.RoleNone, 80)
	second := p.Message(msg, render.RoleUser, 80)

	if !strings.Contains(first, "you") {
		t.Error("the first message of a run must carry its label")
	}
	if strings.Contains(second, "you") {
		t.Error("a consecutive same-role message must not repeat the label")
	}
	if !strings.Contains(second, "second message") {
		t.Error("content lost when the prefix was omitted")
	}
}

// TestInlineConsecutiveRunStaysAligned pins the width half of the same rule:
// dropping the label must not shift the content left. Both renders share one
// ContentWidth (which cannot see prev — see the type's doc), so the spaces
// standing in for the label on a consecutive message must be the exact same
// width as the label itself, or a run's content would drift out of column
// the moment it stopped being the first message.
//
// Covers both RoleUser and RoleAssistant: messagePrefix is one shared
// function branching on role, but a table missing either role would leave
// that role's own width-agreement unpinned. This matters more than usual for
// RoleUser specifically — a live multi-turn session could reproduce a
// genuine consecutive-ASSISTANT run (via mid-run message injection) but not a
// genuine consecutive-USER one in the time available (see the report), so
// this case is the only place that path is exercised at all, live or
// otherwise. (A prior version of this test only covered RoleAssistant —
// review finding.)
func TestInlineConsecutiveRunStaysAligned(t *testing.T) {
	p := New()
	cases := []struct {
		name  string
		role  render.Role
		token string
	}{
		{"user", render.RoleUser, "aligned"},
		{"assistant", render.RoleAssistant, "aligned"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := render.Message{Role: c.role, Content: c.token}

			labeled := p.Message(msg, render.RoleNone, 80)
			continued := p.Message(msg, c.role, 80)

			// The column is the RENDERED width up to the token, not a byte
			// offset: the separator (·) is multi-byte, so strings.Index would
			// overstate the column by counting its extra byte.
			col := func(out string) int {
				idx := strings.Index(stripANSI(out), c.token)
				if idx < 0 {
					return -1
				}
				return lipgloss.Width(stripANSI(out)[:idx])
			}
			if labeled == "" || continued == "" {
				t.Fatalf("expected non-empty output, got %q and %q", labeled, continued)
			}
			if got, want := col(continued), col(labeled); got != want {
				t.Errorf("consecutive message content starts at column %d, want %d (same as the labeled run start): %q vs %q", got, want, continued, labeled)
			}
		})
	}
}

func TestCapsAreInlineWidgetCaps(t *testing.T) {
	c := New().Caps()
	if c.AltScreen {
		t.Error("the inline widget must not take the alt screen")
	}
	if c.Mouse {
		t.Error("the inline widget does not enable mouse reporting")
	}
	if c.OverlayDialogs {
		t.Error("the inline widget stacks dialogs into the scrollback, it does not overlay them")
	}
}

// TestFooterRowsStyledByKind pins the styling this task's review caught
// regressing once: FooterJobs/FooterShell must render with theme.Meta and
// lipgloss's Width fill (which pads short lines AND wraps ones exceeding the
// given width — not just padding), while FooterLoops/FooterGoal (and the
// zero-value FooterKindUnset) render with theme.Subtle, unfilled. Asserted
// against the real styles' own output, not by reflecting on style objects.
func TestFooterRowsStyledByKind(t *testing.T) {
	p := New()
	const width = 20

	footer := func(kind render.FooterRowKind, text string) string {
		out := p.Footer(render.ViewState{
			Footers: []render.FooterRow{{Text: text, Kind: kind}},
		}, width)
		// Footer always leads with the help line (empty here) then "\n" before
		// the row; strip that to isolate the row's own rendering.
		return strings.TrimPrefix(out, "\n")
	}

	t.Run("jobs gets Meta+Width", func(t *testing.T) {
		text := "jobs: 1 running"
		want := theme.Meta.Width(width).Render(text)
		if got := footer(render.FooterJobs, text); got != want {
			t.Errorf("FooterJobs row = %q, want %q", got, want)
		}
	})

	t.Run("shell gets Meta+Width", func(t *testing.T) {
		text := "shell: 1 running"
		want := theme.Meta.Width(width).Render(text)
		if got := footer(render.FooterShell, text); got != want {
			t.Errorf("FooterShell row = %q, want %q", got, want)
		}
	})

	t.Run("loops gets Subtle, unfilled", func(t *testing.T) {
		text := "1 loop(s): x"
		want := theme.Subtle.Render(text)
		if got := footer(render.FooterLoops, text); got != want {
			t.Errorf("FooterLoops row = %q, want %q", got, want)
		}
	})

	t.Run("goal gets Subtle, unfilled", func(t *testing.T) {
		text := "goal: ship it"
		want := theme.Subtle.Render(text)
		if got := footer(render.FooterGoal, text); got != want {
			t.Errorf("FooterGoal row = %q, want %q", got, want)
		}
	})

	t.Run("unset Kind falls back to the plain default, not Jobs styling", func(t *testing.T) {
		text := "some future row"
		want := theme.Subtle.Render(text)
		if got := footer(render.FooterKindUnset, text); got != want {
			t.Errorf("FooterKindUnset row = %q, want %q (the Subtle default, not Meta+Width)", got, want)
		}
	})

	t.Run("Width wraps an over-long jobs row instead of spilling", func(t *testing.T) {
		long := strings.Repeat("x", width*3)
		want := theme.Meta.Width(width).Render(long)
		got := footer(render.FooterJobs, long)
		if got != want {
			t.Fatalf("over-long FooterJobs row = %q, want %q", got, want)
		}
		lines := strings.Split(got, "\n")
		if len(lines) < 2 {
			t.Fatalf("expected the over-long row to wrap onto multiple lines, got %d: %q", len(lines), got)
		}
	})
}
