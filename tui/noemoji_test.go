package tui

import (
	"testing"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/tui/render"
)

// containsEmoji reports whether s contains a rune in the common emoji ranges.
func containsEmoji(s string) bool {
	for _, r := range s {
		if (r >= 0x1F000 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) {
			return true
		}
	}
	return false
}

// TestNoEmojiInRenderHelpers guards the calm, no-emoji editorial voice: the
// user-facing render helpers must not emit emoji glyphs.
func TestNoEmojiInRenderHelpers(t *testing.T) {
	// The ask_user dialog: question + options, rendered through the presenter
	// (buildAskDialog replaced the old renderAsk plain-text block).
	req := chat.AskRequest{Question: "Pick one", Options: []string{"alpha", "beta"}}
	ask := testPresenter().Dialog(buildAskDialog(req, &render.SelectList{Items: req.Options}, false), 80)
	if containsEmoji(ask) {
		t.Fatalf("ask dialog output contains emoji: %q", ask)
	}

	// Jobs footer (running/done/failed counts). jobsFooterRow is the live path
	// View() calls — renderJobsFooter (the fully-styled string this test used
	// to check) was deleted once it had zero production call sites left.
	row, ok := jobsFooterRow([]agentJob{
		{ID: "a1", Type: "explore", Task: "scan", Status: chat.AgentStatusRunning},
		{ID: "b2", Type: "plan", Task: "draft", Status: chat.AgentStatusCompleted},
		{ID: "c3", Type: "edit", Task: "patch", Status: chat.AgentStatusFailed},
	})
	if !ok {
		t.Fatal("jobsFooterRow should report a row for non-empty jobs")
	}
	if containsEmoji(row.Glyph + row.Text) {
		t.Fatalf("jobsFooterRow output contains emoji: %q", row.Text)
	}

	// Tool-approval labels, both sub-agent and root variants.
	sub := toolApprovalLabel(chat.ToolCallRequest{Name: "echo", AgentID: "a1b2c3d4e5"})
	if containsEmoji(sub) {
		t.Fatalf("toolApprovalLabel (sub-agent) contains emoji: %q", sub)
	}
	root := toolApprovalLabel(chat.ToolCallRequest{Name: "echo"})
	if containsEmoji(root) {
		t.Fatalf("toolApprovalLabel (root) contains emoji: %q", root)
	}

	// Ctrl+O log viewer list.
	lv := newLogsModel()
	lv.showLogs = true
	if out := lv.renderLogsViewer(); containsEmoji(out) {
		t.Fatalf("renderLogsViewer output contains emoji: %q", out)
	}

	// Completion popup (tags, names, descriptions, ghost hint).
	cmds, skills, agents := sampleRegistries()
	var c compState
	c.setRegistries(cmds, skills, agents)
	c.sync("/rev")
	comp := renderCompletion(c, "/rev", 80)
	if containsEmoji(comp) {
		t.Fatalf("renderCompletion output contains emoji: %q", comp)
	}

	// Tool-output pruning notice, both the plural and the singular reading.
	for _, n := range []string{prunedNotice(3, 12400), prunedNotice(1, 900)} {
		if containsEmoji(n) {
			t.Fatalf("prunedNotice output contains emoji: %q", n)
		}
	}

	// Compaction notice. It sat outside this sweep for months and carried an
	// emoji the whole time, which is what a guard listing its subjects one by
	// one costs when a helper is added and nobody remembers to list it.
	if n := compactNotice(47200, 12100); containsEmoji(n) {
		t.Fatalf("compactNotice output contains emoji: %q", n)
	}

	// The sibling notice that package chat writes into the transcript — which
	// this model renders verbatim — is guarded on its own side, by
	// chat.TestCompactionTranscriptNoticeHasNoEmoji. The rule is about what a
	// user sees, and a user sees both; reaching that one from here would mean
	// exporting a chat function that exists only for this test, so each package
	// guards the strings it writes.
}
