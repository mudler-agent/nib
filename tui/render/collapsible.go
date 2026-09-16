package render

// CollapsibleBox caps a body to MaxLines. It backs the live model-reasoning
// trace (Phase 3 Task 10): collapsed, it shows the LAST MaxLines rather than
// the first. A head-anchored box would freeze on the trace's opening words
// and never move once the body outgrows MaxLines, which reads to the user as
// a hang — tailing makes the collapsed box double as its own progress
// indicator. Expanded, it shows every line.
type CollapsibleBox struct {
	Lines     []string
	MaxLines  int
	Collapsed bool
}

// Visible returns the lines this box currently shows: everything when
// expanded, or not truncated to fewer lines than MaxLines; otherwise the
// trailing MaxLines lines.
func (b CollapsibleBox) Visible() []string {
	if !b.Collapsed || b.MaxLines <= 0 || len(b.Lines) <= b.MaxLines {
		return b.Lines
	}
	return b.Lines[len(b.Lines)-b.MaxLines:]
}

// Hidden reports how many leading lines Visible omits.
func (b CollapsibleBox) Hidden() int {
	return len(b.Lines) - len(b.Visible())
}
