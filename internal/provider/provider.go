package provider

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Provider is the core interface every homelab integration must implement.
// Resources expose read-only state (data/snapshots); Tools expose
// actions/mutations (read-write operations).
type Provider interface {
	Name() string

	// --- Resources (read-only state) ---
	GetResources() ([]mcp.Resource, error)
	GetResourceContent(ctx context.Context, uri string) (string, error)
	GetResourceTemplates() ([]mcp.ResourceTemplate, error)

	// --- Prompts ---
	GetPrompts() ([]mcp.Prompt, error)
	GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error)

	// --- Tools (actions / mutations) ---
	GetTools() ([]mcp.Tool, error)
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error)
}