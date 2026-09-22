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
	"strings"
)

//go:embed README.md
var embeddedReadme string

// EmbeddedReadme returns the full README.md that was compiled into the binary.
func EmbeddedReadme() string {
	return embeddedReadme
}

// Section is a top-level (##) heading from the README plus its body text.
type Section struct {
	Title string
	Body  string
}

// ReadmeSections parses the embedded README into top-level (##) sections.
// The heading line (e.g. "## Quickstart") is stripped from the title; the
// body includes everything from the line after the heading up to the next
// ## heading or end of file.
func ReadmeSections() []Section {
	return parseSections(embeddedReadme)
}

// SectionByName returns the section whose title matches name (case-insensitive),
// or "" if not found.
func SectionByName(name string) (Section, bool) {
	for _, s := range ReadmeSections() {
		if strings.EqualFold(s.Title, name) {
			return s, true
		}
	}
	return Section{}, false
}

// TableOfContents returns a compact index of the README's sections — just
// the section titles, one per line. The model reads this first to decide
// which section to fetch, keeping context usage low.
func TableOfContents() string {
	var b strings.Builder
	for _, s := range ReadmeSections() {
		b.WriteString("- ")
		b.WriteString(s.Title)
		b.WriteString("\n")
	}
	return b.String()
}

// parseSections splits markdown into top-level (##) sections. Everything
// before the first ## heading (the title block, badges, intro) is returned
// as a section titled "Intro".
func parseSections(md string) []Section {
	lines := strings.Split(md, "\n")
	var sections []Section
	var current Section
	var body []string

	flush := func() {
		if current.Title != "" {
			current.Body = strings.TrimRight(strings.Join(body, "\n"), "\n")
			sections = append(sections, current)
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			current = Section{Title: strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))}
			body = nil
		} else if current.Title != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}
