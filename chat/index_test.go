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
