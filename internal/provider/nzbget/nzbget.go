package nzbget

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"homelab-mcp/internal/provider"
)

type Provider struct {
	client   *http.Client
	rpcURL   string
	username string
	password string
}

func NewProvider(baseURL, username, password string, skipVerify bool) (*Provider, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Make sure baseURL ends with jsonrpc
	rpcURL := baseURL
	if rpcURL[len(rpcURL)-1] != '/' {
		rpcURL += "/"
	}
	rpcURL += "jsonrpc"

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	return &Provider{
		client:   client,
		rpcURL:   rpcURL,
		username: username,
		password: password,
	}, nil
}

func (p *Provider) Name() string {
	return "NZBGet"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "nzbget://status",
			Name:     "NZBGet Server Status",
			MIMEType: "application/json",
		},
		{
			URI:      "nzbget://listgroups",
			Name:     "NZBGet Active Downloads",
			MIMEType: "application/json",
		},
		{
			URI:      "nzbget://history",
			Name:     "NZBGet Download History",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	var method string
	switch uri {
	case "nzbget://status":
		method = "status"
	case "nzbget://listgroups":
		method = "listgroups"
	case "nzbget://history":
		data, err := p.fetchFromNZBGet("history")
		if err != nil {
			return "", err
		}
		return truncateNZBGetHistory(data, 25), nil
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromNZBGet(method)
}

func truncateNZBGetHistory(data string, limit int) string {
	var envelope struct {
		Version string                   `json:"version"`
		Result  []map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return data
	}
	if len(envelope.Result) > limit {
		envelope.Result = envelope.Result[:limit]
	}
	out, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return data
	}
	return string(out)
}

func (p *Provider) fetchFromNZBGet(method string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload := map[string]interface{}{
		"version": "1.1",
		"method":  method,
		"params":  []interface{}{},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON-RPC payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.rpcURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if p.username != "" || p.password != "" {
		req.SetBasicAuth(p.username, p.password)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	pruned, err := provider.PruneJSON(respBody, []string{"log", "hash", "parameters", "scriptstatuses"})
	if err == nil {
		return string(pruned), nil
	}
	return string(respBody), nil
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{
		{
			Name:        "check_usenet_downloads",
			Description: "Checks NZBGet for active Usenet downloads and their status.",
			Arguments:   []*mcp.PromptArgument{},
		},
	}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "check_usenet_downloads" {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Please check the nzbget://listgroups and nzbget://status resources and tell me what is currently downloading, its progress, and the overall download speed.",
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

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	return nil, fmt.Errorf("tool not found: %s", name)
}
