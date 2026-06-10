package lidarr

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcphelper "homelab-mcp/internal/mcp"
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
	return "Lidarr"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "lidarr://queue",
			Name:     "Lidarr Download Queue",
			MIMEType: "application/json",
		},
		{
			URI:      "lidarr://system/status",
			Name:     "Lidarr System Status",
			MIMEType: "application/json",
		},
		{
			URI:      "lidarr://artist",
			Name:     "Lidarr Artists",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	var endpoint string
	switch uri {
	case "lidarr://queue":
		endpoint = "/api/v1/queue"
	case "lidarr://system/status":
		endpoint = "/api/v1/system/status"
	case "lidarr://artist":
		data, err := p.fetchFromLidarr("/api/v1/artist")
		if err != nil {
			return "", err
		}
		limited, err := filterArtistList([]byte(data), "")
		if err != nil {
			return data, nil
		}
		return string(limited), nil
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromLidarr(endpoint)
}

func (p *Provider) fetchFromLidarr(apiPath string) (string, error) {
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
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	content := string(bodyBytes)

	// Prune heavy keys
	pruned, err := provider.PruneJSON([]byte(content), []string{"images", "overview", "links", "statistics"})
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
			Name:        "check_music_downloads",
			Description: "Checks the Lidarr queue for active music downloads and their status.",
			Arguments:   []*mcp.PromptArgument{},
		},
	}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "check_music_downloads" {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please check the lidarr://queue resource and tell me what music is currently downloading and its progress.",
					},
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		*mcphelper.NewTool("search_lidarr_artists", "Search the Lidarr library for tracked music artists. Returns matching artists pruned of heavy metadata. Omit the query to get the first 25 artists alphabetically.", map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Case-insensitive artist name search term. Omit to list the first 25 artists.",
			},
		}),
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if name == "search_lidarr_artists" {
		data, err := p.fetchFromLidarr("/api/v1/artist")
		if err != nil {
			return mcphelper.ErrorResult(fmt.Errorf("failed to fetch artists: %w", err)), nil
		}
		query, _ := arguments["query"].(string)
		result, err := filterArtistList([]byte(data), query)
		if err != nil {
			return mcphelper.ErrorResult(fmt.Errorf("failed to filter artists: %w", err)), nil
		}
		return mcphelper.TextResult(string(result)), nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func filterArtistList(data []byte, query string) ([]byte, error) {
	var artists []map[string]interface{}
	if err := json.Unmarshal(data, &artists); err != nil {
		return data, err
	}
	q := strings.ToLower(query)
	var matched []map[string]interface{}
	for _, a := range artists {
		name, _ := a["artistName"].(string)
		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		matched = append(matched, a)
		if q == "" && len(matched) >= 25 {
			break
		}
	}
	if matched == nil {
		matched = []map[string]interface{}{}
	}
	return json.MarshalIndent(matched, "", "  ")
}
