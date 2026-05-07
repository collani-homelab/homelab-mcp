package unraid

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
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
		w.Write([]byte(mockResponse))
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "test-key", false)
	content, err := p.GetResourceContent("unraid://test-server/containers")
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
		w.Write([]byte(errorResponse))
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "", false)
	
	// Test unsupported URI
	_, err := p.GetResourceContent("unsupported://uri")
	if err == nil || !strings.Contains(err.Error(), "unsupported resource URI") {
		t.Errorf("expected unsupported URI error, got %v", err)
	}

	// Test GraphQL error from server
	_, err = p.GetResourceContent("unraid://test-server/containers")
	if err == nil || !strings.Contains(err.Error(), "GraphQL error: Some GraphQL Error") {
		t.Errorf("expected GraphQL error, got %v", err)
	}
}

func TestProvider_GetResourceContent_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer ts.Close()

	p, _ := NewProvider("test-server", ts.URL, "", false)
	
	_, err := p.GetResourceContent("unraid://test-server/containers")
	if err == nil || !strings.Contains(err.Error(), "API error: status=500") {
		t.Errorf("expected HTTP 500 error, got %v", err)
	}
}
