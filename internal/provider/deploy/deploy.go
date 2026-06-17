package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcphelper "homelab-mcp/internal/mcp"
)

const ghOrg = "collani-homelab"

type Provider struct {
	webhookURL string
	ghToken    string
	client     *http.Client
}

func NewProvider(webhookURL, ghToken string) *Provider {
	return &Provider{
		webhookURL: webhookURL,
		ghToken:    ghToken,
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
		*mcphelper.NewTool(
			"get_deploy_status",
			"Returns the status of the most recent deploy workflow run for a given repo. Use after pushing code to check if the GHA deploy completed successfully.",
			map[string]interface{}{
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name within collani-homelab org (e.g. 'homelab-mcp', 'homelab-context', 'homelab-platform')",
				},
				"workflow": map[string]interface{}{
					"type":        "string",
					"description": "Workflow filename to query (default: 'deploy.yml')",
				},
			},
		),
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "redeploy_service":
		serviceName, ok := arguments["service_name"].(string)
		if !ok || serviceName == "" {
			return mcphelper.ErrorResult(fmt.Errorf("service_name is required")), nil
		}
		return p.redeployService(serviceName), nil
	case "get_deploy_status":
		repo, ok := arguments["repo"].(string)
		if !ok || repo == "" {
			return mcphelper.ErrorResult(fmt.Errorf("repo is required")), nil
		}
		workflow := "deploy.yml"
		if w, ok := arguments["workflow"].(string); ok && w != "" {
			workflow = w
		}
		return p.getDeployStatus(repo, workflow), nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *Provider) redeployService(serviceName string) *mcp.CallToolResult {
	if p.webhookURL == "" {
		return mcphelper.ErrorResult(fmt.Errorf("deploy webhook not configured: DEPLOY_WEBHOOK_URL is not set"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/deploy/%s", p.webhookURL, url.PathEscape(serviceName))
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

type deployStatus struct {
	Repo        string `json:"repo"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	RunURL      string `json:"run_url"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at,omitempty"`
}

func (p *Provider) getDeployStatus(repo, workflow string) *mcp.CallToolResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/actions/workflows/%s/runs?per_page=1&branch=main",
		ghOrg, repo, workflow,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to create request: %w", err))
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if p.ghToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.ghToken)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("GitHub API unreachable: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return mcphelper.ErrorResult(fmt.Errorf("GitHub API error (HTTP %d): %s", resp.StatusCode, string(body)))
	}

	var payload struct {
		WorkflowRuns []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
			RunStarted string `json:"run_started_at"`
			UpdatedAt  string `json:"updated_at"`
		} `json:"workflow_runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to decode GitHub response: %w", err))
	}

	if len(payload.WorkflowRuns) == 0 {
		return mcphelper.TextResult(fmt.Sprintf(`{"repo":%q,"status":"not_found","conclusion":"","run_url":"","started_at":"","completed_at":""}`, repo))
	}

	run := payload.WorkflowRuns[0]
	completedAt := ""
	if run.Status == "completed" {
		completedAt = run.UpdatedAt
	}

	status := deployStatus{
		Repo:        repo,
		Status:      run.Status,
		Conclusion:  run.Conclusion,
		RunURL:      run.HTMLURL,
		StartedAt:   run.RunStarted,
		CompletedAt: completedAt,
	}
	out, _ := json.Marshal(status)
	return mcphelper.TextResult(string(out))
}
