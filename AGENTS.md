# AGENTS.md

## Updating documentation

The README.md is the single source of user-facing documentation. It is also
compiled into the binary via `go:embed` in `selfdoc/docs.go`, so the agent can
read its own documentation at runtime through the `self_read` tool.

When you change user-facing behavior, update README.md too. Then sync the
embedded copy:

```
make sync-readme
```

This copies `README.md` to `selfdoc/README.md`. Commit both files together.

If you skip this, the agent's `self_read` tool serves stale documentation,
and the model will answer questions about nib with outdated information.

## CI checks

`go test ./...` includes `selfdoc` tests that verify the embedded README
contains its major section headings and that sections have non-empty bodies.
These tests fail if the copy is empty or the wrong file was embedded.
