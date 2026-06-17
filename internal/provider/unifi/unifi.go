package unifi

import (
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

func (p *Provider) GetResourceContent(ctx context.Context, uri string) (string, error) {
	var apiPath string
	switch uri {
	case "unifi://clients":
		apiPath = "stat/sta"
	case "unifi://devices":
		apiPath = "stat/device"
	case "unifi://network/health":
		apiPath = "stat/health"
	case "unifi://network/alarms":
		apiPath = "rest/alarm"
	case "unifi://switches/poe":
		raw, err := p.fetchFromUniFi(ctx, "stat/device")
		if err != nil {
			return "", err
		}
		return filterPoEDevices(raw)
	default:
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	return p.fetchFromUniFi(ctx, apiPath)
}

// filterPoEDevices extracts PoE-capable switches and their per-port power data
// from a raw stat/device response, discarding non-switch devices and non-PoE ports.
func filterPoEDevices(raw string) (string, error) {
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return raw, err
	}

	var switches []map[string]interface{}
	for _, device := range resp.Data {
		if devType, _ := device["type"].(string); devType != "usw" {
			continue
		}
		portTable, _ := device["port_table"].([]interface{})
		var poePorts []map[string]interface{}
		for _, entry := range portTable {
			port, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if _, hasPoe := port["poe_mode"]; !hasPoe {
				continue
			}
			poePorts = append(poePorts, map[string]interface{}{
				"port_idx":    port["port_idx"],
				"name":        port["name"],
				"poe_mode":    port["poe_mode"],
				"poe_enable":  port["poe_enable"],
				"poe_power":   port["poe_power"],
				"poe_voltage": port["poe_voltage"],
				"poe_current": port["poe_current"],
			})
		}
		if len(poePorts) == 0 {
			continue
		}
		switches = append(switches, map[string]interface{}{
			"name":       device["name"],
			"mac":        device["mac"],
			"model":      device["model"],
			"ip":         device["ip"],
			"port_table": poePorts,
		})
	}

	if switches == nil {
		switches = []map[string]interface{}{}
	}
	out, err := json.MarshalIndent(switches, "", "  ")
	if err != nil {
		return raw, err
	}
	return string(out), nil
}

// defaultDevicePruneKeys are noisy fields stripped from stat/device responses.
var defaultDevicePruneKeys = []string{
	"_id", "site_id", "oui", "fingerprint", "_is_guest_by_uap", "tx_bytes-r", "rx_bytes-r",
}

func (p *Provider) fetchFromUniFi(ctx context.Context, apiPath string) (string, error) {
	return p.fetchFromUniFiPruned(ctx, apiPath, defaultDevicePruneKeys)
}

func (p *Provider) fetchFromUniFiPruned(ctx context.Context, apiPath string, pruneKeys []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
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

	pruned, err := provider.PruneJSON([]byte(content), pruneKeys)
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
	defer func() { _ = resp.Body.Close() }()

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

func (p *Provider) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
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
	return []mcp.Tool{
		{
			Name:        "get_unifi_devices",
			Description: "Retrieves a pruned list of UniFi infrastructure devices (APs, switches, gateways) from the network controller.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_unifi_clients",
			Description: "Retrieves a pruned list of active client devices (stations) connected to the UniFi network, including hostname, IP, MAC, VLAN, uplink AP/switch, signal strength, and satisfaction score.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}, nil
}

func (p *Provider) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "get_unifi_devices":
		content, err := p.fetchFromUniFi(ctx, "stat/device")
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to fetch unifi devices: %v", err)}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: content}}}, nil

	case "get_unifi_clients":
		content, err := p.fetchFromUniFiPruned(ctx, "stat/sta", []string{
			// Internal IDs and tracking noise
			"_id", "site_id", "user_id", "usergroup_id",
			"network_id", "last_connection_network_id",
			"network_members_group_ids",
			// Fingerprint noise
			"oui", "fingerprint", "fingerprint_engine_version", "fingerprint_source",
			"dev_family", "dev_cat", "dev_id", "dev_vendor", "os_name", "confidence",
			// Internal per-device seen/uptime tracking
			"_uptime_by_ugw", "_uptime_by_usw", "_uptime_by_uap",
			"_last_seen_by_ugw", "_last_seen_by_usw", "_last_seen_by_uap",
			"_is_guest_by_ugw", "_is_guest_by_usw", "_is_guest_by_uap",
			"_last_reachable_by_gw",
			// High-frequency byte counters (available via Prometheus instead)
			"tx_bytes", "rx_bytes", "tx_packets", "rx_packets",
			"tx_bytes-r", "rx_bytes-r",
			"wired-tx_bytes", "wired-rx_bytes",
			"wired-tx_packets", "wired-rx_packets",
			"wifi_tx_attempts", "wifi_tx_dropped", "wifi_tx_retries_percentage",
			// Misc noise
			"eagerly_discovered", "qos_policy_applied", "last_1x_identity",
			"satisfaction_avg", "anomalies",
		})
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to fetch unifi clients: %v", err)}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: content}}}, nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}
