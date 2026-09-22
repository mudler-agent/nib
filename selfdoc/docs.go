//go:generate cp ../README.md README.md

// Package selfdoc embeds nib's user-facing README.md into the binary so the
// agent can read its own documentation at runtime — its features,
// configuration, slash commands, plugins, and skills — without a network
// connection or filesystem access to the install directory.
//
// The embedded copy is synced from the root README.md by `go generate` (or
// `make sync-readme`). Run it after editing README.md.
package selfdoc

import (
	_ "embed"
)

//go:embed README.md
var embeddedReadme string

// EmbeddedReadme returns the README.md that was compiled into the binary.
// It is the user-facing documentation the agent reads when asked about nib
// itself — its features, configuration, slash commands, plugins, and skills.
func EmbeddedReadme() string {
	return embeddedReadme
}
