package types

import "strings"

// selfKnowledgeSuffix is the self-knowledge paragraph appended to the system
// prompt. It tells the model:
//   - That it is <prog>, an AI terminal assistant.
//   - That it can read its own documentation via the self_read tool.
//   - Which topics the documentation covers.
//
// The version string is injected by the Session (which has access to
// internal.Version) — see chat.Session.Reload. It is appended after this text.
//
// This text lives here (not in config.defaultPrompt) for the same reason
// toolGuidance does: a custom `prompt:` in config REPLACES defaultPrompt
// outright, so anything in it is lost for customized deployments. Appending
// here reaches every session unconditionally.
func selfKnowledgeSuffix(prog string) string {
	var b strings.Builder
	b.WriteString("You are ")
	b.WriteString(prog)
	b.WriteString(", an AI terminal assistant. ")
	b.WriteString("When the user asks about ")
	b.WriteString(prog)
	b.WriteString(" itself — its features, configuration, slash commands, plugins, skills, or how to use a capability — ")
	b.WriteString("call the self_read tool to read ")
	b.WriteString(prog)
	b.WriteString("'s embedded documentation. ")
	b.WriteString("Call self_read with no arguments first to list available sections, then call it again with a section name to read that section. ")
	b.WriteString("Do not guess or fabricate ")
	b.WriteString(prog)
	b.WriteString("'s features: read the documentation first, then answer.")
	return b.String()
}
