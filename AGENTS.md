# AGENTS.md

## Updating documentation

The README.md is the single source of user-facing documentation. It is also
compiled into the binary via `go:embed` in `selfdoc/docs.go` and surfaced as
the built-in "about-nib" skill (see `builtin/builtin.go`). The model loads
this skill on demand via the `load_skill` tool when asked about nib's own
features, configuration, or capabilities.

When you change user-facing behavior, update README.md too. Then sync the
embedded copy:

```
make sync-readme
```

This copies `README.md` to `selfdoc/README.md`. Commit both files together.

If you skip this, the "about-nib" skill serves stale documentation, and the
model will answer questions about nib with outdated information.

## CI checks

`go test ./...` includes `selfdoc` tests that verify the embedded README
contains its major section headings, and `builtin` tests that verify the
"about-nib" skill is present and can be overridden by user config.
