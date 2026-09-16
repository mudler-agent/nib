package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoRawColorLiteralsInRender mirrors the guard in the tui package. The
// original uses os.ReadDir("."), which does not descend into subdirectories —
// so without this the rule would silently stop applying to the presenters.
func TestNoRawColorLiteralsInRender(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), "lipgloss.Color(") {
			t.Errorf("%s contains a raw lipgloss.Color( literal; colors must come from the theme package", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
