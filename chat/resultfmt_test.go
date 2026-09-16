package chat

import (
	"strings"
	"testing"
)

func TestFormatToolResultKnownTools(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		result  string
		want    string
		notWant string
	}{
		{
			name:    "grep renders a file list, not JSON",
			tool:    "grep",
			result:  `{"matches":["a.go:3:foo","b.go:9:bar"],"count":2,"success":true}`,
			want:    "a.go:3:foo",
			notWant: `"matches"`,
		},
		{
			name:    "grep tolerates the legacy path/line object shape too",
			tool:    "grep",
			result:  `{"matches":[{"path":"a.go","line":3},{"path":"b.go","line":9}]}`,
			want:    "a.go:3",
			notWant: `"path"`,
		},
		{
			name:    "glob renders paths",
			tool:    "glob",
			result:  `{"files":["x/y.go","z.go"],"count":2,"success":true}`,
			want:    "x/y.go",
			notWant: `"files"`,
		},
		{
			name:    "glob tolerates the legacy paths-key shape too",
			tool:    "glob",
			result:  `{"paths":["x/y.go","z.go"]}`,
			want:    "x/y.go",
			notWant: `"paths"`,
		},
		{
			name:   "read renders the file content, not the JSON envelope",
			tool:   "read",
			result: `{"content":"   1| package main","total_lines":1,"success":true}`,
			want:   "   1| package main",
		},
		{
			name:    "read passes non-JSON content through untouched",
			tool:    "read",
			result:  "package main\n\nfunc main() {}",
			want:    "package main",
			notWant: "total_lines",
		},
		{
			name:    "bash renders stdout, not the JSON envelope",
			tool:    "bash",
			result:  `{"script":"echo hi","stdout":"hi\n","stderr":"","exit_code":0,"success":true}`,
			want:    "hi",
			notWant: `"stdout"`,
		},
		{
			name:   "edit summarizes the replacement count",
			tool:   "edit",
			result: `{"replacements":2,"success":true}`,
			want:   "2",
		},
		{
			name:   "write falls back to rows when there's nothing tool-specific to say",
			tool:   "write",
			result: `{"success":true}`,
			want:   "success",
		},
		{
			name:    "bash_job_output renders stdout like bash, not the JSON envelope",
			tool:    "bash_job_output",
			result:  `{"job_id":"j1","status":"completed","done":true,"exit_code":0,"stdout":"line one\nline two\n","stderr":""}`,
			want:    "line one\nline two",
			notWant: `"stdout"`,
		},
		{
			name:    "load_skill renders the instructions body, not the JSON envelope",
			tool:    "load_skill",
			result:  `{"name":"golang-testing","instructions":"# Golang Testing\n\nUse table-driven tests.","found":true}`,
			want:    "# Golang Testing\n\nUse table-driven tests.",
			notWant: `"instructions"`,
		},
		{
			name:   "load_skill not-found falls back to rows, surfacing the error",
			tool:   "load_skill",
			result: `{"name":"nope","instructions":"","found":false,"error":"skill not found"}`,
			want:   "skill not found",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FormatToolResult(c.tool, c.result)
			if !strings.Contains(got, c.want) {
				t.Errorf("FormatToolResult(%q) = %q, want it to contain %q", c.tool, got, c.want)
			}
			if c.notWant != "" && strings.Contains(got, c.notWant) {
				t.Errorf("FormatToolResult(%q) = %q, should not contain raw JSON %q", c.tool, got, c.notWant)
			}
		})
	}
}

// TestFormatToolResultUnknownToolDegradesToRows: an MCP tool has no formatter,
// so it must degrade the way arguments already do — flattened key/value rows,
// not a JSON dump.
func TestFormatToolResultUnknownToolDegradesToRows(t *testing.T) {
	got := FormatToolResult("some_mcp_tool", `{"status":"ok","count":3}`)
	if strings.Contains(got, "{") || strings.Contains(got, `"`) {
		t.Errorf("unknown tool result still looks like JSON: %q", got)
	}
	for _, want := range []string{"status", "ok", "count", "3"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatToolResult = %q, want it to contain %q", got, want)
		}
	}
}

func TestFormatToolResultNonJSONPassesThrough(t *testing.T) {
	const raw = "this is not json at all"
	if got := FormatToolResult("whatever", raw); got != raw {
		t.Errorf("FormatToolResult = %q, want the raw text %q", got, raw)
	}
}

// TestFormatToolResultBashSilentFailure: a bash result with no stdout/stderr
// but a nonzero exit code must say so, not render nothing — a silent failure
// must not look identical to a silent success.
func TestFormatToolResultBashSilentFailure(t *testing.T) {
	got := FormatToolResult("bash", `{"script":"exit 2","stdout":"","stderr":"","exit_code":2,"success":false}`)
	if want := "(exit 2, no output)"; got != want {
		t.Errorf("FormatToolResult(bash) = %q, want %q", got, want)
	}
}

// TestFormatToolResultBashSilentSuccess: no stdout/stderr and a zero exit has
// nothing tool-specific to say (same as fmtWriteResult on a bare success), so
// fmtBashResult returns "" and it falls through to rows — still readable
// text, not the JSON dump this task removes, just not a one-line summary.
func TestFormatToolResultBashSilentSuccess(t *testing.T) {
	got := FormatToolResult("bash", `{"script":"true","stdout":"","stderr":"","exit_code":0,"success":true}`)
	if strings.Contains(got, "{") || strings.Contains(got, `"`) {
		t.Errorf("silent success still looks like JSON: %q", got)
	}
	if !strings.Contains(got, "success") {
		t.Errorf("FormatToolResult(bash) = %q, want the success row surfaced", got)
	}
}

// TestFormatToolResultSchemaDriftDegrades: a formatter whose expected shape
// doesn't match (e.g. edit result missing "replacements") must return "" and
// fall through to rows, not panic or render garbage.
func TestFormatToolResultSchemaDriftDegrades(t *testing.T) {
	got := FormatToolResult("edit", `{"success":false,"error":"old string not found in file"}`)
	if strings.Contains(got, "{") {
		t.Errorf("schema drift still looks like JSON: %q", got)
	}
	if !strings.Contains(got, "error") || !strings.Contains(got, "old string not found in file") {
		t.Errorf("FormatToolResult = %q, want the error surfaced via rows", got)
	}
}
