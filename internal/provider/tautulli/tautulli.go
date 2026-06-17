package tautulli

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"homelab-mcp/internal/provider"
)

type Provider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func NewProvider(baseURL, apiKey string, skipVerify bool) (*Provider, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("apiKey is required")
	}

	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	return &Provider{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}, nil
}

func (p *Provider) Name() string {
	return "Tautulli"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "tautulli://activity",
			Name:     "Tautulli Current Activity",
			MIMEType: "application/json",
		},
		{
			URI:      "tautulli://history",
			Name:     "Tautulli Watch History",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(ctx context.Context, uri string) (string, error) {
	var cmd string
	switch uri {
	case "tautulli://activity":
		cmd = "get_activity"
	case "tautulli://history":
		cmd = "get_history"
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromTautulli(ctx, cmd)
}

func (p *Provider) fetchFromTautulli(ctx context.Context, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/api/v2?apikey=%s&cmd=%s", p.baseURL, p.apiKey, cmd)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	content := string(bodyBytes)

	// Tautulli responses can be large. Let's prune some common metadata keys or token heavy ones.
	// Common keys to prune might be image urls or identifiers that aren't useful.
	pruned, err := provider.PruneJSON([]byte(content), []string{"thumb", "art", "grandparent_thumb", "user_thumb", "session_key", "machine_id", "session_id"})
	if err == nil {
		return string(pruned), nil
	}
	return content, nil
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{
		{
			Name:        "analyze_plex_activity",
			Description: "Analyzes current Plex streaming activity using Tautulli",
			Arguments:   []*mcp.PromptArgument{},
		},
	}, nil
}

func (p *Provider) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "analyze_plex_activity" {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please check tautulli://activity for current transcoding sessions — for each stream note whether it is direct play or being transcoded, the reason if transcoding, and the video/audio codec decisions. Then check tautulli://history to summarize what has been watched in the last 24 hours.",
					},
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{}, nil
}

func (p *Provider) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	return nil, fmt.Errorf("tool not found: %s", name)
}
