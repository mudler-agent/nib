package textdiff

import (
	"fmt"
	"strings"
	"testing"
)

// render prints a diff as unified-style rows so tests compare one string.
func render(d Diff) string {
	var b strings.Builder
	for _, l := range d.Lines {
		switch l.Kind {
		case Context:
			fmt.Fprintf(&b, "%d,%d  %s\n", l.OldNo, l.NewNo, l.Text)
		case Del:
			fmt.Fprintf(&b, "%d,- -%s\n", l.OldNo, l.Text)
		case Add:
			fmt.Fprintf(&b, "-,%d +%s\n", l.NewNo, l.Text)
		case Gap:
			b.WriteString("...\n")
		}
	}
	return b.String()
}

func TestComputeReplacementInMiddle(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\n"
	new := "a\nb\nc\nD\ne\nf\ng\n"
	d := Compute(old, new, 1)
	want := "3,3  c\n4,- -d\n-,4 +D\n5,5  e\n"
	if got := render(d); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if d.Added != 1 || d.Removed != 1 {
		t.Fatalf("stats = +%d -%d, want +1 -1", d.Added, d.Removed)
	}
}

func TestComputeSeparateHunksGetGap(t *testing.T) {
	old := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
	new := "X\n2\n3\n4\n5\n6\n7\n8\nY\n"
	d := Compute(old, new, 1)
	want := "1,- -1\n-,1 +X\n2,2  2\n...\n8,8  8\n9,- -9\n-,9 +Y\n"
	if got := render(d); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestComputeNewFileIsAllAdds(t *testing.T) {
	d := Compute("", "package main\n\nfunc main() {}\n", 2)
	if d.Added != 3 || d.Removed != 0 {
		t.Fatalf("stats = +%d -%d, want +3 -0", d.Added, d.Removed)
	}
	for i, l := range d.Lines {
		if l.Kind != Add || l.NewNo != i+1 {
			t.Fatalf("line %d = %+v, want Add #%d", i, l, i+1)
		}
	}
}

func TestComputeGroupsDeletesBeforeAdds(t *testing.T) {
	d := Compute("x\na\nb\ny\n", "x\nA\nB\ny\n", 0)
	want := "2,- -a\n3,- -b\n-,2 +A\n-,3 +B\n"
	if got := render(d); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestComputeIdenticalIsEmpty(t *testing.T) {
	d := Compute("a\nb\n", "a\nb", 3)
	if !d.Empty() || len(d.Lines) != 0 {
		t.Fatalf("identical texts gave %+v", d)
	}
}

func TestComputeHugeRewriteFallsBackToBlocks(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < 2500; i++ {
		fmt.Fprintf(&a, "old %d\n", i)
		fmt.Fprintf(&b, "new %d\n", i)
	}
	d := Compute(a.String(), b.String(), 0)
	if d.Added != 2500 || d.Removed != 2500 {
		t.Fatalf("stats = +%d -%d", d.Added, d.Removed)
	}
	if d.Lines[0].Kind != Del || d.Lines[2500].Kind != Add {
		t.Fatalf("expected a block delete then a block add")
	}
}

func TestFragmentClearsLineNumbers(t *testing.T) {
	d := Fragment("a\nb\n", "a\nc\n")
	for _, l := range d.Lines {
		if l.OldNo != 0 || l.NewNo != 0 {
			t.Fatalf("fragment kept a line number: %+v", l)
		}
	}
	if len(d.Lines) != 3 {
		t.Fatalf("fragment should keep every line, got %d", len(d.Lines))
	}
}
