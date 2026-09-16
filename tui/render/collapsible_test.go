package render

import (
	"reflect"
	"testing"
)

// TestCollapsedShowsTheLatestLines is the important one: a head-anchored box
// would freeze on the trace's opening words and never move, which reads as a
// hang. Tailing makes the collapsed box its own progress indicator.
func TestCollapsedShowsTheLatestLines(t *testing.T) {
	b := CollapsibleBox{
		Lines:     []string{"one", "two", "three", "four", "five"},
		MaxLines:  2,
		Collapsed: true,
	}
	if got, want := b.Visible(), []string{"four", "five"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Visible() = %v, want %v", got, want)
	}
	if got, want := b.Hidden(), 3; got != want {
		t.Errorf("Hidden() = %d, want %d", got, want)
	}
}

func TestExpandedShowsEverything(t *testing.T) {
	b := CollapsibleBox{Lines: []string{"one", "two", "three"}, MaxLines: 2, Collapsed: false}
	if got := len(b.Visible()); got != 3 {
		t.Errorf("expanded Visible() length = %d, want 3", got)
	}
	if got := b.Hidden(); got != 0 {
		t.Errorf("expanded Hidden() = %d, want 0", got)
	}
}

// TestNegativeMaxLinesIsNotTruncated: a negative MaxLines is as meaningless
// as zero and must not panic or slice out of range.
func TestNegativeMaxLinesIsNotTruncated(t *testing.T) {
	b := CollapsibleBox{Lines: []string{"one", "two", "three"}, MaxLines: -1, Collapsed: true}
	if got, want := b.Visible(), []string{"one", "two", "three"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Visible() = %v, want %v", got, want)
	}
	if got := b.Hidden(); got != 0 {
		t.Errorf("Hidden() = %d, want 0", got)
	}
}

func TestShorterThanMaxIsNotTruncated(t *testing.T) {
	b := CollapsibleBox{Lines: []string{"one"}, MaxLines: 5, Collapsed: true}
	if got, want := b.Visible(), []string{"one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Visible() = %v, want %v", got, want)
	}
	if got := b.Hidden(); got != 0 {
		t.Errorf("Hidden() = %d, want 0", got)
	}
}
