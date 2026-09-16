package tui

import (
	"fmt"
	"strings"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

const (
	// agentThreadInlineCap bounds how many sub-agent tool lines render inline in
	// the transcript thread before older ones collapse to a "+N earlier" note.
	agentThreadInlineCap = 8
	// compactTaskWidth bounds the sub-agent task shown in the transcript header.
	compactTaskWidth = 72
)

// compactTask returns the first line of s, trimmed and ellipsized to maxRunes
// (the ellipsis counts toward maxRunes). Returns "" for blank input.
func compactTask(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = strings.TrimSpace(s[:nl])
	}
	r := []rune(s)
	if maxRunes > 0 && len(r) > maxRunes {
		if maxRunes == 1 {
			return "…"
		}
		return string(r[:maxRunes-1]) + "…"
	}
	return s
}

// capThreadLines returns the lines to render for one sub-agent thread run: when
// len(lines) exceeds limit, a leading "… +N earlier" marker followed by the
// last `limit` lines; otherwise lines unchanged.
func capThreadLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	hidden := len(lines) - limit
	out := make([]string, 0, limit+1)
	out = append(out, fmt.Sprintf("… +%d earlier", hidden))
	out = append(out, lines[len(lines)-limit:]...)
	return out
}

// agentTranscriptLine renders a durable one-line transcript marker for a
// sub-agent lifecycle event, or "" for statuses that should not be logged.
func agentTranscriptLine(ev chat.AgentEvent) string {
	typ := ev.Type
	if typ == "" {
		typ = "agent"
	}
	switch ev.Status {
	case chat.AgentStatusRunning:
		if t := compactTask(ev.Task, compactTaskWidth); t != "" {
			return fmt.Sprintf("sub-agent %s started: %s", typ, t)
		}
		return fmt.Sprintf("sub-agent %s started", typ)
	case chat.AgentStatusCompleted:
		return fmt.Sprintf("sub-agent %s finished%s", typ, ev.StatsSuffix())
	case chat.AgentStatusFailed:
		if ev.Err != nil {
			return fmt.Sprintf("sub-agent %s failed: %v", typ, ev.Err)
		}
		return fmt.Sprintf("sub-agent %s failed", typ)
	default:
		return ""
	}
}

// agentJob is the UI view of a sub-agent for the jobs footer.
type agentJob struct {
	ID     string
	Type   string
	Task   string
	Status chat.AgentStatus
}

// jobsFooterRow returns the plain {Glyph, Text, Kind} data for the jobs
// footer (no glyph — the jobs line has never carried one) and whether there
// is one to show. The presenter styles it (render.FooterJobs gets the
// original theme.Meta + width-fill treatment — see inline.Footer).
func jobsFooterRow(jobs []agentJob) (render.FooterRow, bool) {
	if len(jobs) == 0 {
		return render.FooterRow{}, false
	}
	var running, done, failed int
	for _, j := range jobs {
		switch j.Status {
		case chat.AgentStatusRunning:
			running++
		case chat.AgentStatusCompleted:
			done++
		case chat.AgentStatusFailed:
			failed++
		}
	}
	parts := []string{fmt.Sprintf("jobs: %d running", running)}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", done))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	parts = append(parts, "(ctrl+b background · ctrl+o logs)")
	return render.FooterRow{Text: strings.Join(parts, "  ·  "), Kind: render.FooterJobs}, true
}

// toolApprovalLabel builds the tool-approval header, labeling sub-agent calls.
func toolApprovalLabel(req chat.ToolCallRequest) string {
	if req.AgentID != "" {
		return fmt.Sprintf("%s %s · %s wants to run", theme.SubAgent, render.ShortID(req.AgentID), req.Name)
	}
	return req.Name + " wants to run"
}

// approvalRows builds a render.Dialog's Rows for a tool-approval prompt: the
// structured argument card when chat.ToolArgRows recognizes the tool (one row
// per field), or a single fallback row carrying the raw formatted call
// otherwise — reported via the second return value (render.Dialog's
// RowsUnstructured) rather than an empty-key convention, since chat.ToolArgRows
// keys come from JSON object keys and a tool emitting a literal "" key would
// otherwise collide with that convention.
func approvalRows(req chat.ToolCallRequest) (rows [][2]string, unstructured bool) {
	if rows, ok := chat.ToolArgRows(req.Name, req.Arguments); ok {
		out := make([][2]string, len(rows))
		for i, r := range rows {
			out[i] = [2]string{r.Key, r.ValueDisplay()}
		}
		return out, false
	}
	return [][2]string{{"", chat.FormatToolCall(req.Name, req.Arguments)}}, true
}

// firstRunningJobID returns the id of the first running job, or "".
func (m Model) firstRunningJobID() string {
	for _, j := range m.jobs {
		if j.Status == chat.AgentStatusRunning {
			return j.ID
		}
	}
	return ""
}

// applyAgentEvent upserts a job by ID and refreshes status.
func (m *Model) applyAgentEvent(ev chat.AgentEvent) {
	for i := range m.jobs {
		if m.jobs[i].ID == ev.ID {
			m.jobs[i].Status = ev.Status
			if ev.Type != "" {
				m.jobs[i].Type = ev.Type
			}
			return
		}
	}
	m.jobs = append(m.jobs, agentJob{ID: ev.ID, Type: ev.Type, Task: ev.Task, Status: ev.Status})
}

// jobByID returns the tracked sub-agent job with the given id.
func (m Model) jobByID(id string) (agentJob, bool) {
	for _, j := range m.jobs {
		if j.ID == id {
			return j, true
		}
	}
	return agentJob{}, false
}
