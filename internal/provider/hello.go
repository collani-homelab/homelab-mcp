package provider

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HelloProvider is a skeleton provider for testing the MCP server implementation.
type HelloProvider struct{}

func (p *HelloProvider) Name() string {
	return "hello-world"
}

func (p *HelloProvider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:         "hello://world",
			Name:        "Hello World Resource",
			Description: "A simple skeleton resource to test the homelab-mcp server",
			MIMEType:    "text/plain",
		},
	}, nil
}

func (p *HelloProvider) GetResourceContent(uri string) (string, error) {
	// Validate this is the URI we expect
	if uri == "hello://world" {
		return "Hello, Agentic Homelab World! The MCP server is responding successfully.", nil
	}
	
	// Return an error if the AI asks for a resource URI we don't own
	return "", fmt.Errorf("resource not found: %s", uri)
}