package unifi

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
	return "UniFi"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "unifi://clients",
			Name:     "UniFi Active Clients",
			MIMEType: "application/json",
		},
		{
			URI:      "unifi://devices",
			Name:     "UniFi Devices",
			MIMEType: "application/json",
		},
		{
			URI:      "unifi://network/health",
			Name:     "UniFi Network Health",
			MIMEType: "application/json",
		},
		{
			URI:      "unifi://switches/poe",
			Name:     "UniFi PoE Status",
			MIMEType: "application/json",
		},
		{
			URI:      "unifi://network/alarms",
			Name:     "UniFi Network Alarms",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	var apiPath string
	switch uri {
	case "unifi://clients":
		apiPath = "stat/sta"
	case "unifi://devices", "unifi://switches/poe":
		apiPath = "stat/device"
	case "unifi://network/health":
		apiPath = "stat/health"
	case "unifi://network/alarms":
		apiPath = "rest/alarm"
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromUniFi(apiPath)
}

func (p *Provider) fetchFromUniFi(apiPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var content string
	var err error

	// Try with /proxy/network/api/s/default prefix first
	endpoint := fmt.Sprintf("%s/proxy/network/api/s/default/%s", p.baseURL, apiPath)
	content, err = p.doRequest(ctx, endpoint)
	if err != nil {
		// Fallback to /api/s/default prefix
		fallbackEndpoint := fmt.Sprintf("%s/api/s/default/%s", p.baseURL, apiPath)
		content, err = p.doRequest(ctx, fallbackEndpoint)
		if err != nil {
			return "", err
		}
	}

	pruned, err := provider.PruneJSON([]byte(content), []string{"_id", "site_id", "oui", "fingerprint", "_is_guest_by_uap", "tx_bytes-r", "rx_bytes-r"})
	if err == nil {
		return string(pruned), nil
	}
	return content, nil
}

func (p *Provider) doRequest(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-KEY", p.apiKey)

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

	return string(bodyBytes), nil
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{
		{
			Name:        "troubleshoot_client",
			Description: "Helps diagnose why a specific MAC address might be having issues.",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "mac",
					Description: "The MAC address of the client to troubleshoot",
					Required:    true,
				},
			},
		},
	}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if name == "troubleshoot_client" {
		mac := arguments["mac"]
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("I need to troubleshoot a client with MAC address %s. Please check the unifi://clients resource for this MAC, check its experience score, and suggest what might be wrong.", mac),
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
