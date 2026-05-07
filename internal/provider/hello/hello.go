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
