package mcp

import (
	"context"
	"encoding/json"
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
			content, err := p.GetResourceContent(ctx, req.Params.URI)
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
				content, err := p.GetResourceContent(ctx, req.Params.URI)
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
				return p.GetPrompt(ctx, req.Params.Name, req.Params.Arguments)
			})
		}
	}

	tools, err := p.GetTools()
	if err != nil {
		slog.Error("Failed to get tools from provider", "provider", p.Name(), "error", err)
		return
	}

	if len(tools) > 0 {
		slog.Info("Registering tools for provider", "provider", p.Name(), "count", len(tools))
	}

	for _, tool := range tools {
		t := tool // Copy for closure
		slog.Debug("Adding tool", "provider", p.Name(), "name", t.Name)
		s.mcpServer.AddTool(&t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			slog.Info("Calling tool", "name", req.Params.Name)
			var args map[string]interface{}
			if req.Params.Arguments != nil {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					slog.Error("Failed to unmarshal tool arguments", "tool", req.Params.Name, "error", err)
					// Return a tool-level error so the LLM can self-correct
					result := &mcp.CallToolResult{}
					result.SetError(err)
					return result, nil
				}
			}
			return p.CallTool(ctx, req.Params.Name, args)
		})
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

	mediaPrompt := mcp.Prompt{
		Name:        "media_stack_status",
		Description: "Provides a consolidated status report for the entire media stack: active Plex streams, Usenet download progress, and the TV/movie/music acquisition queues.",
	}
	s.mcpServer.AddPrompt(&mediaPrompt, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: `Please read the following resources and provide a consolidated media stack status report:

**Streaming Activity**
- plex://sessions — who is currently watching, what they're watching, and the stream quality.
- tautulli://activity — current transcoding/direct play sessions from Tautulli.

**Download Queue**
- nzbget://status — overall NZBGet download speed, paused/running state, and remaining data.
- nzbget://listgroups — individual items currently downloading via Usenet.

**Acquisition Queues**
- sonarr://queue — TV episodes currently queued or downloading in Sonarr.
- radarr://queue — movies currently queued or downloading in Radarr.
- lidarr://queue — music currently queued or downloading in Lidarr.

Summarize into three sections: 1) What's streaming right now, 2) What's downloading and at what speed, 3) Any queue issues or stalled items that need attention.`,
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
		
		mcpHandler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return s.mcpServer
		}, &mcp.SSEOptions{})

		bearerToken := os.Getenv("MCP_BEARER_TOKEN")
		corsOrigin := os.Getenv("CORS_ALLOW_ORIGIN")
		if corsOrigin == "" {
			corsOrigin = "http://localhost:3000"
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bearerToken != "" && r.Method != "OPTIONS" {
				if r.Header.Get("Authorization") != "Bearer "+bearerToken {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id, Mcp-Protocol-Version")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			mcpHandler.ServeHTTP(w, r)
		})
		
		srv := &http.Server{Addr: ":" + port, Handler: handler}
		go func() {
			<-ctx.Done()
			if err := srv.Shutdown(context.Background()); err != nil {
				slog.Error("SSE server shutdown error", "err", err)
			}
		}()
		slog.Info("Starting MCP server via Streamable HTTP (SSE compatible)", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}

	slog.Info("Starting MCP server via stdio")
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

