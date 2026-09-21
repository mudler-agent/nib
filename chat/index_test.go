package chat

import (
	"strings"
	"testing"
)

// The description once said "currently .go" after extractors for sixteen more
// languages landed, so a model working in Python read it and skipped the tool.
// It must name what codeindex actually supports.
func TestIndexToolDescriptionListsSupportedLanguages(t *testing.T) {
	desc := indexToolDefinition(func(p string) string { return p }).Tool().Function.Description
	for _, ext := range []string{".go", ".py", ".ts", ".rs", ".java"} {
		if !strings.Contains(desc, ext) {
			t.Fatalf("description does not list %s:\n%s", ext, desc)
		}
	}
	if strings.Contains(desc, "currently .go") {
		t.Fatalf("description still claims Go-only support:\n%s", desc)
	}
}

// "Use it FIRST on a source file you have not seen" made the model index every
// file instead of reading it. The description must point index at large files
// only.
func TestIndexToolDescriptionDoesNotPutIndexFirst(t *testing.T) {
	desc := indexToolDefinition(func(p string) string { return p }).Tool().Function.Description
	if strings.Contains(desc, "FIRST") {
		t.Fatalf("description still makes index the first step:\n%s", desc)
	}
	if !strings.Contains(desc, "large source file") {
		t.Fatalf("description does not scope index to large files:\n%s", desc)
	}
}
