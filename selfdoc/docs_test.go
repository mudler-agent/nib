package selfdoc

import (
	"strings"
	"testing"
)

func TestEmbeddedReadme(t *testing.T) {
	got := EmbeddedReadme()
	if len(got) == 0 {
		t.Fatal("EmbeddedReadme() returned empty string")
	}
	if len(got) < 100 {
		t.Fatalf("EmbeddedReadme() suspiciously short: %d bytes", len(got))
	}
}

func TestEmbeddedReadmeNotZeroValue(t *testing.T) {
	if EmbeddedReadme() == "" {
		t.Fatal("embeddedReadme is empty — go:embed failed or README.md is missing")
	}
}

func TestReadmeSections(t *testing.T) {
	sections := ReadmeSections()
	if len(sections) == 0 {
		t.Fatal("ReadmeSections() returned empty slice")
	}
	// The README has these top-level headings.
	want := []string{"Quickstart", "Configuration", "Plugins", "Skills"}
	titles := make(map[string]bool)
	for _, s := range sections {
		titles[s.Title] = true
	}
	for _, w := range want {
		if !titles[w] {
			t.Errorf("ReadmeSections() missing %q (have: %v)", w, sectionTitles(sections))
		}
	}
}

func TestReadmeSectionsHaveBodies(t *testing.T) {
	sections := ReadmeSections()
	for _, s := range sections {
		if len(s.Body) == 0 {
			t.Errorf("section %q has empty body", s.Title)
		}
	}
}

func TestSectionByName(t *testing.T) {
	sec, ok := SectionByName("Quickstart")
	if !ok {
		t.Fatal("SectionByName(\"Quickstart\") not found")
	}
	if !strings.Contains(sec.Body, "go install") && !strings.Contains(sec.Body, "nib") {
		t.Errorf("Quickstart body doesn't look like a quickstart: %q", sec.Body[:min(200, len(sec.Body))])
	}
}

func TestSectionByNameCaseInsensitive(t *testing.T) {
	sec, ok := SectionByName("quickstart")
	if !ok {
		t.Fatal("SectionByName(\"quickstart\") not found (should be case-insensitive)")
	}
	if sec.Title != "Quickstart" {
		t.Errorf("returned title %q, want %q", sec.Title, "Quickstart")
	}
}

func TestSectionByNameNotFound(t *testing.T) {
	_, ok := SectionByName("Nonexistent Section")
	if ok {
		t.Fatal("SectionByName should return false for nonexistent section")
	}
}

func TestTableOfContents(t *testing.T) {
	toc := TableOfContents()
	if len(toc) == 0 {
		t.Fatal("TableOfContents() returned empty string")
	}
	if !strings.Contains(toc, "Quickstart") {
		t.Errorf("TableOfContents() missing Quickstart: %q", toc)
	}
	if !strings.Contains(toc, "Configuration") {
		t.Errorf("TableOfContents() missing Configuration: %q", toc)
	}
	// ToC should be much shorter than the full README.
	if len(toc) >= len(EmbeddedReadme()) {
		t.Errorf("TableOfContents() (%d bytes) should be shorter than full README (%d bytes)",
			len(toc), len(EmbeddedReadme()))
	}
}

func sectionTitles(sections []Section) []string {
	titles := make([]string, len(sections))
	for i, s := range sections {
		titles[i] = s.Title
	}
	return titles
}
