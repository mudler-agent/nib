package tui

import (
	"strconv"
	"strings"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// buildAskDialog turns a pending ask_user question into a render.Dialog: the
// question as Title, one DialogOption per visible choice, and the live
// selection state from list — Selected (single-select) or Checked
// (multi-select) — so a Presenter can mark which row is highlighted or
// ticked. This replaces the old renderAsk, which built a numbered/checkbox
// block as plain text answered by typing an index through the normal
// composer (parseAskAnswer, unchanged below) — the "weird UX" this task
// exists to fix: ask_user now uses the same key-driven-modal idiom as tool
// approval instead of looking like plain text.
//
// list is nil, or has no Items, for a free-text-only question (no Options on
// the underlying chat.AskRequest) — the Dialog then carries Title alone, same
// degrade every Presenter already gives an empty-Options Dialog. list.Window()
// decides which Options make it into the Dialog: Dialog itself carries no
// MaxVisible, so the windowing has to happen here, at the one call site that
// holds both the full item list and the visible-rows budget.
//
// blockedByApproval is true when a tool approval is ALSO pending
// (Model.awaitingApproval) — cogito can propagate the tool-call callback into
// a background sub-agent while the root agent is independently blocked on
// ask_user (see currentDialogs in tui/model.go), and the approval's own
// key-driven choice mode swallows every keypress but its own (arrows, space,
// typed text included) until it resolves. Without this, the ask dialog shows
// arrows and a "type your own answer" hint that silently do nothing — exactly
// the confusing dead end this whole task exists to remove. When true, it
// overrides the normal per-mode hint with theme.AskBlockedByApproval instead
// of a hint describing affordances that don't currently work.
func buildAskDialog(req chat.AskRequest, list *render.SelectList, blockedByApproval bool) render.Dialog {
	d := render.Dialog{Kind: render.DialogAsk, Title: req.Question}
	switch {
	case list == nil || len(list.Items) == 0:
		d.Hint = theme.AskHintFreeText
	default:
		start, end := list.Window()
		for i := start; i < end; i++ {
			d.Options = append(d.Options, render.DialogOption{Text: list.Items[i]})
		}
		d.Selected = list.Selected - start
		if list.MultiSelect {
			d.Checked = append([]bool(nil), list.Checked[start:end]...)
			d.Hint = theme.AskHintMultiSelect
		} else {
			d.Hint = theme.AskHintSingleSelect
		}
	}
	if blockedByApproval {
		d.Hint = theme.AskBlockedByApproval
	}
	return d
}

// parseAskAnswer maps a typed answer onto req.Options. For single-select a lone
// 1-based index returns that option. For multi-select a list of indices (comma-
// or space-separated, e.g. "1,3" or "1 3") returns the chosen options joined by
// ", ". Anything that doesn't parse cleanly as indices is returned verbatim as a
// free-form answer.
func parseAskAnswer(input string, req chat.AskRequest) string {
	trimmed := strings.TrimSpace(input)
	if len(req.Options) == 0 {
		return input
	}

	if req.MultiSelect {
		fields := strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		})
		var chosen []string
		for _, f := range fields {
			n, err := strconv.Atoi(f)
			if err != nil || n < 1 || n > len(req.Options) {
				return input // not a clean index list → treat as free text
			}
			chosen = append(chosen, req.Options[n-1])
		}
		if len(chosen) > 0 {
			return strings.Join(chosen, ", ")
		}
		return input
	}

	if n, err := strconv.Atoi(trimmed); err == nil && n >= 1 && n <= len(req.Options) {
		return req.Options[n-1]
	}
	return input
}
