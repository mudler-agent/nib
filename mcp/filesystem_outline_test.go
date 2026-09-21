package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bigGoFile writes a Go file longer than outlineMinLines and returns its path.
func bigGoFile(t *testing.T, name string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("package big\n\n")
	for i := 0; b.Len() == 0 || strings.Count(b.String(), "\n") <= outlineMinLines; i++ {
		fmt.Fprintf(&b, "func F%d() int {\n\treturn %d\n}\n\n", i, i)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A whole read of a large source file costs tens of thousands of tokens,
// usually to find one function. It returns the outline instead, with the line
// ranges the model needs for a ranged read.
func TestReadOfLargeSourceFileReturnsOutline(t *testing.T) {
	p := bigGoFile(t, "big.go")
	_, out, err := readFile(context.Background(), &mcp.CallToolRequest{}, readFileInput{Path: p})
	if err != nil || !out.Success {
		t.Fatalf("read failed: %v %s", err, out.Error)
	}
	if !out.Outline {
		t.Fatalf("outline = false for a %d-line Go file", out.TotalLines)
	}
	for _, want := range []string{"Function: F0() int [3-5]", "offset and limit", fmt.Sprint(out.TotalLines)} {
		if !strings.Contains(out.Content, want) {
			t.Fatalf("outline content missing %q:\n%.400s", want, out.Content)
		}
	}
	if strings.Contains(out.Content, "return 0") {
		t.Fatalf("outline carries file source:\n%.400s", out.Content)
	}
}

// An explicit range is the model asking for lines, and it gets them.
func TestRangedReadOfLargeSourceFileReturnsLines(t *testing.T) {
	p := bigGoFile(t, "big.go")
	_, out, _ := readFile(context.Background(), &mcp.CallToolRequest{}, readFileInput{Path: p, Offset: 2, Limit: 3})
	if out.Outline || !strings.Contains(out.Content, "return 0") {
		t.Fatalf("ranged read did not return lines: outline=%v\n%s", out.Outline, out.Content)
	}
}

// With no extractor there is no outline to offer, so the read behaves as before.
func TestReadOfLargeUnsupportedFileReturnsLines(t *testing.T) {
	p := bigGoFile(t, "big.txt")
	_, out, _ := readFile(context.Background(), &mcp.CallToolRequest{}, readFileInput{Path: p})
	if out.Outline || !strings.Contains(out.Content, "return 0") {
		t.Fatalf("unsupported file did not return lines: outline=%v", out.Outline)
	}
}

// An outline is not the file's contents, so it must not unlock edit: edit
// matches exact text the model has not seen yet.
func TestOutlineReadDoesNotMarkFileSeen(t *testing.T) {
	p := bigGoFile(t, "big.go")
	fs := newFileSystem(filepath.Dir(p))
	if _, out, _ := fs.read(context.Background(), &mcp.CallToolRequest{}, readFileInput{Path: p}); !out.Outline {
		t.Fatal("expected an outline read")
	}
	if fs.hasSeen(p) {
		t.Fatal("outline read marked the file seen")
	}
	fs.read(context.Background(), &mcp.CallToolRequest{}, readFileInput{Path: p, Limit: 10})
	if !fs.hasSeen(p) {
		t.Fatal("ranged read did not mark the file seen")
	}
}
