package cmd

import (
	"fmt"
	"strings"

	"github.com/mudler/nib/config"
	"github.com/mudler/nib/internal"
	"github.com/mudler/nib/types"
)

// AboutText renders the /about output: program name, version, commit,
// config paths, model/provider, and the tool/skill/agent/MCP inventory.
// It is used by both the CLI and the TUI.
func AboutText(cfg types.Config) string {
	prog := types.ProgramNameOr(cfg.ProgramName)
	v := strings.TrimSpace(internal.PrintableVersion())
	if v == "()" || v == "" {
		v = "dev (local build)"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s %s\n", prog, v))
	b.WriteString("\n")

	// Config paths
	b.WriteString("Configuration paths (first existing wins):\n")
	for _, p := range config.ConfigPaths() {
		b.WriteString(fmt.Sprintf("  %s\n", p))
	}
	b.WriteString(fmt.Sprintf("Writable config: %s\n", config.WritablePath()))
	b.WriteString("\n")

	// Model/endpoint
	model := cfg.Model
	if model == "" {
		model = "(unset)"
	}
	provider := cfg.Provider
	if provider == "" {
		provider = "(unset)"
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "(default)"
	}
	b.WriteString(fmt.Sprintf("Provider: %s\n", provider))
	b.WriteString(fmt.Sprintf("Model: %s\n", model))
	b.WriteString(fmt.Sprintf("Base URL: %s\n", baseURL))
	b.WriteString("\n")

	// Tools
	b.WriteString("Built-in tools:\n")
	if len(cfg.BuiltinTools) == 0 {
		b.WriteString("  (all enabled)\n")
	} else {
		for _, t := range cfg.BuiltinTools {
			b.WriteString(fmt.Sprintf("  %s\n", t))
		}
	}
	b.WriteString("\n")

	// Skills
	if len(cfg.Skills) > 0 {
		b.WriteString("Skills:\n")
		for _, s := range cfg.Skills {
			b.WriteString(fmt.Sprintf("  %s: %s\n", s.Name, s.Description))
		}
		b.WriteString("\n")
	}

	// Agents
	if len(cfg.Agents) > 0 {
		b.WriteString("Agents:\n")
		for _, a := range cfg.Agents {
			b.WriteString(fmt.Sprintf("  %s: %s\n", a.Name, a.Description))
		}
		b.WriteString("\n")
	}

	// MCP servers
	if len(cfg.MCPServers) > 0 {
		b.WriteString("MCP servers:\n")
		for name := range cfg.MCPServers {
			b.WriteString(fmt.Sprintf("  %s\n", name))
		}
		b.WriteString("\n")
	}

	b.WriteString("The agent can read its own documentation via the self_read tool.\n")
	b.WriteString("Documentation is embedded in the binary — no network required.\n")

	return b.String()
}
