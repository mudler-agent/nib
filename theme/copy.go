package theme

// Microcopy — calm, lowercase, no wizard metaphor, no emoji.
const (
	BrandName = "nib"

	// LabelYouText labels the user's own chat messages (paired with LabelYou,
	// the style, in theme.go).
	LabelYouText = "you"

	HelpDefault      = "enter send · ctrl+y use command · G/end newest · esc exit"
	HelpApproval     = "1 once · 2 always · 3 this turn · 4 this session · n no · e edit · esc deny"
	HelpApprovalEdit = "enter submit · esc cancel"
	ApproveEditHint  = "describe the change · enter submit · esc cancel"

	// YoloOn/YoloOff are the transcript notices the /yolo toggle appends.
	YoloOn  = "yolo on — every tool call is auto-approved"
	YoloOff = "yolo off — tool calls need approval again"

	// NewOutputText is the footer marker shown when the viewport is scrolled up
	// and content has arrived below the fold. Composed with NewOutputGlyph by
	// NewOutputMarker() in theme.go — the glyph is swappable, this text isn't.
	NewOutputText = "new output"

	// The numbered approval menu. Line 2 is dynamic — the TUI composes
	// ApproveAlwaysPrefix + chat.GrantScope(...) + ApproveAlwaysSuffix.
	ApproveOnce         = "[1] run it once"
	ApproveAlwaysPrefix = "[2] always allow "
	ApproveAlwaysSuffix = "  (this session)"
	ApproveTurn         = "[3] yes to everything this turn"
	ApproveSession      = "[4] yes to everything this session"
	ApproveDenyEdit     = "[n] no · [e] edit"

	EmptyTagline = "a calm assistant for your terminal."
	EmptyTryLead = "try:"
	EmptySlash   = "type /  for skills, agents & commands"
	SlashHint    = "/ for skills"
	Starting     = "starting…"

	CLIWelcome = "a calm assistant for your terminal."
	CLIExit    = "ctrl+c or 'exit' to leave · 'help' for commands"

	// CLIHelp is cmd/cli.go's help() output — the CLI's own command list, kept
	// separate from the TUI's slash-completion popup. /yolo works in CLI mode
	// (cmd/cli.go's KindYolo case) and belongs here alongside exit/clear/help.
	CLIHelp = "commands:  exit  ·  clear  ·  help  ·  /yolo"

	// CLINotAvailable is the CLI dispatch loop's catch-all for a resolved
	// slash.Action whose Kind has no explicit case there — a %s format string
	// naming the command. It exists so a Kind with no CLI meaning (no picker,
	// no popup, nothing to wire up) is refused with a clear message instead of
	// silently falling through to KindSend and reaching the model as chat
	// text — see cmd/cli.go's default arm.
	CLINotAvailable = "%s is not available in CLI mode."

	// CLIResumeHint is appended to CLINotAvailable for /resume specifically:
	// unlike /loop or /goal, it has a real non-interactive equivalent already
	// wired up (app.applyResumeFlag), so the refusal can point at it instead
	// of just saying no.
	CLIResumeHint = "restart with `nib --resume` (or `nib --resume <id>`) to load a recorded session."

	// Shown when a CLI approval prompt gets no answer at all. A closed stdin
	// (the piped one-shot idiom) and a cancelled run are both "nobody
	// decided", which is not a yes, so the call is denied.
	CLIDeniedNoInput  = "denied: stdin closed, nobody left to approve this"
	CLIDeniedNoAnswer = "denied: no answer (the run was cancelled)"

	// Shown when --yolo / NIB_YOLO auto-approves every tool call. The header
	// carries the compact badge; the CLI prints the fuller notice at startup.
	YoloBadge  = "yolo"
	YoloNotice = "yolo — auto-approving every tool call (no prompts)"

	// StatusRunning is shown between an approved tool call and its result.
	StatusRunning = "running…"

	// Reasoning box copy. A collapsed box shows the trailing
	// ReasoningMaxLines lines of the live trace; the TUI composes the hint
	// line as "… " + n + ReasoningMore + ReasoningExpand.
	ReasoningMore     = " more · "
	ReasoningExpand   = "ctrl+r expand"
	ReasoningCollapse = "ctrl+r collapse"

	// ask_user dialog copy (Phase 3 Task 11). HelpAsk is the footer help line
	// while a question is pending; the AskHint* lines sit beneath the option
	// list itself and, unlike HelpAsk, always mention the free-text escape
	// hatch (typing instead of picking), since that's the one thing every ask
	// dialog offers regardless of how it's answered.
	HelpAsk             = "up/down move · pgup/pgdn page · enter pick · esc cancel"
	AskHintSingleSelect = "up/down/pgup/pgdn move · enter pick · or type your answer"
	AskHintMultiSelect  = "up/down/pgup/pgdn move · space toggle · enter confirm · or type your answer"
	AskHintFreeText     = "type your answer"

	// AskBlockedByApproval replaces the ask dialog's normal hint when a tool
	// approval is also pending: the approval's key-driven choice mode swallows
	// every keypress (arrows, space, typed text) except its own, so none of
	// the usual ask-dialog affordances actually do anything until it resolves
	// — a silent dead end without this note.
	AskBlockedByApproval = "waiting on the tool approval above — resolve that first"

	// /resume picker copy (Phase 3 Task 15). ResumeTitle is the dialog's
	// heading; HelpResume is the footer help line while the picker is open —
	// no free-text escape hatch here (unlike HelpAsk), since a session id
	// picked from a list has no meaningful typed alternative. ResumeEmpty is
	// the notice for a cwd-scoped picker with nothing to show; ResumeRestored
	// (a %d format string for the message count) confirms a successful
	// restore.
	ResumeTitle    = "resume a session"
	HelpResume     = "up/down move · pgup/pgdn page · enter resume · d delete · esc cancel"
	ResumeEmpty    = "no recorded sessions here · /resume --all to look wider"
	ResumeRestored = "restored session · %d messages"

	// ResumeDeleteConfirm replaces the picker's normal hint once its delete
	// key has been pressed once (Task 20): deleting a recorded session
	// removes the file outright with no trash/undo (chat.SessionStore has
	// neither), so a single "d" only ARMS deletion of the highlighted row —
	// this is the prompt shown while armed. A second "d" (with nothing else
	// pressed in between) performs the delete; any other key cancels the arm.
	ResumeDeleteConfirm = "press d again to delete this session · any other key cancels"

	// YoloUsage is the /yolo slash command's usage error, shown when the
	// argument after "yolo" is neither empty, "on" nor "off".
	YoloUsage = "usage: /yolo [on|off]"

	// Built-in `/` completion entries (tui/completion.go's buildCompItems).
	// Name is the verb shown, matched against the typed query, and used to
	// build the option's Insert token; Desc is the one-line summary shown
	// beside it in the popup.
	CompLoopName    = "loop"
	CompLoopDesc    = "recurring or self-paced task"
	CompCompactName = "compact"
	CompCompactDesc = "compact the conversation"
	CompGoalName    = "goal"
	CompGoalDesc    = "set a goal nib checks before stopping"
	CompModelName   = "model"
	CompModelDesc   = "switch the session model"
	CompModelsName  = "models"
	CompModelsDesc  = "list the models this endpoint serves"
	CompAttachName  = "attach"
	CompAttachDesc  = "stage a file for the next message"
	CompYoloName    = "yolo"
	CompYoloDesc    = "toggle (or on/off) auto-approve every tool call"
	CompResumeName  = "resume"
	CompResumeDesc  = "resume a recorded session"

	// ToolResultNoOutput is fmtBashResult's (chat/resultfmt.go) fallback for a
	// failed bash/bash_job_output call whose stdout and stderr were both
	// empty — a %d format string for the exit code.
	ToolResultNoOutput = "(exit %d, no output)"

	// UsageEstimatedPrefix marks the session usage badge (tui/model.go's
	// usageBadge) when its figure is chat.Session.EstimatedUsage's byte/4
	// guess rather than measured spend — the same "~" convention prunedNotice
	// and compactNotice already use for their own estimates. Kept to a single
	// ASCII character on purpose: footerBadges drops the whole usage badge
	// when the footer is tight, so a longer marker only makes it disappear
	// sooner.
	UsageEstimatedPrefix = "~"
)

// ReasoningMaxLines is how many trailing lines a collapsed reasoning box
// shows.
const ReasoningMaxLines = 5

// CLIApprovePrompt builds the line-based CLI approval prompt (the TUI uses
// the numbered single-key menu instead). alwaysScope describes what `a`
// grants for this call — e.g. "`git …`", "any bash command", or a tool name.
func CLIApprovePrompt(alwaysScope string) string {
	return "y yes · a always (" + alwaysScope + ") · all this turn · n no · or type a change"
}

// Status verbs shown while the agent works.
const (
	VerbThinking = "thinking"
	VerbWorking  = "working"
	VerbReading  = "reading"
)

// EmptyExamples are the sample prompts shown on the first-run empty state.
var EmptyExamples = []string{
	"what changed in the last commit?",
	"undo my last git commit",
	"find every TODO in this repo",
}
