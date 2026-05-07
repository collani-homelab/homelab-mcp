package provider

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Provider interface {
	Name() string
	GetResources() ([]mcp.Resource, error)
	GetResourceContent(uri string) (string, error)
}