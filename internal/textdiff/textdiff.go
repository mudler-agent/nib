// Package textdiff computes line diffs for display: what an edit or write
// changed in a file, grouped into hunks with a little surrounding context. It
// has no dependencies and no notion of styling — the chat layer builds diffs
// from tool calls and the render layer paints them.
package textdiff

import "strings"

// Kind classifies one line of a diff.
type Kind int

const (
	Context Kind = iota // unchanged line shown around a change
	Add                 // line present only in the new text
	Del                 // line present only in the old text
	Gap                 // elided run of unchanged lines between two hunks
)

// Line is one displayed row of a diff. OldNo and NewNo are 1-based line
// numbers in the old and new text; 0 means the side has no such line (an Add
// has no OldNo) or the numbers are unknown (a fragment diff, see Fragment).
type Line struct {
	Kind  Kind
	OldNo int
	NewNo int
	Text  string
}

// Diff is a displayable diff: its rows (hunks separated by Gap rows) and the
// total count of added and removed lines.
type Diff struct {
	Lines   []Line
	Added   int
	Removed int
}

// Empty reports whether the diff changes nothing.
func (d Diff) Empty() bool { return d.Added == 0 && d.Removed == 0 }

// maxCells bounds the LCS table. The common prefix and suffix are trimmed
// before the table is built, so only a rewrite of a large region reaches this;
// past it the changed region is shown as a block delete followed by a block
// add, which is still a correct (if less minimal) diff.
const maxCells = 4_000_000

// Compute diffs old against new line by line and groups the result into hunks
// with ctx lines of context on each side. A negative ctx keeps every
// unchanged line (no Gap rows).
func Compute(old, new string, ctx int) Diff {
	a, b := splitLines(old), splitLines(new)
	ops := lineOps(a, b)
	var d Diff
	for _, op := range ops {
		switch op.Kind {
		case Add:
			d.Added++
		case Del:
			d.Removed++
		}
	}
	d.Lines = hunks(ops, ctx)
	return d
}

// Fragment diffs two snippets whose position in the file is unknown — an edit's
// old and new strings when the file itself could not be read. Every line is
// kept and line numbers are cleared, so a renderer shows no number gutter
// rather than numbers that would be wrong.
func Fragment(old, new string) Diff {
	d := Compute(old, new, -1)
	for i := range d.Lines {
		d.Lines[i].OldNo, d.Lines[i].NewNo = 0, 0
	}
	return d
}

// splitLines splits text into lines, dropping the empty element a trailing
// newline would leave, so "a\nb\n" and "a\nb" are both two lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// lineOps returns the full edit script: every line of a and b as a Context,
// Del or Add row, with line numbers filled in.
func lineOps(a, b []string) []Line {
	// Trim the common prefix and suffix, which is all of a typical edit.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	midA, midB := a[pre:len(a)-suf], b[pre:len(b)-suf]

	out := make([]Line, 0, len(a)+len(b))
	oldNo, newNo := 1, 1
	emit := func(k Kind, text string) {
		l := Line{Kind: k, Text: text}
		switch k {
		case Context:
			l.OldNo, l.NewNo = oldNo, newNo
			oldNo++
			newNo++
		case Del:
			l.OldNo = oldNo
			oldNo++
		case Add:
			l.NewNo = newNo
			newNo++
		}
		out = append(out, l)
	}

	for _, s := range a[:pre] {
		emit(Context, s)
	}
	if len(midA)*len(midB) > maxCells {
		for _, s := range midA {
			emit(Del, s)
		}
		for _, s := range midB {
			emit(Add, s)
		}
	} else {
		for _, op := range lcsOps(midA, midB) {
			switch op.kind {
			case Context:
				emit(Context, midA[op.i])
			case Del:
				emit(Del, midA[op.i])
			case Add:
				emit(Add, midB[op.j])
			}
		}
	}
	for _, s := range a[len(a)-suf:] {
		emit(Context, s)
	}
	return out
}

type op struct {
	kind Kind
	i, j int
}

// lcsOps walks a longest-common-subsequence table to an edit script. Within a
// changed run it emits deletions before additions, the order people expect to
// read a replacement in.
func lcsOps(a, b []string) []op {
	n, m := len(a), len(b)
	// t[i][j] = LCS length of a[i:], b[j:], flattened.
	t := make([]int32, (n+1)*(m+1))
	at := func(i, j int) int32 { return t[i*(m+1)+j] }
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				t[i*(m+1)+j] = at(i+1, j+1) + 1
			} else {
				t[i*(m+1)+j] = max(at(i+1, j), at(i, j+1))
			}
		}
	}
	ops := make([]op, 0, n+m)
	i, j := 0, 0
	var adds []op
	flush := func() { ops = append(ops, adds...); adds = adds[:0] }
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			flush()
			ops = append(ops, op{Context, i, j})
			i++
			j++
		case j < m && (i == n || at(i, j+1) > at(i+1, j)):
			adds = append(adds, op{Add, i, j})
			j++
		default:
			ops = append(ops, op{Del, i, j})
			i++
		}
	}
	flush()
	return ops
}

// hunks keeps each changed line plus ctx unchanged lines on either side,
// replacing every longer unchanged run with one Gap row. ctx < 0 keeps all.
func hunks(ops []Line, ctx int) []Line {
	if ctx < 0 {
		return ops
	}
	keep := make([]bool, len(ops))
	for i, l := range ops {
		if l.Kind == Context {
			continue
		}
		for k := max(0, i-ctx); k <= min(len(ops)-1, i+ctx); k++ {
			keep[k] = true
		}
	}
	var out []Line
	gap := false
	for i, l := range ops {
		if keep[i] {
			if gap && len(out) > 0 {
				out = append(out, Line{Kind: Gap})
			}
			gap = false
			out = append(out, l)
			continue
		}
		gap = true
	}
	return out
}
