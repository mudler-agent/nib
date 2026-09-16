package render

import (
	"strings"
	"testing"

	"github.com/mudler/nib/theme"
)

func TestLoaderComposesSpinnerAndStatus(t *testing.T) {
	got := Loader("-", "thinking")
	if !strings.Contains(got, "-") || !strings.Contains(got, "thinking") {
		t.Errorf("Loader() = %q, want it to contain spinner and status", got)
	}
}

// TestLoaderExactComposition pins Loader's actual contract: since the
// trailer parameter was dropped, Loader is exactly "spinner + \" \" +
// styled-status" with no padding path at all — spinner and status
// (theme.SpinnerFrames output, and a caller-chosen verb) never carry
// whitespace of their own, so a bare "no trailing whitespace" check
// (the previous form of this test) can never fail no matter what Loader
// does internally. Asserting the exact string for each arity (both,
// spinner-only, status-only, neither) actually fails if a future change
// reintroduces a trailing separator or otherwise changes the join.
func TestLoaderExactComposition(t *testing.T) {
	cases := []struct {
		name    string
		spinner string
		status  string
		want    string
	}{
		{"both", "-", "working", "- " + theme.Reasoning.Render("working")},
		{"spinner only", "-", "", "-"},
		{"status only", "", "working", theme.Reasoning.Render("working")},
		{"neither", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Loader(c.spinner, c.status); got != c.want {
				t.Errorf("Loader(%q, %q) = %q, want %q", c.spinner, c.status, got, c.want)
			}
		})
	}
}
