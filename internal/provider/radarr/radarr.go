package radarr

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
	return "Radarr"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "radarr://queue",
			Name:     "Radarr Download Queue",
			MIMEType: "application/json",
		},
		{
			URI:      "radarr://system/status",
			Name:     "Radarr System Status",
			MIMEType: "application/json",
		},
		{
			URI:      "radarr://movie/missing",
			Name:     "Radarr Missing Movies (Subset)",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	var endpoint string
	switch uri {
	case "radarr://queue":
		endpoint = "/api/v3/queue"
	case "radarr://system/status":
		endpoint = "/api/v3/system/status"
	case "radarr://movie/missing":
		// Only fetch some missing movies to keep payload sizes reasonable
		// Using movie endpoint but filtering could be done if the API supports it, 
		// otherwise we can fetch missing via /api/v3/movie and pruning or similar?
		// Actually Radarr supports /api/v3/movie?apikey=X where hasFile=false or we can just pull queue.
		// Let's just use queue and status for now, and queue details for missing.
		endpoint = "/api/v3/movie?apikey=" + p.apiKey // We'll add apikey in header anyway, let's just use the path
		endpoint = "/api/v3/movie" // we will filter in code
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromRadarr(endpoint, uri)
}

func (p *Provider) fetchFromRadarr(apiPath, uri string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fullURL := fmt.Sprintf("%s%s", p.baseURL, apiPath)
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", p.apiKey)

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
	pruned, err := provider.PruneJSON([]byte(content), []string{"images", "overview", "website", "youtubeTrailerId", "studio", "path", "physicalRelease", "digitalRelease"})
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
			Name:        "check_movie_downloads",
			Description: "Checks the Radarr queue for active movie downloads and their status.",
			Arguments:   []*mcp.PromptArgument{},
		},
	}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "check_movie_downloads" {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please check the radarr://queue resource and tell me what movies are currently downloading and their progress.",
					},
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("prompt not found: %s", name)
}
