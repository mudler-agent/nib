# Hide `ask_user` in YOLO mode

## Goal

YOLO mode must run without blocking prompts. The session must not advertise
the `ask_user` tool while automatic approval is active.

This rule applies when automatic approval comes from any supported source:

- `--yolo`
- `NIB_YOLO`
- `approval_mode: auto`
- `/yolo on`
- the session settings API

Turning automatic approval off must restore `ask_user` for the next model
request when the built-in tool allowlist permits it.

## Design

`Session.toolOptions` builds the tool set for each model request. It must add
`ask_user` only when both conditions are true:

1. The built-in tool allowlist enables `ask_user`.
2. `Session.AutoApprove()` is false.

The session already stores the automatic-approval state in an atomic value.
The tool builder reads that value when it builds each request. This keeps
runtime `/yolo` changes effective on the next turn without new state.

The implementation must not auto-answer questions. It must not register a
replacement tool. A model can still ask a question in ordinary response text,
because YOLO mode controls tools rather than generated text.

## Error handling

No new error path is required. A model cannot select `ask_user` when the tool
is absent from the request schema. Existing provider behavior handles any
invalid tool call that a provider emits without a matching schema.

## Tests

Tests must inspect the real tool options produced by a session and prove these
cases:

- A normal session advertises `ask_user` when the allowlist permits it.
- An automatic-approval session does not advertise `ask_user`.
- Enabling automatic approval removes `ask_user` from the next tool set.
- Disabling automatic approval restores `ask_user` in the next tool set.
- An allowlist that excludes `ask_user` continues to exclude it after YOLO is
  disabled.

Run `go test ./...` after the focused tests pass.

## Out of scope

- Preventing questions in ordinary assistant text.
- Automatically choosing an option or returning an empty answer.
- Changing approval behavior for other tools.
- Changing the behavior of a tool call that is already running.
