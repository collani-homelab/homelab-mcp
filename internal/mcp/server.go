package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"os"

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

	templates, err := p.GetResourceTemplates()
	if err == nil {
		for _, tmpl := range templates {
			t := tmpl // Copy for closure
			slog.Debug("Adding resource template", "provider", p.Name(), "uriTemplate", t.URITemplate)
			s.mcpServer.AddResourceTemplate(&t, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				slog.Info("Reading resource from template", "uri", req.Params.URI)
				content, err := p.GetResourceContent(req.Params.URI)
				if err != nil {
					slog.Error("Failed to read resource template", "uri", req.Params.URI, "error", err)
					return nil, err
				}
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:      req.Params.URI,
							MIMEType: t.MIMEType,
							Text:     content,
						},
					},
				}, nil
			})
		}
	}

	prompts, err := p.GetPrompts()
	if err == nil {
		for _, prompt := range prompts {
			pr := prompt // Copy for closure
			slog.Debug("Adding prompt", "provider", p.Name(), "name", pr.Name)
			s.mcpServer.AddPrompt(&pr, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				slog.Info("Executing prompt", "name", req.Params.Name)
				return p.GetPrompt(req.Params.Name, req.Params.Arguments)
			})
		}
	}
}

// Providers returns the list of registered providers (useful for testing).
func (s *Server) Providers() []provider.Provider {
	return s.providers
}

func (s *Server) Run(ctx context.Context) error {
	// Register global prompts
	prompt := mcp.Prompt{
		Name:        "homelab_status_report",
		Description: "A pre-defined prompt that fetches high-level health from all homelab providers.",
	}
	s.mcpServer.AddPrompt(&prompt, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please read the system stats from Unraid and the network health from UniFi, and provide a high-level summary of the homelab's status.",
					},
				},
			},
		}, nil
	})

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "sse" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		
		sse := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return s.mcpServer
		}, &mcp.SSEOptions{})
		
		// Basic CORS wrapper
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			sse.ServeHTTP(w, r)
		})
		
		slog.Info("Starting MCP server via SSE", "port", port)
		return http.ListenAndServe(":"+port, handler)
	}

	slog.Info("Starting MCP server via stdio")
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

