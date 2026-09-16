package render

import "strings"

// BlockRows reports how many terminal rows a rendered block occupies when it
// is concatenated straight onto whatever follows it — which is exactly what
// every Frame implementation does: header, body, dialogs, composer and footer
// are written back to back with no separator of their own.
//
// That makes the row count a property of the newlines, not of lipgloss.Height:
// a block that ends with "\n" leaves the next block starting on a fresh row, so
// it occupies one row per newline and lipgloss.Height (which counts the empty
// piece after the final newline as a row) would over-count it by one. A block
// that does NOT end with a newline shares its last row with whatever follows;
// it is counted whole, so a layout budget built on this can only ever
// over-reserve, never under-reserve. Both presenters' Header and Dialog end
// with a newline, so today the first branch is the one that runs.
func BlockRows(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if strings.HasSuffix(s, "\n") {
		return n
	}
	return n + 1
}
