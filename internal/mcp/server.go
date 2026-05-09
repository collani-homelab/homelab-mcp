package mcp

import (
	"context"
	"log/slog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"homelab-mcp/internal/provider"
)

type Server struct {
	mcpServer *mcp.Server
	providers []provider.Provider
}

func NewServer(name, version string) *Server {
	s := mcp.NewServer(
		&mcp.Implementation{
			Name:    name,
			Version: version,
		},
		nil,
	)

	return &Server{
		mcpServer: s,
	}
}

func (s *Server) AddProvider(p provider.Provider) {
	s.providers = append(s.providers, p)

	resources, err := p.GetResources()
	if err != nil {
		slog.Error("Failed to get resources from provider", "provider", p.Name(), "error", err)
		return
	}

	slog.Info("Registering resources for provider", "provider", p.Name(), "count", len(resources))

	for _, res := range resources {
		r := res // Copy for closure
		slog.Debug("Adding resource", "provider", p.Name(), "uri", r.URI)
		s.mcpServer.AddResource(&r, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			slog.Info("Reading resource", "uri", req.Params.URI)
			content, err := p.GetResourceContent(req.Params.URI)
			if err != nil {
				slog.Error("Failed to read resource", "uri", req.Params.URI, "error", err)
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      req.Params.URI,
						MIMEType: r.MIMEType,
						Text:     content,
					},
				},
			}, nil
		})
	}
}

// Providers returns the list of registered providers (useful for testing).
func (s *Server) Providers() []provider.Provider {
	return s.providers
}

func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
