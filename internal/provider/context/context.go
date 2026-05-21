package ragcontext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	mcphelper "homelab-mcp/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Provider struct {
	baseURL string
	client  *http.Client
}

func NewProvider(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = os.Getenv("HOMELAB_CONTEXT_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8081"
		}
	}
	return &Provider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *Provider) Name() string {
	return "homelab-context"
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
		*mcphelper.NewTool("query_knowledge", "Search the homelab context database (RAG) for relevant architecture, configurations, and historical decisions.", map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search query to look up in the context database.",
				"required":    true,
			},
			"top_k": map[string]interface{}{
				"type":        "integer",
				"description": "The maximum number of results to return (default 5).",
			},
			"filter_type": map[string]interface{}{
				"type":        "string",
				"description": "Optional category filter: architecture, roadmap, artifact, config.",
			},
		}),
		*mcphelper.NewTool("index_document", "Index a document (text block, architecture policy, or configuration metadata) into the homelab context database.", map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The raw text content of the document/chunk to be indexed.",
				"required":    true,
			},
			"source": map[string]interface{}{
				"type":        "string",
				"description": "The source/file identifier (e.g. /home/wcollani/repos/homelab/src/meta/GLOBAL_CLAUDE.md).",
				"required":    true,
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "The category of the document: architecture, roadmap, artifact, config.",
				"required":    true,
			},
		}),
	}, nil
}

func (p *Provider) CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case "query_knowledge":
		query, ok := arguments["query"].(string)
		if !ok || query == "" {
			return mcphelper.ErrorResult(fmt.Errorf("missing or invalid query parameter")), nil
		}
		topK := 5
		if k, ok := arguments["top_k"].(float64); ok {
			topK = int(k)
		}
		var filterType string
		if f, ok := arguments["filter_type"].(string); ok {
			filterType = f
		}
		return p.queryKnowledge(query, topK, filterType)

	case "index_document":
		content, ok := arguments["content"].(string)
		if !ok || content == "" {
			return mcphelper.ErrorResult(fmt.Errorf("missing or invalid content parameter")), nil
		}
		source, ok := arguments["source"].(string)
		if !ok || source == "" {
			return mcphelper.ErrorResult(fmt.Errorf("missing or invalid source parameter")), nil
		}
		docType, ok := arguments["type"].(string)
		if !ok || docType == "" {
			return mcphelper.ErrorResult(fmt.Errorf("missing or invalid type parameter")), nil
		}
		return p.indexDocument(content, source, docType)
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

type queryRequest struct {
	Query  string                 `json:"query"`
	TopK   int                    `json:"top_k"`
	Filter map[string]interface{} `json:"filter,omitempty"`
}

type queryResult struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Content  string                 `json:"content"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (p *Provider) queryKnowledge(query string, topK int, filterType string) (*mcp.CallToolResult, error) {
	reqBody := queryRequest{
		Query: query,
		TopK:  topK,
	}
	if filterType != "" {
		reqBody.Filter = map[string]interface{}{
			"type": filterType,
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to marshal query request: %w", err)), nil
	}

	url := fmt.Sprintf("%s/query", p.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to create http request: %w", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return mcphelper.TextResult(fmt.Sprintf("Error: homelab-context RAG server is offline or unreachable at %s: %v. RAG queries are temporarily unavailable.", p.baseURL, err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcphelper.TextResult(fmt.Sprintf("Error: homelab-context server returned status code %d at %s. RAG queries are temporarily unavailable.", resp.StatusCode, p.baseURL)), nil
	}

	var result queryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to decode response: %w", err)), nil
	}

	if len(result.Results) == 0 {
		return mcphelper.TextResult("No relevant knowledge base entries found matching the query."), nil
	}

	var buf bytes.Buffer
	buf.WriteString("Found relevant context in homelab-context RAG database:\n\n")
	for i, res := range result.Results {
		source := "unknown"
		docType := "unknown"
		if res.Metadata != nil {
			if s, ok := res.Metadata["source"].(string); ok {
				source = s
			}
			if t, ok := res.Metadata["type"].(string); ok {
				docType = t
			}
		}
		fmt.Fprintf(&buf, "[%d] Score: %.2f | Source: %s | Type: %s\n", i+1, res.Score, source, docType)
		fmt.Fprintf(&buf, "Content:\n%s\n\n---\n\n", res.Content)
	}

	return mcphelper.TextResult(buf.String()), nil
}

type indexRequest struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

type indexResponse struct {
	ID            string `json:"id"`
	ChunksIndexed int    `json:"chunks_indexed"`
}

func (p *Provider) indexDocument(content, source, docType string) (*mcp.CallToolResult, error) {
	reqBody := indexRequest{
		Content: content,
		Metadata: map[string]interface{}{
			"source": source,
			"type":   docType,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to marshal index request: %w", err)), nil
	}

	url := fmt.Sprintf("%s/index", p.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to create http request: %w", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return mcphelper.TextResult(fmt.Sprintf("Error: homelab-context RAG server is offline or unreachable at %s: %v. Indexing is temporarily unavailable.", p.baseURL, err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcphelper.TextResult(fmt.Sprintf("Error: homelab-context server returned status code %d at %s. Indexing is temporarily unavailable.", resp.StatusCode, p.baseURL)), nil
	}

	var result indexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return mcphelper.ErrorResult(fmt.Errorf("failed to decode response: %w", err)), nil
	}

	return mcphelper.TextResult(fmt.Sprintf("Success: Indexed document from source '%s' (type: %s). Chunks generated and stored: %d.", source, docType, result.ChunksIndexed)), nil
}
