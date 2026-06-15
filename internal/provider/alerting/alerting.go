package alerting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Provider struct {
	httpClient *http.Client
}

func NewProvider() *Provider {
	return &Provider{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *Provider) Name() string {
	return "alerting"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	return "", fmt.Errorf("resource not found: %s", uri)
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		{
			Name:        "send_notification",
			Description: "Send a notification alert via ntfy.sh to homelab administrators.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "The message content of the notification alert.",
					},
				},
				"required": []string{"message"},
			},
		},
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if name == "send_notification" {
		message, ok := arguments["message"].(string)
		if !ok {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "message must be a string"}}}, nil
		}

		err := p.sendNtfyAlert(message)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to send notification: %v", err)}}}, nil
		}

		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Notification sent successfully."}}}, nil
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *Provider) sendNtfyAlert(message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	base := os.Getenv("NTFY_URL")
	if base == "" {
		base = "http://localhost:9099"
	}
	url := base + "/homelab_alerts"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(message))
	if err != nil {
		return err
	}

	// Just a standard text/plain post. Ntfy accepts raw text bodies.
	req.Header.Set("Content-Type", "text/plain")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
