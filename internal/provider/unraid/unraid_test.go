package unraid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewProvider(t *testing.T) {
	// Valid configuration
	p, err := NewProvider("test", "http://localhost", "apikey", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected provider, got nil")
	}

	// Missing name
	_, err = NewProvider("", "http://localhost", "apikey", false)
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Missing baseURL
	_, err = NewProvider("test", "", "apikey", false)
	if err == nil {
		t.Error("expected error for missing baseURL")
	}

	// Invalid URL
	_, err = NewProvider("test", ":", "apikey", false)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestProvider_Name(t *testing.T) {
	p, _ := NewProvider("test-server", "http://localhost", "", false)
	expected := "Unraid (test-server)"
	if p.Name() != expected {
		t.Errorf("expected %q, got %q", expected, p.Name())
	}
}

func TestProvider_GetResources(t *testing.T) {
	p, _ := NewProvider("test-server", "http://localhost", "", false)
	resources, err := p.GetResources()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 7 {
		t.Fatalf("expected 7 resource, got %d", len(resources))
	}
	expectedURI := "unraid://test-server/containers"
	if resources[0].URI != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, resources[0].URI)
	}
}

func TestProvider_GetResourceContent_Success(t *testing.T) {
	mockResponse := `{
		"data": {
			"docker": {
				"containers": [
					{
						"id": "123",
						"names": ["/test-container"],
						"image": "test-image",
						"state": "RUNNING",
						"status": "Up 2 hours",
						"autoStart": true
					}
				]
			}
		}
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected API key 'test-key', got %s", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "test-key", false)
	content, err := p.GetResourceContent(context.Background(), "unraid://test-server/containers")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that the returned content matches the 'data' portion of the JSON
	var parsedResponse map[string]interface{}
	err = json.Unmarshal([]byte(content), &parsedResponse)
	if err != nil {
		t.Fatalf("expected valid JSON in response content, got error: %v", err)
	}
	
	if _, ok := parsedResponse["docker"]; !ok {
		t.Errorf("expected 'docker' key in content data")
	}
}

func TestProvider_GetResourceContent_Errors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock a GraphQL error
		errorResponse := `{
			"errors": [{"message": "Some GraphQL Error"}]
		}`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(errorResponse))
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "", false)
	
	// Test unsupported URI
	_, err := p.GetResourceContent(context.Background(), "unsupported://uri")
	if err == nil || !strings.Contains(err.Error(), "unsupported resource URI") {
		t.Errorf("expected unsupported URI error, got %v", err)
	}

	// Test GraphQL error from server
	_, err = p.GetResourceContent(context.Background(), "unraid://test-server/containers")
	if err == nil || !strings.Contains(err.Error(), "GraphQL error: Some GraphQL Error") {
		t.Errorf("expected GraphQL error, got %v", err)
	}
}

func TestProvider_GetResourceContent_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "", false)
	
	_, err := p.GetResourceContent(context.Background(), "unraid://test-server/containers")
	if err == nil || !strings.Contains(err.Error(), "API error: status=500") {
		t.Errorf("expected HTTP 500 error, got %v", err)
	}
}

func TestProvider_GetResourceContent_ContainerLogs(t *testing.T) {
	containerListResponse := `{
		"data": {
			"docker": {
				"containers": [
					{
						"id": "123",
						"names": ["/test-container"]
					}
				]
			}
		}
	}`

	logsResponse := `{
		"data": {
			"docker": {
				"logs": {
					"lines": [
						{
							"timestamp": "2026-05-18T14:00:00Z",
							"message": "Log line 1"
						},
						{
							"timestamp": "2026-05-18T14:01:00Z",
							"message": "Log line 2"
						}
					]
				}
			}
		}
	}`

	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		
		query, _ := reqBody["query"].(string)
		w.WriteHeader(http.StatusOK)
		
		if strings.Contains(query, "containers") {
			_, _ = w.Write([]byte(containerListResponse))
		} else if strings.Contains(query, "logs") {
			_, _ = w.Write([]byte(logsResponse))
		} else {
			t.Errorf("unexpected GraphQL query: %s", query)
		}
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "", false)
	content, err := p.GetResourceContent(context.Background(), "unraid://test-server/containers/test-container/logs")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedLogs := "[2026-05-18T14:00:00Z] Log line 1\n[2026-05-18T14:01:00Z] Log line 2\n"
	if content != expectedLogs {
		t.Errorf("expected logs:\n%q\ngot:\n%q", expectedLogs, content)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
}

func TestProvider_Tools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		
		query, _ := reqBody["query"].(string)
		w.WriteHeader(http.StatusOK)

		if strings.Contains(query, "metrics") {
			_, _ = w.Write([]byte(`{"data": {"metrics": {"cpu": {"percentTotal": 5.2}}}}`))
		} else if strings.Contains(query, "array") {
			_, _ = w.Write([]byte(`{"data": {"array": {"state": "STARTED"}}}`))
		} else if strings.Contains(query, "docker") {
			_, _ = w.Write([]byte(`{"data": {"docker": {"containers": []}}}`))
		} else if strings.Contains(query, "upsDevices") {
			if r.URL.Path == "/fail_ups" {
				// Trigger an error
				_, _ = w.Write([]byte(`{"errors": [{"message": "No UPS data returned from apcaccess"}]}`))
			} else {
				_, _ = w.Write([]byte(`{"data": {"upsDevices": [{"name": "APC UPS"}]}}`))
			}
		} else {
			t.Errorf("unexpected query: %s", query)
		}
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "", false)

	// Test GetTools
	tools, err := p.GetTools()
	if err != nil {
		t.Fatalf("GetTools error: %v", err)
	}
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}

	// Test CallTool - System Stats
	res, err := p.CallTool(context.Background(), "get_unraid_system_stats_test-server", nil)
	if err != nil {
		t.Fatalf("CallTool system stats failed: %v", err)
	}
	if res.IsError {
		t.Errorf("system stats returned error: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// Test CallTool - Array Status
	res, err = p.CallTool(context.Background(), "get_unraid_array_status_test-server", nil)
	if err != nil {
		t.Fatalf("CallTool array status failed: %v", err)
	}
	if res.IsError {
		t.Errorf("array status returned error: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// Test CallTool - Containers
	res, err = p.CallTool(context.Background(), "get_unraid_containers_test-server", nil)
	if err != nil {
		t.Fatalf("CallTool containers failed: %v", err)
	}
	if res.IsError {
		t.Errorf("containers returned error: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// Test CallTool - UPS Status (Success)
	res, err = p.CallTool(context.Background(), "get_unraid_ups_status_test-server", nil)
	if err != nil {
		t.Fatalf("CallTool ups status failed: %v", err)
	}
	if res.IsError {
		t.Errorf("ups status returned error: %s", res.Content[0].(*mcp.TextContent).Text)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "upsDevices") {
		t.Errorf("expected upsDevices in response, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// Test CallTool - UPS Status (Graceful Error Handled)
	// We can use a different provider instance pointing to a server that triggers errors
	pError, _ := NewProvider("fail_ups", ts.URL+"/fail_ups", "", false)
	res, err = pError.CallTool(context.Background(), "get_unraid_ups_status_fail_ups", nil)
	if err != nil {
		t.Fatalf("CallTool ups status error test failed: %v", err)
	}
	if res.IsError {
		t.Errorf("ups status error call should be handled gracefully (IsError=false), but got IsError=true")
	}
	txt := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(txt, "error") || !strings.Contains(txt, "No UPS data returned from apcaccess") {
		t.Errorf("expected graceful JSON error message, got: %s", txt)
	}
}
