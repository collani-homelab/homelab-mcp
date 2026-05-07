package hello

import (
	"testing"
)

func TestHelloProvider_Name(t *testing.T) {
	p := &HelloProvider{}
	if p.Name() != "hello" {
		t.Errorf("expected name 'hello', got %s", p.Name())
	}
}

func TestHelloProvider_GetResources(t *testing.T) {
	p := &HelloProvider{}
	resources, err := p.GetResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].URI != "hello://world" {
		t.Errorf("expected URI 'hello://world', got %s", resources[0].URI)
	}
}

func TestHelloProvider_GetResourceContent(t *testing.T) {
	p := &HelloProvider{}
	content, err := p.GetResourceContent("hello://world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hello from the Homelab MCP Server!"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}

	_, err = p.GetResourceContent("unknown")
	if err == nil {
		t.Error("expected error for unknown URI, got nil")
	}
}
