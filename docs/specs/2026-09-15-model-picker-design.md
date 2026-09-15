# Searchable model picker

**Date:** 2026-09-15
**Status:** Approved design, ready for implementation planning

## Goal

Make model switching practical in the TUI. Bare `/model` opens a searchable
picker that uses the configured OpenAI-compatible models endpoint. The selected
model becomes the session model without changing the conversation history.

## Existing behavior

nib already supports model discovery and session switching:

- `chat.Session.ListModels` reads model IDs from the configured `/v1/models`
  endpoint.
- `chat.Session.SetModel` rebuilds the model client and preserves history.
- `/models` prints the endpoint model list.
- `/model <name>` validates and switches the session model.
- Bare `/model` currently prints the same list as `/models`.

The new picker changes only the bare `/model` experience in the TUI. The CLI
and explicit command forms keep their current behavior.

## Command behavior

The slash resolver distinguishes the following actions:

- `/model` requests the interactive model picker.
- `/models` requests the existing text listing.
- `/model <name>` requests the existing direct switch.

The TUI handles the picker action. The plain CLI treats it as a text model
listing because a full-screen selector is not available there.

## Picker interaction

Bare `/model` starts an asynchronous call to `Session.ListModels`. The TUI stays
responsive while the endpoint request runs. A loading message replaces the
normal composer until the call finishes or reaches `chat.ModelListTimeout`.

After a successful call, the picker shows the returned model IDs in endpoint
order. It marks the current session model and selects it initially. If the
current model is absent, the picker selects the first model.

The picker owns keyboard input while it is open:

- Printable characters append to the search query.
- Backspace removes the final character from the query.
- Up and Down move the selection within the filtered results.
- Enter selects the highlighted model and closes the picker.
- Esc closes the picker without changing the session.

Filtering uses a case-insensitive substring match. A query change resets the
selection to the first match. The picker displays `No matching models.` when no
model matches. Enter has no effect in that state.

The picker displays the query, visible matches, the current-model marker, and a
short key hint. Its list height is bounded by the available terminal height.
The picker scrolls when the filtered result count exceeds that height.

## Switching behavior

The picker obtains every selectable name from `Session.ListModels`. Therefore,
selection calls `Session.SetModel` directly and does not repeat the endpoint
request. The TUI appends the existing `model: <name>` confirmation to the
transcript after the switch.

The switch applies to the next model turn. A turn already in flight keeps its
client snapshot. The session preserves messages, model context, tools,
credentials, metadata, tracing, and reasoning effort. The new model starts with
a cold prefix, as it does for the existing direct switch.

The TUI permits the picker only when the session is ready and no model turn is
active. This avoids changing the interaction mode while queued or background
work owns the composer. Direct `/model <name>` behavior remains unchanged.

## Components

### Slash action

Add a dedicated picker action to `slash.Action.Kind`. Bare `/model` resolves to
this action. `/models` and `/model <name>` retain their existing action kinds.

### Picker state

Add a focused TUI component that owns:

- the complete endpoint model list;
- the filtered model list;
- the current query;
- the selected result index;
- the visible window offset;
- the loading state; and
- any endpoint error.

The component exposes small operations for opening, applying loaded models,
filtering, moving the selection, choosing a model, closing, and rendering.

### Asynchronous endpoint request

The TUI dispatches a Bubble Tea command that calls `Session.ListModels` with
`chat.ModelListTimeout`. The command returns a private message that contains
either the model IDs or an error. The update loop applies that message only
when the picker still awaits the same request. This prevents a late response
from reopening a picker that the user cancelled.

### TUI integration

The main model routes keys to the picker before routing them to completion,
queue, approval, or composer behavior. The view renders the picker above the
composer area and hides the normal composer while the picker is active.

Existing `/` command completion continues to offer `/model`. Accepting that
completion inserts `/model ` as it does now. Submitting bare `/model` opens the
picker.

## Error handling

- If model discovery fails, the TUI closes the loading state and adds the
  endpoint error to the transcript.
- If the endpoint returns no models, the picker opens with a clear
  `No models available.` message. Enter does nothing, and Esc closes it.
- If filtering returns no matches, the picker stays open and accepts edits.
- If `SetModel` cannot build a client, current `SetModel` behavior logs the
  failure and retains the current model. This feature does not change that API.
- Cancelling the picker never changes the current model.

## Testing

Use test-driven development for each behavior.

- Slash tests prove that bare `/model`, `/models`, and `/model <name>` resolve
  to distinct actions.
- Picker unit tests cover case-insensitive filtering, selection reset,
  navigation bounds, scrolling, empty results, and current-model selection.
- TUI update tests prove that `/model` starts an asynchronous lookup and keeps
  the update loop responsive.
- TUI update tests cover loading success, loading failure, cancellation, late
  response rejection, Enter selection, and Esc cancellation.
- Rendering tests cover the query, highlighted result, current marker, empty
  endpoint result, no search matches, and key hints.
- Regression tests prove that `/models`, `/model <name>`, and the CLI retain
  their existing behavior.
- Session tests continue to prove that `SetModel` preserves history and runtime
  configuration.

Run the focused `slash`, `chat`, `tui`, and `cmd` package tests. Then run the
complete Go test suite before completion.

## Out of scope

- Fuzzy matching or ranking
- Model metadata beyond the endpoint model ID
- Persisting the selected model to configuration
- Changing the model for a turn that is already running
- Adding a full-screen selector to the plain CLI
- Caching model lists across picker openings
