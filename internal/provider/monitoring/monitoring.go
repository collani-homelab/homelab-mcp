package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Provider struct {
	prometheusURL string
	lokiURL       string
	httpClient    *http.Client
}

func NewProvider() *Provider {
	promURL := os.Getenv("PROMETHEUS_URL")
	if promURL == "" {
		promURL = "http://localhost:9090"
	}
	lokiURL := os.Getenv("LOKI_URL")
	if lokiURL == "" {
		lokiURL = "http://localhost:3100"
	}

	return &Provider{
		prometheusURL: promURL,
		lokiURL:       lokiURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *Provider) Name() string {
	return "monitoring"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{}, nil
}

func (p *Provider) GetResourceContent(ctx context.Context, uri string) (string, error) {
	return "", fmt.Errorf("resource not found: %s", uri)
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{}, nil
}

func (p *Provider) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		{
			Name:        "query_promql",
			Description: "Execute a PromQL query against Prometheus to retrieve metrics.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The PromQL query string",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "query_logql",
			Description: "Execute a LogQL query against Loki to retrieve logs.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The LogQL query string",
					},
					"lookback": map[string]interface{}{
						"type":        "string",
						"description": "How far back to look for logs, as a Prometheus duration string (e.g. '1h', '30m', '24h'). Defaults to '1h'.",
					},
				},
				"required": []string{"query"},
			},
		},
	}, nil
}

func (p *Provider) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "query_promql":
		query, ok := arguments["query"].(string)
		if !ok {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "query must be a string"}}}, nil
		}
		res, err := p.executePrometheusQuery(ctx, query)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Prometheus query failed: %v", err)}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: res}}}, nil

	case "query_logql":
		query, ok := arguments["query"].(string)
		if !ok {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "query must be a string"}}}, nil
		}
		lookback := "1h"
		if lb, ok := arguments["lookback"].(string); ok && lb != "" {
			lookback = lb
		}
		res, err := p.executeLokiQuery(ctx, query, lookback)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Loki query failed: %v", err)}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: res}}}, nil
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *Provider) executePrometheusQuery(ctx context.Context, query string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u, err := url.Parse(fmt.Sprintf("%s/api/v1/query", p.prometheusURL))
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Just return JSON formatted string.
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func (p *Provider) executeLokiQuery(ctx context.Context, query, lookback string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u, err := url.Parse(fmt.Sprintf("%s/loki/api/v1/query_range", p.lokiURL))
	if err != nil {
		return "", err
	}
	now := time.Now()
	duration, err := time.ParseDuration(lookback)
	if err != nil {
		duration = time.Hour
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", now.Add(-duration).UnixNano()))
	q.Set("end", fmt.Sprintf("%d", now.UnixNano()))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(out), nil
}
