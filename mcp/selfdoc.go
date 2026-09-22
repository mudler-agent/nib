package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/mudler/nib/selfdoc"
)

// selfReadInput is the argument to the self_read tool. When Section is empty,
// the tool returns the table of contents — a compact list of available
// sections. When Section names a heading (case-insensitive), the tool
// returns that section's body text only.
type selfReadInput struct {
	Section string `json:"section,omitempty" jsonschema:"the section heading to read (case-insensitive). Omit to list all available sections."`
}

// selfReadOutput is the result of the self_read tool.
type selfReadOutput struct {
	Content string `json:"content" jsonschema:"either the table of contents (when section is omitted) or the section's body text"`
	Section string `json:"section,omitempty" jsonschema:"the section name that was read, or 'table of contents'"`
	Found   bool   `json:"found" jsonschema:"whether the section was found"`
}

// selfRead returns either the table of contents (compact) or a single section's
// body. It never returns the full README — that would consume too much context.
// The model is expected to call it with no arguments first, then call it again
// with the section name it wants.
func selfRead(_ context.Context, _ *mcp.CallToolRequest, in selfReadInput) (*mcp.CallToolResult, selfReadOutput, error) {
	if in.Section == "" {
		return nil, selfReadOutput{
			Content: selfdoc.TableOfContents(),
			Section: "table of contents",
			Found:   true,
		}, nil
	}

	sec, ok := selfdoc.SectionByName(in.Section)
	if !ok {
		return nil, selfReadOutput{
			Content: fmt.Sprintf("Section %q not found. Call self_read with no arguments to list available sections.", in.Section),
			Section: in.Section,
			Found:   false,
		}, nil
	}

	return nil, selfReadOutput{
		Content: sec.Body,
		Section: sec.Title,
		Found:   true,
	}, nil
}

// StartSelfDocMCPServer starts an in-memory MCP server exposing a self_read
// tool that returns nib's embedded README.md, one section at a time. The
// agent calls it when the user asks about nib itself — its features,
// configuration, slash commands, plugins, skills, or how to use a capability.
func StartSelfDocMCPServer(ctx context.Context, transport mcp.Transport) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "selfdoc",
		Version: "v1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_read",
		Description: "Read nib's own embedded documentation (README.md). Call with no arguments to list available sections, then call again with a section name to read that section. Use this when the user asks about nib itself — its features, configuration, slash commands, plugins, skills, or how to use a capability.",
	}, selfRead)

	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("selfdoc MCP server: %w", err)
	}
	return nil
}
