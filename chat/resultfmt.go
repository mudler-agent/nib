package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mudler/nib/theme"
)

// toolResultFormatters maps a tool name to a formatter over its decoded result.
// Mirrors toolFormatters, which covers arguments. A tool absent here degrades to
// flattened key/value rows — the same fallback argRows already gives arguments —
// and a result that is not JSON at all passes through as text.
//
// read, bash, bash_job_output and load_skill are here (not left to fall
// through as "obviously not JSON") because the MCP layer wraps every built-in
// tool's structured output as JSON text (github.com/modelcontextprotocol/
// go-sdk's AddTool marshals the output struct when the handler doesn't set
// Content itself) — so a file's content, a script's stdout, or a skill's
// instructions arrives as one field of a JSON envelope, not raw text. Without
// a formatter here, the row fallback would flatten that field and show only
// its first line (the JSON decoder restores real newlines, so this isn't
// hypothetical), which is a worse regression than the JSON dump this task is
// fixing.
var toolResultFormatters = map[string]func(any) string{
	"grep":            fmtGrepResult,
	"glob":            fmtGlobResult,
	"edit":            fmtEditResult,
	"write":           fmtWriteResult,
	"read":            fmtReadResult,
	"bash":            fmtBashResult,
	"bash_job_output": fmtBashResult, // bgOutputResult has the same stdout/stderr/exit_code shape
	"load_skill":      fmtLoadSkillResult,
}

// FormatToolResult renders a tool's output for human reading. A tool with a
// formatter gets purpose-built text; a JSON object with no formatter (any MCP
// tool, or a future built-in) degrades to flattened key/value rows — the same
// fallback FormatToolCall already gives arguments; a result that is not JSON
// at all passes through unchanged.
func FormatToolResult(name, result string) string {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return ""
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return result // not JSON: it is already text
	}
	if f, ok := toolResultFormatters[name]; ok {
		if out := f(decoded); out != "" {
			return out
		}
	}
	if m, ok := decoded.(map[string]any); ok {
		return joinRows(argRows(m))
	}
	return result
}

// fmtGrepResult renders a grep result as one match per line. The real shape
// is {"matches":["path:line:content", …]}; a {"path":..,"line":..} object per
// match is also accepted so a future/legacy shape degrades to text as well.
func fmtGrepResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	matches, ok := m["matches"].([]any)
	if !ok {
		return ""
	}
	rows := make([]string, 0, len(matches))
	for _, raw := range matches {
		switch e := raw.(type) {
		case string:
			if e != "" {
				rows = append(rows, e)
			}
		case map[string]any:
			row := argStr(e, "path")
			if row == "" {
				continue
			}
			if l := argStr(e, "line"); l != "" {
				row += ":" + l
			}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return ""
	}
	return strings.Join(rows, "\n")
}

// fmtGlobResult renders a glob result as one path per line. The real shape is
// {"files":[...]}; the older {"paths":[...]} shape is also accepted.
func fmtGlobResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	paths, ok := m["files"].([]any)
	if !ok {
		paths, ok = m["paths"].([]any)
	}
	if !ok {
		return ""
	}
	rows := make([]string, 0, len(paths))
	for _, p := range paths {
		if s, ok := p.(string); ok && s != "" {
			rows = append(rows, s)
		}
	}
	if len(rows) == 0 {
		return ""
	}
	return strings.Join(rows, "\n")
}

// fmtEditResult summarizes a successful edit as a replacement count. A path
// isn't part of the real edit result (the tool call above already named it),
// so it's included only when present. A failed edit (no "replacements") has
// nothing tool-specific to say, so it returns "" and falls through to rows,
// which surface the "error" field.
func fmtEditResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	n := argStr(m, "replacements")
	if n == "" {
		return ""
	}
	noun := "replacements"
	if n == "1" {
		noun = "replacement"
	}
	if path := argStr(m, "path"); path != "" {
		return "edited " + path + " " + theme.Sep + " " + n + " " + noun
	}
	return "edited " + theme.Sep + " " + n + " " + noun
}

// fmtWriteResult summarizes a write. The real write result carries only
// success/error — nothing beyond what the row fallback already shows — so
// this only fires for a path/bytes shape (kept for forward compatibility);
// otherwise it returns "" and lets rows render "success  true" or the error.
func fmtWriteResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	path := argStr(m, "path")
	if path == "" {
		return ""
	}
	if b := argStr(m, "bytes"); b != "" {
		return "wrote " + path + " " + theme.Sep + " " + b + " bytes"
	}
	return "wrote " + path
}

// fmtReadResult renders the read tool's file content, dropping the
// total_lines/success envelope around it.
func fmtReadResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	content, ok := m["content"].(string)
	if !ok {
		return ""
	}
	return content
}

// fmtLoadSkillResult renders the load_skill tool's instructions body,
// dropping the name/found/error envelope around it. A not-found result has
// no instructions, so it returns "" and falls through to rows, surfacing the
// "error" field instead.
func fmtLoadSkillResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	instructions, ok := m["instructions"].(string)
	if !ok || instructions == "" {
		return ""
	}
	return instructions
}

// fmtBashResult renders a bash result as its stdout followed by its stderr,
// dropping the script/exit_code/success envelope around them. An empty
// result with a nonzero exit renders a short status instead of nothing, so a
// silent failure isn't indistinguishable from a silent success.
func fmtBashResult(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	stdout, hasOut := m["stdout"].(string)
	stderr, hasErr := m["stderr"].(string)
	if !hasOut && !hasErr {
		return ""
	}
	var parts []string
	if s := strings.TrimRight(stdout, "\n"); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimRight(stderr, "\n"); s != "" {
		parts = append(parts, s)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	if succ, isBool := m["success"].(bool); isBool && !succ {
		ec, _ := m["exit_code"].(float64)
		return fmt.Sprintf(theme.ToolResultNoOutput, int(ec))
	}
	return ""
}
