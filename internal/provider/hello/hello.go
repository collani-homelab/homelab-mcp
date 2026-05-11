package hello

import (
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HelloProvider struct{}

func (h *HelloProvider) Name() string {
	return "hello"
}

func (h *HelloProvider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:         "hello://world",
			Name:        "Hello World",
			Description: "A simple hello world resource",
			MIMEType:    "text/plain",
		},
	}, nil
}

func (h *HelloProvider) GetResourceContent(uri string) (string, error) {
	if uri == "hello://world" {
		return "Hello from the Homelab MCP Server!", nil
	}
	return "", fmt.Errorf("resource not found: %s", uri)
}

func (h *HelloProvider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (h *HelloProvider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{}, nil
}

func (h *HelloProvider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (h *HelloProvider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{}, nil
}

func (h *HelloProvider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	return nil, fmt.Errorf("tool not found: %s", name)
}
