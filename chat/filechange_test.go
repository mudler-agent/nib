package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewFileChangeWriteNewFile(t *testing.T) {
	dir := t.TempDir()
	c := PreviewFileChange(dir, "write", `{"path":"new.go","content":"package x\n"}`)
	if c == nil || !c.Created || c.Before != "" || c.After != "package x\n" {
		t.Fatalf("got %+v, want a created file with the new content", c)
	}
}

func TestPreviewFileChangeWriteOverwriteDiffsAgainstDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	c := PreviewFileChange(dir, "write", `{"path":"a.txt","content":"one\nTWO\n"}`)
	if c == nil || c.Created || c.Before != "one\ntwo\n" {
		t.Fatalf("got %+v, want the current file as Before", c)
	}
	if d := c.Diff(); d.Added != 1 || d.Removed != 1 {
		t.Fatalf("diff = +%d -%d, want +1 -1", d.Added, d.Removed)
	}
}

func TestPreviewFileChangeEditAppliesToFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "m.go", "a\nb\nc\nb\n")
	c := PreviewFileChange(dir, "edit", `{"path":"m.go","old":"b","new":"B"}`)
	if c == nil || c.Fragment || c.After != "a\nB\nc\nb\n" {
		t.Fatalf("got %+v, want the first occurrence replaced", c)
	}
	c = PreviewFileChange(dir, "edit", `{"path":"m.go","old":"b","new":"B","all":true}`)
	if c == nil || c.After != "a\nB\nc\nB\n" {
		t.Fatalf("got %+v, want every occurrence replaced", c)
	}
	// Line numbers come from the real file.
	if d := c.Diff(); d.Lines[0].OldNo == 0 {
		t.Fatalf("whole-file diff lost its line numbers: %+v", d.Lines[0])
	}
}

func TestPreviewFileChangeEditFallsBackToFragment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "m.go", "a\n")
	for _, args := range []string{
		`{"path":"m.go","old":"missing","new":"x"}`, // old not in file
		`{"path":"nope.go","old":"a","new":"x"}`,    // file absent
	} {
		c := PreviewFileChange(dir, "edit", args)
		if c == nil || !c.Fragment {
			t.Fatalf("%s: got %+v, want a fragment of old/new", args, c)
		}
	}
}

func TestPreviewFileChangeSkipsBinaryAndOtherTools(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bin", "a\x00b")
	if c := PreviewFileChange(dir, "write", `{"path":"bin","content":"x"}`); c != nil {
		t.Fatalf("binary file got a change: %+v", c)
	}
	if c := PreviewFileChange(dir, "bash", `{"script":"ls"}`); c != nil {
		t.Fatalf("bash got a change: %+v", c)
	}
}

func TestFileChangeSettleReadsDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "actual\n")
	c := &FileChange{Path: "a.txt", Before: "old\n", After: "predicted\n"}
	c.settle(dir)
	if c.After != "actual\n" {
		t.Fatalf("After = %q, want what is on disk", c.After)
	}
}

func TestChangeTrackerTakeRemoves(t *testing.T) {
	var tr changeTracker
	c := &FileChange{Path: "x"}
	tr.put(changeKey("", "write", "{}"), c)
	if got := tr.take(changeKey("", "write", "{}")); got != c {
		t.Fatalf("take = %v, want the stored change", got)
	}
	if got := tr.take(changeKey("", "write", "{}")); got != nil {
		t.Fatalf("second take = %v, want nil", got)
	}
}

func TestArgsFileChange(t *testing.T) {
	if c := ArgsFileChange("edit", `{"path":"m.go","old":"a","new":"b"}`); c == nil || !c.Fragment {
		t.Fatalf("edit: got %+v, want a fragment", c)
	}
	if c := ArgsFileChange("write", `{"path":"m.go","content":"x\n"}`); c == nil || c.After != "x\n" {
		t.Fatalf("write: got %+v", c)
	}
	if c := ArgsFileChange("read", `{"path":"m.go"}`); c != nil {
		t.Fatalf("read: got %+v, want nil", c)
	}
}

func TestToolOutcome(t *testing.T) {
	cases := []struct {
		result     string
		failed     bool
		wantDetail string
	}{
		{`{"success":true}`, false, ""},
		{`{"success":false,"error":"no such file"}`, true, ""},
		{`{"error":"boom"}`, true, ""},
		{`{"stdout":"","exit_code":2,"success":false}`, true, "exit 2"},
		{`{"stdout":"ok","exit_code":0,"success":true}`, false, ""},
		{`plain text from an MCP tool`, false, ""},
	}
	for _, c := range cases {
		failed, detail := ToolOutcome(c.result)
		if failed != c.failed || detail != c.wantDetail {
			t.Errorf("ToolOutcome(%s) = %v %q, want %v %q", c.result, failed, detail, c.failed, c.wantDetail)
		}
	}
}

func TestPreviewResultKeepsFirstLineIndent(t *testing.T) {
	got := PreviewResult("read", `{"content":"   1| package main\n   2| \n   3| import \"fmt\"","success":true}`, 0)
	if want := "   1| package main\n   2| \n   3| import \"fmt\""; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
