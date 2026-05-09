package unraid

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Provider struct {
	name    string
	client  *http.Client
	baseURL string
	apiKey  string
}

// NewProvider creates a new Unraid provider.
// name is a friendly identifier (e.g. "dionysus")
// baseURL should be the root URL, e.g., "https://192.168.1.10" or "http://tower.local"
func NewProvider(name, baseURL, apiKey string, skipVerify bool) (*Provider, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	return &Provider{
		name:    name,
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}, nil
}

func (p *Provider) Name() string {
	return fmt.Sprintf("Unraid (%s)", p.name)
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      fmt.Sprintf("unraid://%s/containers", p.name),
			Name:     fmt.Sprintf("Unraid Docker Containers (%s)", p.name),
			MIMEType: "application/json",
		},
		{
			URI:      fmt.Sprintf("unraid://%s/system/stats", p.name),
			Name:     fmt.Sprintf("Unraid System Stats (%s)", p.name),
			MIMEType: "application/json",
		},
		{
			URI:      fmt.Sprintf("unraid://%s/array/status", p.name),
			Name:     fmt.Sprintf("Unraid Array Status (%s)", p.name),
			MIMEType: "application/json",
		},
		{
			URI:      fmt.Sprintf("unraid://%s/vms", p.name),
			Name:     fmt.Sprintf("Unraid Virtual Machines (%s)", p.name),
			MIMEType: "application/json",
		},
		{
			URI:      fmt.Sprintf("unraid://%s/system/ups", p.name),
			Name:     fmt.Sprintf("Unraid UPS Status (%s)", p.name),
			MIMEType: "application/json",
		},
		{
			URI:      fmt.Sprintf("unraid://%s/notifications", p.name),
			Name:     fmt.Sprintf("Unraid System Notifications (%s)", p.name),
			MIMEType: "application/json",
		},
	}, nil
}

type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	containersURI := fmt.Sprintf("unraid://%s/containers", p.name)
	statsURI := fmt.Sprintf("unraid://%s/system/stats", p.name)
	arrayURI := fmt.Sprintf("unraid://%s/array/status", p.name)
	vmsURI := fmt.Sprintf("unraid://%s/vms", p.name)
	upsURI := fmt.Sprintf("unraid://%s/system/ups", p.name)
	notificationsURI := fmt.Sprintf("unraid://%s/notifications", p.name)

	var query string
	switch uri {
	case containersURI:
		query = `query {
  docker {
    containers {
      id
      names
      image
      state
      status
      autoStart
    }
  }
}`
	case statsURI:
		query = `query {
  info {
    time
  }
  metrics {
    memory {
      total
      free
      used
    }
  }
}`
	case arrayURI:
		query = `query {
  array {
    state
    parityCheckStatus {
      status
      progress
      speed
      duration
    }
    parities {
      name
      size
      status
    }
    disks {
      name
      size
      status
    }
  }
}`
	case vmsURI:
		query = `query {
  vms {
    domains {
      name
      state
    }
  }
}`
	case upsURI:
		query = `query {
  upsDevices {
    name
    status
    battery {
      chargeLevel
      estimatedRuntime
      health
    }
    power {
      inputVoltage
      loadPercentage
    }
  }
}`
	case notificationsURI:
		query = `query {
  notifications {
    list(filter: { offset: 0, limit: 10, type: UNREAD }) {
      title
      subject
      description
      importance
      formattedTimestamp
    }
  }
}`
	default:
		// Check for container logs template match
		var containerName string
		if _, err := fmt.Sscanf(uri, fmt.Sprintf("unraid://%s/containers/%%s/logs", p.name), &containerName); err == nil {
			// A real implementation would fetch docker logs here using GraphQL or Docker API.
			// Currently Unraid GraphQL for docker logs is restricted or complex, so we return a placeholder.
			return fmt.Sprintf("Logs for container %s are not yet implemented natively.", containerName), nil
		}
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	reqBody, err := json.Marshal(graphQLRequest{Query: query})
	if err != nil {
		return "", fmt.Errorf("failed to marshal query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The .env file from the user already has /graphql appended to the URL.
	// We should be careful. Let's just use the baseURL if it ends with /graphql, otherwise append it.
	// But let's just use the user's provided URL directly as the endpoint for now.
	endpoint := p.baseURL
	
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(bodyBytes, &gqlResp); err != nil {
		return "", fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return "", fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return string(gqlResp.Data), nil
}

func (p *Provider) GetResourceTemplates() ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{
		{
			URITemplate: fmt.Sprintf("unraid://%s/containers/{name}/logs", p.name),
			Name:        "Unraid Container Logs",
			MIMEType:    "text/plain",
		},
	}, nil
}

func (p *Provider) GetPrompts() ([]mcp.Prompt, error) {
	return []mcp.Prompt{}, nil
}

func (p *Provider) GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

