package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mudler/nib/theme"
)

func TestModelPickerOpenAndCloseResetState(t *testing.T) {
	p := modelPicker{active: true, all: []string{"old"}, query: "old", selected: 3, offset: 2}
	p.open(17)
	if !p.active || !p.loading || p.requestID != 17 {
		t.Fatalf("open state = %+v", p)
	}
	if len(p.all) != 0 || len(p.matches) != 0 || p.query != "" || p.selected != 0 || p.offset != 0 {
		t.Fatalf("open did not reset prior state: %+v", p)
	}

	p.close()
	if !reflect.DeepEqual(p, modelPicker{}) {
		t.Fatalf("close state = %+v, want zero value", p)
	}
}

func TestModelPickerSetModelsPreservesOrderAndSelectsCurrent(t *testing.T) {
	p := modelPicker{active: true, loading: true}
	p.setModels([]string{"zeta", "Alpha", "beta", "alpine"}, "beta", 2)
	if p.loading {
		t.Fatal("picker remained loading after models arrived")
	}
	if want := []string{"zeta", "Alpha", "beta", "alpine"}; !reflect.DeepEqual(p.matches, want) {
		t.Fatalf("matches = %v, want endpoint order %v", p.matches, want)
	}
	if p.selected != 2 || p.offset != 1 {
		t.Fatalf("selection = %d, offset = %d; want current at 2 and visible offset 1", p.selected, p.offset)
	}
	if got, ok := p.choice(); !ok || got != "beta" {
		t.Fatalf("choice = %q, %v; want beta, true", got, ok)
	}
}

func TestModelPickerSetModelsFallsBackToFirstResult(t *testing.T) {
	p := modelPicker{active: true, loading: true}
	p.setModels([]string{"first", "second"}, "missing", 4)
	if p.selected != 0 || p.offset != 0 {
		t.Fatalf("selection = %d, offset = %d; want first result", p.selected, p.offset)
	}
}

func TestModelPickerQueryFiltersCaseInsensitiveSubstringInEndpointOrder(t *testing.T) {
	p := modelPicker{all: []string{"Zulu", "ALPHA-large", "beta", "small-alpha"}, selected: 3, offset: 2}
	p.appendQuery("aLpHa", 2)
	if want := []string{"ALPHA-large", "small-alpha"}; !reflect.DeepEqual(p.matches, want) {
		t.Fatalf("matches = %v, want %v", p.matches, want)
	}
	if p.selected != 0 || p.offset != 0 {
		t.Fatalf("query change selection = %d, offset = %d; want reset", p.selected, p.offset)
	}
}

func TestModelPickerBackspaceRemovesOneRuneAndRefilters(t *testing.T) {
	p := modelPicker{all: []string{"café", "cafeteria", "tea"}}
	p.appendQuery("fé", 3)
	p.backspace(3)
	if p.query != "f" {
		t.Fatalf("query = %q, want rune-safe removal to f", p.query)
	}
	if want := []string{"café", "cafeteria"}; !reflect.DeepEqual(p.matches, want) {
		t.Fatalf("matches = %v, want %v", p.matches, want)
	}
	p.backspace(3)
	p.backspace(3)
	if p.query != "" {
		t.Fatalf("backspace past empty query = %q", p.query)
	}
}

func TestModelPickerMoveIsBoundedAndScrollsSelectionIntoView(t *testing.T) {
	p := modelPicker{matches: []string{"a", "b", "c", "d", "e"}}
	p.move(-10, 2)
	if p.selected != 0 || p.offset != 0 {
		t.Fatalf("move above start = selected %d offset %d", p.selected, p.offset)
	}
	p.move(3, 2)
	if p.selected != 3 || p.offset != 2 {
		t.Fatalf("move to fourth = selected %d offset %d; want 3, 2", p.selected, p.offset)
	}
	p.move(20, 2)
	if p.selected != 4 || p.offset != 3 {
		t.Fatalf("move beyond end = selected %d offset %d; want 4, 3", p.selected, p.offset)
	}
	p.move(-2, 2)
	if p.selected != 2 || p.offset != 2 {
		t.Fatalf("move within window = selected %d offset %d; want 2, 2", p.selected, p.offset)
	}
}

func TestModelPickerChoiceRejectsEmptyOrInvalidSelection(t *testing.T) {
	for _, p := range []modelPicker{{}, {matches: []string{"one"}, selected: -1}, {matches: []string{"one"}, selected: 1}} {
		if got, ok := p.choice(); ok || got != "" {
			t.Fatalf("choice for %+v = %q, %v; want empty, false", p, got, ok)
		}
	}
}

func TestModelPickerVisibleRowsAlwaysLeavesOneResult(t *testing.T) {
	if got := modelPickerVisibleRows(2); got != 1 {
		t.Fatalf("visible rows at height 2 = %d, want 1", got)
	}
	if got := modelPickerVisibleRows(8); got != 5 {
		t.Fatalf("visible rows at height 8 = %d, want 5", got)
	}
}

func TestModelPickerRenderStatesAndAffordances(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		got := renderModelPicker(modelPicker{active: true, loading: true}, "", 80, 8)
		if !strings.Contains(got, theme.ModelPickerLoading) {
			t.Fatalf("render = %q, want loading copy", got)
		}
	})
	t.Run("empty endpoint", func(t *testing.T) {
		got := renderModelPicker(modelPicker{active: true}, "", 80, 8)
		if !strings.Contains(got, theme.ModelPickerEmpty) {
			t.Fatalf("render = %q, want empty-endpoint copy", got)
		}
	})
	t.Run("no search matches", func(t *testing.T) {
		p := modelPicker{active: true, all: []string{"one"}, query: "xyz"}
		got := renderModelPicker(p, "", 80, 8)
		if !strings.Contains(got, theme.ModelPickerNoMatches) {
			t.Fatalf("render = %q, want no-match copy", got)
		}
	})
	t.Run("query current highlight and key hint", func(t *testing.T) {
		p := modelPicker{active: true, all: []string{"one", "two", "three"}, matches: []string{"one", "two", "three"}, query: "tw", selected: 1}
		got := renderModelPicker(p, "two", 80, 8)
		for _, want := range []string{theme.ModelPickerSearchLabel, "tw", theme.PromptGlyph + " two", "(current)", theme.ModelPickerKeyHint} {
			if !strings.Contains(got, want) {
				t.Fatalf("render = %q, want %q", got, want)
			}
		}
	})
	t.Run("height and width bound visible results", func(t *testing.T) {
		p := modelPicker{active: true, all: []string{"a-very-long-model-name", "two", "three"}, matches: []string{"a-very-long-model-name", "two", "three"}, selected: 0}
		got := renderModelPicker(p, "", 12, 4)
		if strings.Contains(got, "two") || strings.Contains(got, "three") {
			t.Fatalf("render includes results outside one-row window: %q", got)
		}
		for _, line := range strings.Split(got, "\n") {
			if lipgloss.Width(line) > 12 {
				t.Fatalf("line %q has width %d, want at most 12", line, lipgloss.Width(line))
			}
		}
	})
}
