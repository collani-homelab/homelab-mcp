package phoenix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewProviderFromEnv(t *testing.T) {
	t.Setenv("PHOENIX_URL", "http://phoenix.example")
	p := NewProviderFromEnv()
	if p.baseURL != "http://phoenix.example" {
		t.Errorf("expected baseURL 'http://phoenix.example', got %q", p.baseURL)
	}
}

func TestProvider_Name(t *testing.T) {
	p := NewProvider("http://phoenix")
	if p.Name() != "phoenix" {
		t.Errorf("expected 'phoenix', got %q", p.Name())
	}
}

func TestProvider_GetResources(t *testing.T) {
	p := NewProvider("http://phoenix")
	resources, err := p.GetResources()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 || resources[0].URI != "phoenix://projects" {
		t.Errorf("expected single 'phoenix://projects' resource, got %+v", resources)
	}
}

func TestProvider_GetResourceContent_Projects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "1", "name": "agents"}], "next_cursor": null}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := NewProvider(ts.URL)
	content, err := p.GetResourceContent("phoenix://projects")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(content, "agents") {
		t.Errorf("expected content to contain 'agents', got %s", content)
	}
}

func TestProvider_GetResourceContent_NotFound(t *testing.T) {
	p := NewProvider("http://phoenix")
	_, err := p.GetResourceContent("phoenix://unknown")
	if err == nil {
		t.Error("expected error for unknown resource")
	}
}

func TestProvider_CallTool_QueryTraces(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects/agents/spans", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("attribute") != "llm.model_name:gpt-4" {
			t.Errorf("expected model attribute filter, got query %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "s1", "name": "llm_call", "context": {"trace_id": "t1", "span_id": "sp1"}, "span_kind": "LLM", "start_time": "2024-01-01T00:00:00Z", "end_time": "2024-01-01T00:00:01Z", "status_code": "OK", "attributes": {"llm.model_name": "gpt-4"}}], "next_cursor": null}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := NewProvider(ts.URL)
	result, err := p.CallTool("query_phoenix_traces", map[string]interface{}{
		"project": "agents",
		"model":   "gpt-4",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result.Content)
	}
}

func TestProvider_CallTool_MissingProject(t *testing.T) {
	p := NewProvider("http://phoenix")
	result, err := p.CallTool("query_phoenix_traces", map[string]interface{}{})
	if err != nil {
		t.Fatalf("expected no transport error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for missing project argument")
	}
}

func TestProvider_CallTool_GetEvalScores(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects/agents/spans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "s1", "name": "llm_call", "context": {"trace_id": "t1", "span_id": "sp1"}, "span_kind": "LLM", "start_time": "2024-01-01T00:00:00Z", "end_time": "2024-01-01T00:00:01Z", "status_code": "OK", "attributes": {"llm.model_name": "gpt-4"}}], "next_cursor": null}`))
	})
	mux.HandleFunc("/v1/projects/agents/span_annotations", func(w http.ResponseWriter, r *http.Request) {
		// Phoenix requires span_ids or identifier — verify we always send one.
		if r.URL.Query().Get("span_ids") != "sp1" {
			t.Errorf("expected span_ids=sp1, got query %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "a1", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "source": "API", "user_id": null, "name": "correctness", "annotator_kind": "LLM", "result": {"label": "correct", "score": 1.0, "explanation": null}, "span_id": "sp1"}], "next_cursor": null}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := NewProvider(ts.URL)
	result, err := p.CallTool("get_phoenix_eval_scores", map[string]interface{}{
		"project": "agents",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result.Content)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "gpt-4") || !strings.Contains(text, "correctness") {
		t.Errorf("expected aggregated score for gpt-4/correctness, got %s", text)
	}
}

func TestProvider_CallTool_GetEvalScores_NoSpans(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects/agents/spans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [], "next_cursor": null}`))
	})
	mux.HandleFunc("/v1/projects/agents/span_annotations", func(w http.ResponseWriter, r *http.Request) {
		t.Error("span_annotations should not be called when there are no spans")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := NewProvider(ts.URL)
	result, err := p.CallTool("get_phoenix_eval_scores", map[string]interface{}{
		"project": "agents",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result.Content)
	}
}

func TestProvider_CallTool_UpstreamError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects/agents/spans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := NewProvider(ts.URL)
	result, err := p.CallTool("get_phoenix_span_errors", map[string]interface{}{
		"project": "agents",
	})
	if err != nil {
		t.Fatalf("expected no transport error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for upstream 500")
	}
}
