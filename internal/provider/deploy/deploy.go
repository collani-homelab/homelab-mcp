package deploy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcphelper "homelab-mcp/internal/mcp"
)

type Provider struct {
	webhookURL string
	client     *http.Client
}

func NewProvider(webhookURL string) *Provider {
	return &Provider{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *Provider) Name() string { return "Deploy" }

func (p *Provider) GetResources() ([]mcp.Resource, error) { return []mcp.Resource{}, nil }
func (p *Provider) GetResourceContent(uri string) (string, error) {
	return "", fmt.Errorf("unsupported resource URI: %s", uri)
}
func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}
func (p *Provider) GetPrompts() ([]mcp.Prompt, error) { return []mcp.Prompt{}, nil }
func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		*mcphelper.NewTool(
			"redeploy_service",
			"Triggers a redeployment of a named service on the SRE machine. Pulls the latest image from the registry and restarts the service. Use after pushing a new image via CI/CD or when a service needs a forced restart.",
			map[string]interface{}{
				"service_name": map[string]interface{}{
					"type":        "string",
					"description": "The service name as defined in sre-machine.json (e.g. 'homelab-mcp', 'homelab-monitoring', 'homelab-context')",
				},
			},
		),
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if name == "redeploy_service" {
		serviceName, ok := arguments["service_name"].(string)
		if !ok || serviceName == "" {
			return mcphelper.ErrorResult(fmt.Errorf("service_name is required")), nil
		}
		return p.redeployService(serviceName), nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *Provider) redeployService(serviceName string) *mcp.CallToolResult {
	if p.webhookURL == "" {
		return mcphelper.ErrorResult(fmt.Errorf("deploy webhook not configured: DEPLOY_WEBHOOK_URL is not set"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/deploy/%s", p.webhookURL, serviceName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to create request: %w", err))
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("deploy webhook unreachable: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return mcphelper.ErrorResult(fmt.Errorf("deploy failed (HTTP %d): %s", resp.StatusCode, string(body)))
	}

	return mcphelper.TextResult(fmt.Sprintf("Service '%s' redeployed successfully.", serviceName))
}
