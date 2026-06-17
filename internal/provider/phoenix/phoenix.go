package phoenix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcphelper "homelab-mcp/internal/mcp"
	"homelab-mcp/internal/provider"
)

const defaultLookback = "24h"

type Provider struct {
	baseURL string
	client  *http.Client
}

func NewProvider(baseURL string) *Provider {
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func NewProviderFromEnv() *Provider {
	return NewProvider(os.Getenv("PHOENIX_URL"))
}

func (p *Provider) Name() string { return "phoenix" }

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:         "phoenix://projects",
			Name:        "Phoenix Projects",
			Description: "All Arize Phoenix trace projects.",
			MIMEType:    "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(ctx context.Context, uri string) (string, error) {
	switch {
	case uri == "phoenix://projects":
		return p.listProjects(ctx)
	case strings.HasPrefix(uri, "phoenix://traces/"):
		project := strings.TrimPrefix(uri, "phoenix://traces/")
		return p.listTraces(ctx, project, defaultLookback, 50)
	case strings.HasPrefix(uri, "phoenix://evaluations/"):
		project := strings.TrimPrefix(uri, "phoenix://evaluations/")
		return p.listSpanAnnotations(ctx, project, defaultLookback, 100)
	}
	return "", fmt.Errorf("resource not found: %s", uri)
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{
		{
			URITemplate: "phoenix://traces/{project}",
			Name:        "Phoenix Recent Traces",
			Description: "Recent traces for a Phoenix project, including span summaries.",
			MIMEType:    "application/json",
		},
		{
			URITemplate: "phoenix://evaluations/{project}",
			Name:        "Phoenix Evaluations",
			Description: "Span evaluation/annotation scores for a Phoenix project.",
			MIMEType:    "application/json",
		},
	}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) { return []mcp.Prompt{}, nil }
func (p *Provider) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		*mcphelper.NewTool(
			"query_phoenix_traces",
			"Query Phoenix LLM spans for a project, filtered by model, status, and time range. Returns span name, model, latency, status, and token counts.",
			map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Phoenix project ID or name",
					"required":    true,
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Optional model name to filter by (matches the llm.model_name span attribute)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Optional span status to filter by: OK, ERROR, or UNSET",
				},
				"lookback": map[string]interface{}{
					"type":        "string",
					"description": "How far back to look, as a Go duration string (e.g. '1h', '30m', '24h'). Defaults to '24h'.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of spans to return. Defaults to 50.",
				},
			},
		),
		*mcphelper.NewTool(
			"get_phoenix_eval_scores",
			"Aggregate Phoenix evaluation/annotation scores per model for a project. Joins the most recent page of LLM spans with their annotations — not exhaustive over the full history.",
			map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Phoenix project ID or name",
					"required":    true,
				},
				"lookback": map[string]interface{}{
					"type":        "string",
					"description": "How far back to look for spans, as a Go duration string (e.g. '1h', '24h'). Defaults to '24h'.",
				},
				"annotation_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional annotation name to filter by (e.g. 'correctness', 'hallucination')",
				},
			},
		),
		*mcphelper.NewTool(
			"get_phoenix_span_errors",
			"Surface failed LLM spans (status ERROR) for a Phoenix project, with error messages and model attribution.",
			map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Phoenix project ID or name",
					"required":    true,
				},
				"lookback": map[string]interface{}{
					"type":        "string",
					"description": "How far back to look, as a Go duration string (e.g. '1h', '24h'). Defaults to '24h'.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of error spans to return. Defaults to 50.",
				},
			},
		),
	}, nil
}

func (p *Provider) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "query_phoenix_traces":
		project, ok := arguments["project"].(string)
		if !ok || project == "" {
			return mcphelper.ErrorResult(fmt.Errorf("project is required")), nil
		}
		model, _ := arguments["model"].(string)
		status, _ := arguments["status"].(string)
		lookback := stringOrDefault(arguments["lookback"], defaultLookback)
		limit := intOrDefault(arguments["limit"], 50)

		result, err := p.querySpans(ctx, project, model, status, lookback, limit)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "get_phoenix_eval_scores":
		project, ok := arguments["project"].(string)
		if !ok || project == "" {
			return mcphelper.ErrorResult(fmt.Errorf("project is required")), nil
		}
		lookback := stringOrDefault(arguments["lookback"], defaultLookback)
		annotationName, _ := arguments["annotation_name"].(string)

		result, err := p.getEvalScores(ctx, project, lookback, annotationName)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil

	case "get_phoenix_span_errors":
		project, ok := arguments["project"].(string)
		if !ok || project == "" {
			return mcphelper.ErrorResult(fmt.Errorf("project is required")), nil
		}
		lookback := stringOrDefault(arguments["lookback"], defaultLookback)
		limit := intOrDefault(arguments["limit"], 50)

		result, err := p.querySpans(ctx, project, "", "ERROR", lookback, limit)
		if err != nil {
			return mcphelper.ErrorResult(err), nil
		}
		return mcphelper.TextResult(result), nil
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

func stringOrDefault(v interface{}, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func intOrDefault(v interface{}, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return def
}

func (p *Provider) newRequest(ctx context.Context, path string, query url.Values) (*http.Request, error) {
	u := p.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
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

func (p *Provider) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("PHOENIX_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := p.newRequest(ctx, path, query)
	if err != nil {
		return nil, err
	}

	data, status, err := p.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("phoenix unreachable: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("phoenix returned HTTP %d: %s", status, string(data))
	}
	return data, nil
}

func lookbackStartTime(lookback string) (string, error) {
	d, err := time.ParseDuration(lookback)
	if err != nil {
		return "", fmt.Errorf("invalid lookback duration %q: %w", lookback, err)
	}
	return time.Now().Add(-d).UTC().Format(time.RFC3339), nil
}

type phoenixProject struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type projectsResponse struct {
	Data []phoenixProject `json:"data"`
}

func (p *Provider) listProjects(ctx context.Context) (string, error) {
	data, err := p.get(ctx, "/v1/projects", url.Values{"limit": {"100"}})
	if err != nil {
		return "", err
	}

	var raw projectsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("failed to parse phoenix projects response: %w", err)
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"projects": raw.Data,
		"count":    len(raw.Data),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type spanContext struct {
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
}

type phoenixSpan struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Context       spanContext            `json:"context"`
	SpanKind      string                 `json:"span_kind"`
	StartTime     string                 `json:"start_time"`
	EndTime       string                 `json:"end_time"`
	StatusCode    string                 `json:"status_code"`
	StatusMessage string                 `json:"status_message"`
	Attributes    map[string]interface{} `json:"attributes"`
}

type spansResponse struct {
	Data []phoenixSpan `json:"data"`
}

// attributeNoiseKeys strips large free-text blobs from span attributes,
// keeping the structured fields (model, token counts, etc.) useful for
// aggregation and error triage.
var attributeNoiseKeys = []string{"input.value", "output.value", "llm.input_messages", "llm.output_messages"}

func (p *Provider) fetchSpans(ctx context.Context, project, model, status, lookback string, limit int) ([]phoenixSpan, error) {
	startTime, err := lookbackStartTime(lookback)
	if err != nil {
		return nil, err
	}

	query := url.Values{
		"start_time": {startTime},
		"span_kind":  {"LLM"},
		"limit":      {fmt.Sprintf("%d", limit)},
	}
	if model != "" {
		query.Add("attribute", "llm.model_name:"+model)
	}
	if status != "" {
		query.Add("status_code", strings.ToUpper(status))
	}

	data, err := p.get(ctx, "/v1/projects/"+url.PathEscape(project)+"/spans", query)
	if err != nil {
		return nil, err
	}

	var raw spansResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse phoenix spans response: %w", err)
	}
	return raw.Data, nil
}

func (p *Provider) querySpans(ctx context.Context, project, model, status, lookback string, limit int) (string, error) {
	spans, err := p.fetchSpans(ctx, project, model, status, lookback, limit)
	if err != nil {
		return "", err
	}

	pruned := make([]map[string]interface{}, 0, len(spans))
	for _, s := range spans {
		attrs, err := pruneAttributes(s.Attributes)
		if err != nil {
			return "", err
		}
		pruned = append(pruned, map[string]interface{}{
			"id":             s.ID,
			"name":           s.Name,
			"trace_id":       s.Context.TraceID,
			"span_id":        s.Context.SpanID,
			"span_kind":      s.SpanKind,
			"start_time":     s.StartTime,
			"end_time":       s.EndTime,
			"status_code":    s.StatusCode,
			"status_message": s.StatusMessage,
			"attributes":     attrs,
		})
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"spans": pruned,
		"count": len(pruned),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func pruneAttributes(attrs map[string]interface{}) (interface{}, error) {
	raw, err := json.Marshal(attrs)
	if err != nil {
		return nil, err
	}
	pruned, err := provider.PruneJSON(raw, attributeNoiseKeys)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(pruned, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type traceData struct {
	ID                   string          `json:"id"`
	TraceID              string          `json:"trace_id"`
	StartTime            string          `json:"start_time"`
	EndTime              string          `json:"end_time"`
	TokenCountPrompt     int             `json:"token_count_prompt"`
	TokenCountCompletion int             `json:"token_count_completion"`
	TokenCountTotal      int             `json:"token_count_total"`
	Spans                json.RawMessage `json:"spans,omitempty"`
}

type tracesResponse struct {
	Data []traceData `json:"data"`
}

func (p *Provider) listTraces(ctx context.Context, project, lookback string, limit int) (string, error) {
	startTime, err := lookbackStartTime(lookback)
	if err != nil {
		return "", err
	}

	query := url.Values{
		"start_time":    {startTime},
		"limit":         {fmt.Sprintf("%d", limit)},
		"sort":          {"start_time"},
		"order":         {"desc"},
		"include_spans": {"true"},
	}

	data, err := p.get(ctx, "/v1/projects/"+url.PathEscape(project)+"/traces", query)
	if err != nil {
		return "", err
	}

	var raw tracesResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("failed to parse phoenix traces response: %w", err)
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"traces": raw.Data,
		"count":  len(raw.Data),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type annotationResult struct {
	Label       *string  `json:"label"`
	Score       *float64 `json:"score"`
	Explanation *string  `json:"explanation"`
}

type spanAnnotation struct {
	ID            string            `json:"id"`
	CreatedAt     string            `json:"created_at"`
	Name          string            `json:"name"`
	AnnotatorKind string            `json:"annotator_kind"`
	Result        *annotationResult `json:"result"`
	SpanID        string            `json:"span_id"`
}

type spanAnnotationsResponse struct {
	Data []spanAnnotation `json:"data"`
}

// fetchSpanAnnotations looks up annotations for a set of OTel span IDs.
// Phoenix's span_annotations endpoint requires at least one of span_ids or
// identifier — it is not a free listing endpoint — so an empty spanIDs slice
// short-circuits to an empty result instead of issuing a guaranteed-422 call.
func (p *Provider) fetchSpanAnnotations(ctx context.Context, project string, spanIDs []string, annotationName string, limit int) ([]spanAnnotation, error) {
	if len(spanIDs) == 0 {
		return nil, nil
	}

	query := url.Values{"limit": {fmt.Sprintf("%d", limit)}}
	for _, id := range spanIDs {
		query.Add("span_ids", id)
	}
	if annotationName != "" {
		query.Add("include_annotation_names", annotationName)
	}

	data, err := p.get(ctx, "/v1/projects/"+url.PathEscape(project)+"/span_annotations", query)
	if err != nil {
		return nil, err
	}

	var raw spanAnnotationsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse phoenix span annotations response: %w", err)
	}
	return raw.Data, nil
}

func (p *Provider) listSpanAnnotations(ctx context.Context, project, lookback string, limit int) (string, error) {
	spans, err := p.fetchSpans(ctx, project, "", "", lookback, 500)
	if err != nil {
		return "", err
	}
	spanIDs := make([]string, len(spans))
	for i, s := range spans {
		spanIDs[i] = s.Context.SpanID
	}

	annotations, err := p.fetchSpanAnnotations(ctx, project, spanIDs, "", limit)
	if err != nil {
		return "", err
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"annotations": annotations,
		"count":       len(annotations),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// scoreAggregate accumulates scores for a single (model, annotation name) pair.
type scoreAggregate struct {
	Model          string  `json:"model"`
	AnnotationName string  `json:"annotation_name"`
	Count          int     `json:"count"`
	ScoreSum       float64 `json:"-"`
	AverageScore   float64 `json:"average_score"`
}

func (p *Provider) getEvalScores(ctx context.Context, project, lookback, annotationName string) (string, error) {
	spans, err := p.fetchSpans(ctx, project, "", "", lookback, 500)
	if err != nil {
		return "", err
	}

	modelBySpanID := make(map[string]string, len(spans))
	spanIDs := make([]string, len(spans))
	for i, s := range spans {
		model, _ := s.Attributes["llm.model_name"].(string)
		if model == "" {
			model = "unknown"
		}
		modelBySpanID[s.Context.SpanID] = model
		spanIDs[i] = s.Context.SpanID
	}

	annotations, err := p.fetchSpanAnnotations(ctx, project, spanIDs, annotationName, 1000)
	if err != nil {
		return "", err
	}

	aggregates := make(map[string]*scoreAggregate)
	for _, a := range annotations {
		if a.Result == nil || a.Result.Score == nil {
			continue
		}
		model, known := modelBySpanID[a.SpanID]
		if !known {
			model = "unknown"
		}
		key := model + "|" + a.Name
		agg, ok := aggregates[key]
		if !ok {
			agg = &scoreAggregate{Model: model, AnnotationName: a.Name}
			aggregates[key] = agg
		}
		agg.Count++
		agg.ScoreSum += *a.Result.Score
	}

	results := make([]scoreAggregate, 0, len(aggregates))
	for _, agg := range aggregates {
		agg.AverageScore = agg.ScoreSum / float64(agg.Count)
		results = append(results, *agg)
	}

	out, err := json.MarshalIndent(map[string]interface{}{
		"scores": results,
		"count":  len(results),
		"note":   "bounded to the most recent page of spans/annotations, not exhaustive",
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}
