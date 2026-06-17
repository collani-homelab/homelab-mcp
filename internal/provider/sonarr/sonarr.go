package sonarr

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

	var transport *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = dt.Clone()
	} else {
		transport = &http.Transport{}
	}
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
	return "Sonarr"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "sonarr://queue",
			Name:     "Sonarr Download Queue",
			MIMEType: "application/json",
		},
		{
			URI:      "sonarr://system/status",
			Name:     "Sonarr System Status",
			MIMEType: "application/json",
		},
		{
			URI:      "sonarr://series",
			Name:     "Sonarr Series List",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(ctx context.Context, uri string) (string, error) {
	var endpoint string
	switch uri {
	case "sonarr://queue":
		endpoint = "/api/v3/queue"
	case "sonarr://system/status":
		endpoint = "/api/v3/system/status"
	case "sonarr://series":
		data, err := p.fetchFromSonarr(ctx, "/api/v3/series")
		if err != nil {
			return "", err
		}
		limited, err := filterSeriesList([]byte(data), "")
		if err != nil {
			return data, nil
		}
		return string(limited), nil
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromSonarr(ctx, endpoint)
}

func (p *Provider) fetchFromSonarr(ctx context.Context, apiPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
	pruned, err := provider.PruneJSON([]byte(content), []string{"images", "overview", "seasons", "network", "certification", "ratings"})
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
			Name:        "check_series_downloads",
			Description: "Checks the Sonarr queue for active TV series downloads and their status.",
			Arguments:   []*mcp.PromptArgument{},
		},
	}, nil
}

func (p *Provider) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "check_series_downloads" {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please check the sonarr://queue resource and tell me what TV series are currently downloading and their progress.",
					},
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		*mcphelper.NewTool("search_sonarr_series", "Search the Sonarr library for tracked TV series. Returns matching series pruned of heavy metadata. Omit the query to get the first 25 series alphabetically.", map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Case-insensitive title search term. Omit to list the first 25 series.",
			},
		}),
	}, nil
}

func (p *Provider) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if name == "search_sonarr_series" {
		data, err := p.fetchFromSonarr(ctx, "/api/v3/series")
		if err != nil {
			return mcphelper.ErrorResult(fmt.Errorf("failed to fetch series: %w", err)), nil
		}
		query, _ := arguments["query"].(string)
		result, err := filterSeriesList([]byte(data), query)
		if err != nil {
			return mcphelper.ErrorResult(fmt.Errorf("failed to filter series: %w", err)), nil
		}
		return mcphelper.TextResult(string(result)), nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func filterSeriesList(data []byte, query string) ([]byte, error) {
	var series []map[string]interface{}
	if err := json.Unmarshal(data, &series); err != nil {
		return data, err
	}
	q := strings.ToLower(query)
	var matched []map[string]interface{}
	for _, s := range series {
		title, _ := s["title"].(string)
		if q != "" && !strings.Contains(strings.ToLower(title), q) {
			continue
		}
		matched = append(matched, s)
		if q == "" && len(matched) >= 25 {
			break
		}
	}
	if matched == nil {
		matched = []map[string]interface{}{}
	}
	return json.MarshalIndent(matched, "", "  ")
}
