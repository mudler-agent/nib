package slash

import "testing"

func TestResolveAbout(t *testing.T) {
	a := Resolve("/about", nil, nil, nil)
	if a.Kind != KindAbout {
		t.Errorf("expected KindAbout, got %v", a.Kind)
	}
}

func TestResolveAboutWithExtraArgs(t *testing.T) {
	// /about ignores any trailing text.
	a := Resolve("/about anything", nil, nil, nil)
	if a.Kind != KindAbout {
		t.Errorf("expected KindAbout, got %v", a.Kind)
	}
}
