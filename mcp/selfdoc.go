package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/mudler/nib/selfdoc"
)

// selfReadInput is the argument to the self_read tool. It takes no arguments
// — the embedded README is returned in full.
type selfReadInput struct{}

// selfReadOutput is the result of the self_read tool.
type selfReadOutput struct {
	Content string `json:"content" jsonschema:"the full text of nib's embedded README.md — its features, configuration, slash commands, plugins, and skills"`
	Success bool   `json:"success" jsonschema:"whether operation was successful"`
}

// selfRead returns the embedded README.md. It needs no filesystem path and no
// network connection: the documentation is compiled into the binary.
func selfRead(_ context.Context, _ *mcp.CallToolRequest, _ selfReadInput) (*mcp.CallToolResult, selfReadOutput, error) {
	return nil, selfReadOutput{
		Content: selfdoc.EmbeddedReadme(),
		Success: true,
	}, nil
}

// StartSelfDocMCPServer starts an in-memory MCP server exposing a self_read
// tool that returns nib's embedded README.md. The agent calls it when the user
// asks about nib itself — its features, configuration, slash commands, etc.
func StartSelfDocMCPServer(ctx context.Context, transport mcp.Transport) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "selfdoc",
		Version: "v1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "self_read",
		Description: "Read nib's own embedded documentation (README.md). Call this when the user asks about nib itself — its features, configuration, slash commands, plugins, skills, or how to use a capability. Returns the full text. Takes no arguments.",
	}, selfRead)

	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("selfdoc MCP server: %w", err)
	}
	return nil
}
