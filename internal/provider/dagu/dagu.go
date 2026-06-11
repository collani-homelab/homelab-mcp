package dagu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcphelper "homelab-mcp/internal/mcp"
)

type Provider struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

func NewProvider(baseURL, apiKey string) *Provider {
	return &Provider{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func NewProviderFromEnv() *Provider {
	return NewProvider(os.Getenv("DAGU_API_URL"), os.Getenv("DAGU_API_KEY"))
}

func (p *Provider) Name() string { return "dagu" }

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:         "dagu://dags",
			Name:        "Dagu DAG List",
			Description: "All Dagu DAGs with their current execution status.",
			MIMEType:    "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	if uri == "dagu://dags" {
		return p.listDAGs()
	}
	return "", fmt.Errorf("resource not found: %s", uri)
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
			"list_dags",
			"List all Dagu DAGs with their current execution status (running, success, failed, etc.).",
			nil,
		),
		*mcphelper.NewTool(
			"get_dag",
			"Get detailed status and step breakdown for a named Dagu DAG, including the most recent run.",
			map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The DAG name (filename without .yaml extension)",
					"required":    true,
				},
			},
		),
		*mcphelper.NewTool(
			"trigger_dag",
			"Start a Dagu DAG execution. Optionally pass parameters to the DAG.",
			map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The DAG name to trigger",
					"required":    true,
				},
				"params": map[string]interface{}{
					"type":        "string",
					"description": "Optional parameters string passed to the DAG (e.g. 'key=value')",
				},
			},
		),
		*mcphelper.NewTool(
			"stop_dag",
			"Stop a currently running Dagu DAG execution.",
			map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The DAG file name (without .yaml extension) to stop",
					"required":    true,
				},
				"dag_run_id": map[string]interface{}{
					"type":        "string",
					"description": "The specific dagRunId to stop. Omit to stop the latest running execution.",
				},
			},
		),
		*mcphelper.NewTool(
			"retry_dag",
			"Retry a failed Dagu DAG run from the point of failure.",
			map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The DAG file name (without .yaml extension) to retry",
					"required":    true,
				},
				"dag_run_id": map[string]interface{}{
					"type":        "string",
					"description": "The dagRunId of the failed run to retry",
					"required":    true,
				},
			},
		),
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "list_dags":
		result, err := p.listDAGs()
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "get_dag":
		dagName, ok := arguments["name"].(string)
		if !ok || dagName == "" {
			return mcphelper.ErrorResult(fmt.Errorf("name is required")), nil
		}
		result, err := p.getDAG(dagName)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "trigger_dag":
		dagName, ok := arguments["name"].(string)
		if !ok || dagName == "" {
			return mcphelper.ErrorResult(fmt.Errorf("name is required")), nil
		}
		params, _ := arguments["params"].(string)
		result, err := p.triggerDAG(dagName, params)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "stop_dag":
		dagName, ok := arguments["name"].(string)
		if !ok || dagName == "" {
			return mcphelper.ErrorResult(fmt.Errorf("name is required")), nil
		}
		dagRunID, ok := arguments["dag_run_id"].(string)
		if !ok || dagRunID == "" {
			return mcphelper.ErrorResult(fmt.Errorf("dag_run_id is required to stop a specific run")), nil
		}
		result, err := p.dagRunAction(dagName, dagRunID, "stop")
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "retry_dag":
		dagName, ok := arguments["name"].(string)
		if !ok || dagName == "" {
			return mcphelper.ErrorResult(fmt.Errorf("name is required")), nil
		}
		dagRunID, ok := arguments["dag_run_id"].(string)
		if !ok || dagRunID == "" {
			return mcphelper.ErrorResult(fmt.Errorf("dag_run_id is required for retry")), nil
		}
		result, err := p.dagRunAction(dagName, dagRunID, "retry")
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *Provider) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("%s%s", p.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (p *Provider) doRequest(req *http.Request) ([]byte, int, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// dagSummary is a pruned representation of a DAG for list output.
type dagSummary struct {
	Name        string `json:"name"`
	FileName    string `json:"fileName"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	DAGRunID    string `json:"dagRunId,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
	FinishedAt  string `json:"finishedAt,omitempty"`
	Suspended   bool   `json:"suspended,omitempty"`
}

func (p *Provider) listDAGs() (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("DAGU_API_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := p.newRequest(ctx, http.MethodGet, "/api/v1/dags", nil)
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("dagu unreachable: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("dagu returned HTTP %d: %s", status, string(data))
	}

	var raw struct {
		DAGs []struct {
			DAG struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"dag"`
			FileName     string `json:"fileName"`
			Suspended    bool   `json:"suspended"`
			LatestDAGRun struct {
				DAGRunID    string `json:"dagRunId"`
				StatusLabel string `json:"statusLabel"`
				StartedAt   string `json:"startedAt"`
				FinishedAt  string `json:"finishedAt"`
			} `json:"latestDAGRun"`
		} `json:"dags"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("failed to parse dagu response: %w", err)
	}

	summaries := make([]dagSummary, 0, len(raw.DAGs))
	for _, entry := range raw.DAGs {
		summaries = append(summaries, dagSummary{
			Name:        entry.DAG.Name,
			FileName:    entry.FileName,
			Description: entry.DAG.Description,
			Status:      entry.LatestDAGRun.StatusLabel,
			DAGRunID:    entry.LatestDAGRun.DAGRunID,
			StartedAt:   entry.LatestDAGRun.StartedAt,
			FinishedAt:  entry.LatestDAGRun.FinishedAt,
			Suspended:   entry.Suspended,
		})
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"dags":  summaries,
		"count": len(summaries),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (p *Provider) getDAG(name string) (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("DAGU_API_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := p.newRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/dags/%s", name), nil)
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("dagu unreachable: %w", err)
	}
	if status == http.StatusNotFound {
		return "", fmt.Errorf("DAG %q not found", name)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("dagu returned HTTP %d: %s", status, string(data))
	}

	// Prune noisy fields before returning to keep token count down.
	noiseKeys := []string{"log", "stdout", "stderr", "output"}
	pruned, err := pruneJSON(data, noiseKeys)
	if err != nil {
		return string(data), nil
	}
	return string(pruned), nil
}

func (p *Provider) triggerDAG(name, params string) (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("DAGU_API_URL is not configured")
	}

	payload := map[string]interface{}{}
	if params != "" {
		payload["params"] = params
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := p.newRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/dags/%s/start", name), bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("dagu unreachable: %w", err)
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return "", fmt.Errorf("trigger failed (HTTP %d): %s", status, string(data))
	}

	return fmt.Sprintf("DAG %q triggered successfully.", name), nil
}

func (p *Provider) dagRunAction(dagName, dagRunID, action string) (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("DAGU_API_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/v1/dag-runs/%s/%s/%s", dagName, dagRunID, action)
	req, err := p.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("dagu unreachable: %w", err)
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return "", fmt.Errorf("%s failed (HTTP %d): %s", action, status, string(data))
	}

	return fmt.Sprintf("DAG %q run %q: %s succeeded.", dagName, dagRunID, action), nil
}

// pruneJSON removes specified keys from a JSON byte slice to reduce token usage.
func pruneJSON(data []byte, noiseKeys []string) ([]byte, error) {
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	pruned := pruneValue(parsed, noiseKeys)
	return json.MarshalIndent(pruned, "", "  ")
}

func pruneValue(v interface{}, noiseKeys []string) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k := range val {
			isNoise := false
			for _, nk := range noiseKeys {
				if k == nk {
					isNoise = true
					break
				}
			}
			if isNoise {
				delete(val, k)
			} else {
				val[k] = pruneValue(val[k], noiseKeys)
			}
		}
	case []interface{}:
		for i, child := range val {
			val[i] = pruneValue(child, noiseKeys)
		}
	}
	return v
}
