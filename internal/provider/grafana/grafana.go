package grafana

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
	mcphelper "homelab-mcp/internal/mcp"
)

type Provider struct {
	baseURL string
	apiKey  string
	user    string
	pass    string
	client  *http.Client
}

func NewProvider(baseURL, apiKey, user, pass string) *Provider {
	return &Provider{
		baseURL: baseURL,
		apiKey:  apiKey,
		user:    user,
		pass:    pass,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func NewProviderFromEnv() *Provider {
	return NewProvider(
		os.Getenv("GRAFANA_URL"),
		os.Getenv("GRAFANA_API_KEY"),
		os.Getenv("GRAFANA_USER"),
		os.Getenv("GRAFANA_PASS"),
	)
}

func (p *Provider) Name() string { return "grafana" }

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:         "grafana://dashboards",
			Name:        "Grafana Dashboards",
			Description: "All Grafana dashboards with their UIDs, folders, and tags.",
			MIMEType:    "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	if uri == "grafana://dashboards" {
		return p.listDashboards("")
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
			"list_grafana_dashboards",
			"List Grafana dashboards. Optionally filter by title keyword. Returns UID, title, folder, tags, and URL for each dashboard.",
			map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Optional title search keyword to filter dashboards",
				},
			},
		),
		*mcphelper.NewTool(
			"get_grafana_dashboard",
			"Get a Grafana dashboard's panels, including each panel's title, type, and underlying query expressions. Use the UID from list_grafana_dashboards.",
			map[string]interface{}{
				"uid": map[string]interface{}{
					"type":        "string",
					"description": "The dashboard UID",
					"required":    true,
				},
			},
		),
		*mcphelper.NewTool(
			"get_grafana_alerts",
			"Get currently firing Grafana alerts with their labels, state, and annotations.",
			nil,
		),
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "list_grafana_dashboards":
		query, _ := arguments["query"].(string)
		result, err := p.listDashboards(query)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "get_grafana_dashboard":
		uid, ok := arguments["uid"].(string)
		if !ok || uid == "" {
			return mcphelper.ErrorResult(fmt.Errorf("uid is required")), nil
		}
		result, err := p.getDashboard(uid)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "get_grafana_alerts":
		result, err := p.getFiringAlerts()
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *Provider) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	u := p.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	} else if p.user != "" {
		req.SetBasicAuth(p.user, p.pass)
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (p *Provider) doRequest(req *http.Request) ([]byte, int, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// dashboardSummary is what we surface for list operations.
type dashboardSummary struct {
	UID    string   `json:"uid"`
	Title  string   `json:"title"`
	Folder string   `json:"folderTitle,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	URL    string   `json:"url,omitempty"`
}

func (p *Provider) listDashboards(query string) (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("GRAFANA_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	path := "/api/search?type=dash-db&limit=100"
	if query != "" {
		path += "&query=" + url.QueryEscape(query)
	}

	req, err := p.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("grafana unreachable: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("grafana returned HTTP %d: %s", status, string(data))
	}

	var raw []struct {
		UID         string   `json:"uid"`
		Title       string   `json:"title"`
		FolderTitle string   `json:"folderTitle"`
		Tags        []string `json:"tags"`
		URL         string   `json:"url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("failed to parse grafana response: %w", err)
	}

	summaries := make([]dashboardSummary, 0, len(raw))
	for _, d := range raw {
		summaries = append(summaries, dashboardSummary{
			UID:    d.UID,
			Title:  d.Title,
			Folder: d.FolderTitle,
			Tags:   d.Tags,
			URL:    p.baseURL + d.URL,
		})
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"dashboards": summaries,
		"count":      len(summaries),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// panelSummary strips layout/style noise and keeps only what's useful for analysis.
type panelSummary struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"`
	Targets     []json.RawMessage `json:"targets,omitempty"`
}

// noiseKeys to strip from panel targets (datasource refs, visual config, etc.)
var panelNoiseKeys = []string{
	"gridPos", "fieldConfig", "options", "transformations", "thresholds",
	"overrides", "links", "tooltip", "legend", "fill", "linewidth",
	"aliasColors", "seriesOverrides", "timeRegions", "repeat",
	"repeatIteration", "repeatPanelId", "scopedVars", "maxDataPoints",
	"interval", "cacheTimeout", "pluginVersion", "datasource",
}

func (p *Provider) getDashboard(uid string) (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("GRAFANA_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := p.newRequest(ctx, http.MethodGet, "/api/dashboards/uid/"+uid)
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("grafana unreachable: %w", err)
	}
	if status == http.StatusNotFound {
		return "", fmt.Errorf("dashboard UID %q not found", uid)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("grafana returned HTTP %d: %s", status, string(data))
	}

	// Parse just enough to extract panels cleanly.
	var raw struct {
		Dashboard struct {
			Title  string `json:"title"`
			Tags   []string `json:"tags"`
			Panels []map[string]json.RawMessage `json:"panels"`
		} `json:"dashboard"`
		Meta struct {
			Slug    string `json:"slug"`
			Version int    `json:"version"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("failed to parse dashboard: %w", err)
	}

	panels := make([]panelSummary, 0, len(raw.Dashboard.Panels))
	for _, panel := range raw.Dashboard.Panels {
		ps := panelSummary{}
		if b, ok := panel["id"]; ok {
			_ = json.Unmarshal(b, &ps.ID)
		}
		if b, ok := panel["title"]; ok {
			_ = json.Unmarshal(b, &ps.Title)
		}
		if b, ok := panel["description"]; ok {
			_ = json.Unmarshal(b, &ps.Description)
		}
		if b, ok := panel["type"]; ok {
			_ = json.Unmarshal(b, &ps.Type)
		}
		if b, ok := panel["targets"]; ok {
			var targets []map[string]json.RawMessage
			if err := json.Unmarshal(b, &targets); err == nil {
				for _, t := range targets {
					// Remove noise keys from each target.
					for _, k := range panelNoiseKeys {
						delete(t, k)
					}
					if cleaned, err := json.Marshal(t); err == nil {
						ps.Targets = append(ps.Targets, json.RawMessage(cleaned))
					}
				}
			}
		}
		panels = append(panels, ps)
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"uid":     uid,
		"title":   raw.Dashboard.Title,
		"tags":    raw.Dashboard.Tags,
		"version": raw.Meta.Version,
		"panels":  panels,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (p *Provider) getFiringAlerts() (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("GRAFANA_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try unified alerting endpoint first.
	req, err := p.newRequest(ctx, http.MethodGet, "/api/alertmanager/grafana/api/v1/alerts")
	if err != nil {
		return "", err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("grafana unreachable: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("grafana alerts returned HTTP %d: %s", status, string(data))
	}

	// Prune verbose annotation/label noise.
	var raw []map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return string(data), nil
	}

	noiseKeys := []string{"fingerprint", "generatorURL", "silencedBy", "inhibitedBy"}
	cleaned := make([]map[string]interface{}, 0, len(raw))
	for _, alert := range raw {
		for _, k := range noiseKeys {
			delete(alert, k)
		}
		cleaned = append(cleaned, alert)
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"alerts": cleaned,
		"count":  len(cleaned),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}
