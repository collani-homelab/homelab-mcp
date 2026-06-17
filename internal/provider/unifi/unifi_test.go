package unifi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewProvider(t *testing.T) {
	p, err := NewProvider("https://unifi", "mock-api-key", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected provider, got nil")
	}

	_, err = NewProvider("", "mock-api-key", true)
	if err == nil {
		t.Error("expected error for missing baseURL")
	}

	_, err = NewProvider("https://unifi", "", true)
	if err == nil {
		t.Error("expected error for missing apiKey")
	}
}

func TestProvider_Name(t *testing.T) {
	p, _ := NewProvider("https://unifi", "mock-api-key", true)
	if p.Name() != "UniFi" {
		t.Errorf("expected 'UniFi', got %q", p.Name())
	}
}

func TestProvider_GetResources(t *testing.T) {
	p, _ := NewProvider("https://unifi", "mock-api-key", true)
	resources, err := p.GetResources()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 5 {
		t.Fatalf("expected 5 resources, got %d", len(resources))
	}
	if resources[0].URI != "unifi://clients" {
		t.Errorf("expected URI 'unifi://clients', got %q", resources[0].URI)
	}
	if resources[1].URI != "unifi://devices" {
		t.Errorf("expected URI 'unifi://devices', got %q", resources[1].URI)
	}
}

func TestProvider_GetResourceContent_SuccessAPIKey(t *testing.T) {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/proxy/network/api/s/default/stat/sta", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "mock-api-key" {
			t.Errorf("missing or invalid API token: %s", r.Header.Get("X-API-KEY"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"mac": "00:11:22:33:44:55", "name": "API Key Client"}]}`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	p, _ := NewProvider(ts.URL, "mock-api-key", false)
	content, err := p.GetResourceContent(context.Background(), "unifi://clients")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if !strings.Contains(content, "API Key Client") {
		t.Errorf("expected content to contain 'API Key Client', got %s", content)
	}
}

func TestProvider_GetResourceContent_Fallback(t *testing.T) {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/proxy/network/api/s/default/stat/sta", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/api/s/default/stat/sta", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "mock-api-key" {
			t.Errorf("missing or invalid API token: %s", r.Header.Get("X-API-KEY"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"mac": "00:11:22:33:44:55", "name": "Fallback Client"}]}`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	p, _ := NewProvider(ts.URL, "mock-api-key", false)
	content, err := p.GetResourceContent(context.Background(), "unifi://clients")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if !strings.Contains(content, "Fallback Client") {
		t.Errorf("expected content to contain 'Fallback Client', got %s", content)
	}
}

func TestProvider_GetResourceContent_Devices(t *testing.T) {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/proxy/network/api/s/default/stat/device", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"name": "U6-Pro", "type": "uap"}]}`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	p, _ := NewProvider(ts.URL, "mock-api-key", false)
	content, err := p.GetResourceContent(context.Background(), "unifi://devices")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if !strings.Contains(content, "U6-Pro") {
		t.Errorf("expected content to contain 'U6-Pro', got %s", content)
	}
}
