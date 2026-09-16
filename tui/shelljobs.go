package tui

import (
	"fmt"
	"strings"

	wizmcp "github.com/mudler/nib/mcp"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// shellJobsFooterRow returns the plain {Glyph, Text, Kind} data for the
// shell-jobs footer, and whether there is one to show. The presenter styles
// it (render.FooterShell gets the original theme.Meta + width-fill treatment
// — see inline.Footer).
func shellJobsFooterRow(jobs []wizmcp.ShellJobInfo) (render.FooterRow, bool) {
	if len(jobs) == 0 {
		return render.FooterRow{}, false
	}
	var running, done, failed int
	for _, j := range jobs {
		switch j.Status {
		case "running":
			running++
		case "completed":
			done++
		case "failed":
			failed++
		}
	}
	parts := []string{fmt.Sprintf("shell: %d running", running)}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", done))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	parts = append(parts, "(ctrl+b background · ctrl+o logs)")
	return render.FooterRow{Glyph: theme.ShellJob, Text: strings.Join(parts, "  ·  "), Kind: render.FooterShell}, true
}
