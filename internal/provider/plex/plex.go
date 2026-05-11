package plex

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
	token   string
}

func NewProvider(baseURL, token string, skipVerify bool) (*Provider, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
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
		token:   token,
	}, nil
}

func (p *Provider) Name() string {
	return "Plex"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "plex://sessions",
			Name:     "Plex Active Sessions",
			MIMEType: "application/json",
		},
		{
			URI:      "plex://servers",
			Name:     "Plex Connected Servers",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	var endpoint string
	switch uri {
	case "plex://sessions":
		endpoint = "/status/sessions"
	case "plex://servers":
		endpoint = "/servers"
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromPlex(endpoint)
}

func (p *Provider) fetchFromPlex(apiPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fullURL := fmt.Sprintf("%s%s", p.baseURL, apiPath)
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if p.token != "" {
		req.Header.Set("X-Plex-Token", p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	content := string(bodyBytes)

	// Prune heavy keys
	pruned, err := provider.PruneJSON([]byte(content), []string{"thumb", "art", "parentThumb", "grandparentThumb", "guid", "key", "ratingKey", "sessionKey"})
	if err == nil {
		content = string(pruned)
	}

	return content, nil
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{
		{
			Name:        "analyze_plex_sessions",
			Description: "Analyzes current Plex streaming sessions directly from the Plex server.",
			Arguments:   []*mcp.PromptArgument{},
		},
	}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "analyze_plex_sessions" {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please check the plex://sessions resource and summarize the current streaming activity on Plex, including who is watching what.",
					},
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("prompt not found: %s", name)
}
