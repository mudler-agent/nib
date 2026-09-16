package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/types"
)

// compCategory tags a completion item by source registry.
type compCategory string

const (
	compBuiltin compCategory = "builtin"
	compCmd     compCategory = "cmd"
	compSkill   compCategory = "skill"
	compAgent   compCategory = "agent"
)

// compItem is one entry in the unified `/` completion list.
type compItem struct {
	Cat    compCategory
	Name   string
	Desc   string
	Insert string // canonical token placed in the input on accept (trailing space)
}

// buildCompItems builds the tagged completion list: the built-in verbs first,
// then the command, skill, and agent registries.
func buildCompItems(cmds []types.CommandConfig, skills []types.Skill, agents []types.AgentTypeConfig) []compItem {
	items := make([]compItem, 0, 7+len(cmds)+len(skills)+len(agents))
	items = append(items,
		compItem{Cat: compBuiltin, Name: theme.CompLoopName, Desc: theme.CompLoopDesc, Insert: "/" + theme.CompLoopName + " "},
		compItem{Cat: compBuiltin, Name: theme.CompCompactName, Desc: theme.CompCompactDesc, Insert: "/" + theme.CompCompactName + " "},
		compItem{Cat: compBuiltin, Name: theme.CompGoalName, Desc: theme.CompGoalDesc, Insert: "/" + theme.CompGoalName + " "},
		compItem{Cat: compBuiltin, Name: theme.CompModelName, Desc: theme.CompModelDesc, Insert: "/" + theme.CompModelName + " "},
		compItem{Cat: compBuiltin, Name: theme.CompModelsName, Desc: theme.CompModelsDesc, Insert: "/" + theme.CompModelsName + " "},
		// skill and agent are deliberately absent: their registries already
		// contribute one entry per name below, which completes to a usable line,
		// where a bare "/skill " would complete to a usage error.
		compItem{Cat: compBuiltin, Name: theme.CompAttachName, Desc: theme.CompAttachDesc, Insert: "/" + theme.CompAttachName + " "},
		compItem{Cat: compBuiltin, Name: theme.CompYoloName, Desc: theme.CompYoloDesc, Insert: "/" + theme.CompYoloName + " "},
		compItem{Cat: compBuiltin, Name: theme.CompResumeName, Desc: theme.CompResumeDesc, Insert: "/" + theme.CompResumeName + " "},
	)
	for _, c := range cmds {
		items = append(items, compItem{Cat: compCmd, Name: c.Name, Desc: c.Description, Insert: "/" + c.Name + " "})
	}
	for _, s := range skills {
		items = append(items, compItem{Cat: compSkill, Name: s.Name, Desc: s.Description, Insert: "/skill " + s.Name + " "})
	}
	for _, a := range agents {
		items = append(items, compItem{Cat: compAgent, Name: a.Name, Desc: a.Description, Insert: "/agent " + a.Name + " "})
	}
	return items
}

// filterComp returns items whose name contains the query (case-insensitive
// substring). An empty query returns all items. Order is preserved.
func filterComp(items []compItem, query string) []compItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]compItem, len(items))
		copy(out, items)
		return out
	}
	var out []compItem
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), q) {
			out = append(out, it)
		}
	}
	return out
}

// compState holds the live `/` completion popup state.
type compState struct {
	all     []compItem
	active  bool
	matches []compItem
	sel     int
}

// setRegistries seeds the completion source from the three registries.
func (c *compState) setRegistries(cmds []types.CommandConfig, skills []types.Skill, agents []types.AgentTypeConfig) {
	c.all = buildCompItems(cmds, skills, agents)
}

// sync recomputes active/matches from the current input. The popup is active
// while the user is still typing the verb: input starts with '/' and contains
// no space yet. Once a space is typed (args begin) it deactivates.
func (c *compState) sync(input string) {
	if !strings.HasPrefix(input, "/") || strings.ContainsAny(input, " \t") {
		c.active = false
		c.matches = nil
		c.sel = 0
		return
	}
	c.active = true
	c.matches = filterComp(c.all, input[1:])
	if len(c.matches) == 0 {
		c.active = false
	}
	if c.sel >= len(c.matches) {
		c.sel = 0
	}
}

func (c *compState) up() {
	if c.sel > 0 {
		c.sel--
	}
}

func (c *compState) down() {
	if c.sel < len(c.matches)-1 {
		c.sel++
	}
}

// current returns the selected match.
func (c *compState) current() (compItem, bool) {
	if !c.active || c.sel < 0 || c.sel >= len(c.matches) {
		return compItem{}, false
	}
	return c.matches[c.sel], true
}

// accept returns the selected item's Insert token.
func (c *compState) accept() (string, bool) {
	it, ok := c.current()
	if !ok {
		return "", false
	}
	return it.Insert, true
}

// exact reports whether input already equals the currently SELECTED match's
// full Insert token, modulo the trailing space ("/yolo" with "yolo"
// highlighted, say) — nothing is left to complete, so a caller like KeyEnter
// should submit rather than accept. Comparing against the selection rather
// than requiring a sole match matters for a genuine prefix pair like the
// built-in "/model" and "/models": typing "/model" in full still produces two
// matches (filterComp is substring, and "models" contains "model"), so a
// sole-match gate would never fire for it — reintroducing the double-Enter
// papercut for exactly the verb that caused it. Moving the selection onto
// "models" instead makes this false again, so Enter still completes to the
// longer verb.
//
// This is keyed on Insert, not "/"+it.Name: for compBuiltin and compCmd items
// Insert is "/" + Name + " ", so the two agree, but compSkill and compAgent
// items complete to "/skill <name> " / "/agent <name> " while Name is just
// the bare name — "/" + it.Name would then equal the typed text for a FULL
// skill/agent name too, wrongly reporting nothing left to complete and
// causing KeyEnter to submit "/explore" as a verb instead of accepting the
// "/agent explore " completion.
func (c *compState) exact(input string) bool {
	it, ok := c.current()
	if !ok {
		return false
	}
	return it.Insert == input+" "
}

// ghost returns the suffix of the selected item's Insert beyond the current
// input (the dim hint shown to the user). Empty if no clean continuation.
func (c *compState) ghost(input string) string {
	it, ok := c.current()
	if !ok {
		return ""
	}
	if strings.HasPrefix(it.Insert, input) {
		return it.Insert[len(input):]
	}
	return ""
}

// renderCompletion renders the popup: a tagged, selectable list plus a ghost hint.
func renderCompletion(c compState, input string, width int) string {
	if !c.active || len(c.matches) == 0 {
		return ""
	}
	var b strings.Builder
	for i, it := range c.matches {
		tag := theme.Meta.Render(fmt.Sprintf("[%s]", it.Cat))
		selected := i == c.sel
		nameStyle := theme.Help
		if selected {
			nameStyle = theme.Prompt
		}
		line := fmt.Sprintf("%s %s %s", tag, nameStyle.Render(fmt.Sprintf("%-16s", it.Name)), theme.Meta.Render(it.Desc))
		if selected {
			line = theme.Prompt.Render(theme.PromptGlyph) + " " + line
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if g := c.ghost(input); g != "" {
		b.WriteString(theme.Hint.Render("tab " + theme.Arrow + " " + input + g))
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}
