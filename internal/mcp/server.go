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
		// As per the implementation plan, log errors to stderr.
		slog.Error("failed to get resources from provider", "provider", p.Name(), "error", err)
		return
	}

	for _, res := range resources {
		r := res // Copy for closure
		s.mcpServer.AddResource(&r, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			content, err := p.GetResourceContent(req.Params.URI)
			if err != nil {
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

func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
