// Package builtin provides always-present skills that ship compiled into the
// binary. They are merged into cfg.Skills at config load time, before
// user/plugin/pack skills, so anything the user configures with the same
// name takes precedence.
//
// Currently the only built-in skill is "about-nib", whose instructions are
// the full embedded README.md. The model loads it on demand via the
// load_skill tool when asked about nib's features, configuration, slash
// commands, etc.
package builtin

import (
	"github.com/mudler/nib/selfdoc"
	"github.com/mudler/nib/types"
)

// Skills returns the built-in skills that are always available. They form
// the lowest rung of the precedence ladder: user skills, plugin skills,
// and skill-pack skills all override a built-in of the same name.
func Skills() []types.Skill {
	return []types.Skill{
		{
			Name:         "about-nib",
			Description:  "nib's own documentation — features, configuration, slash commands, plugins, skills, MCP servers, tmux, and embedding. Load this when the user asks about nib itself or how to use a nib capability.",
			Instructions: selfdoc.EmbeddedReadme(),
		},
	}
}

// Apply merges built-in skills into cfg.Skills, skipping any name already
// present. It is called from config.LoadWith before skill.Apply and
// plugin.Apply so that user/plugin/pack skills always win.
func Apply(cfg *types.Config) {
	existing := make(map[string]bool, len(cfg.Skills))
	for _, s := range cfg.Skills {
		existing[s.Name] = true
	}
	var merged []types.Skill
	for _, b := range Skills() {
		if !existing[b.Name] {
			merged = append(merged, b)
		}
	}
	merged = append(merged, cfg.Skills...)
	cfg.Skills = merged
}
