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
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"homelab-mcp/internal/provider"
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

	var transport *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = dt.Clone()
	} else {
		transport = &http.Transport{}
	}
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
		{
			URI:      fmt.Sprintf("unraid://%s/system/syslog", p.name),
			Name:     fmt.Sprintf("Unraid System Log (%s)", p.name),
			MIMEType: "text/plain",
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

func (p *Provider) queryGraphQL(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	reqBody, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(bodyBytes, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data, nil
}

func (p *Provider) GetResourceContent(ctx context.Context, uri string) (string, error) {
	containersURI := fmt.Sprintf("unraid://%s/containers", p.name)
	statsURI := fmt.Sprintf("unraid://%s/system/stats", p.name)
	arrayURI := fmt.Sprintf("unraid://%s/array/status", p.name)
	vmsURI := fmt.Sprintf("unraid://%s/vms", p.name)
	upsURI := fmt.Sprintf("unraid://%s/system/ups", p.name)
	notificationsURI := fmt.Sprintf("unraid://%s/notifications", p.name)
	syslogURI := fmt.Sprintf("unraid://%s/system/syslog", p.name)

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
	case syslogURI:
		query = `query {
  logFile(path: "/var/log/syslog") {
    content
  }
}`
	default:
		// Check for container logs template match
		prefix := fmt.Sprintf("unraid://%s/containers/", p.name)
		suffix := "/logs"
		if strings.HasPrefix(uri, prefix) && strings.HasSuffix(uri, suffix) {
			containerName := strings.TrimSuffix(strings.TrimPrefix(uri, prefix), suffix)
			if containerName != "" && !strings.Contains(containerName, "/") {
				// Step 1: Query containers list to get matching container ID
			listQuery := `query {
				docker {
					containers {
						id
						names
					}
				}
			}`
			dataBytes, err := p.queryGraphQL(ctx, listQuery, nil)
			if err != nil {
				return "", fmt.Errorf("failed to list containers: %w", err)
			}

			var containersResp struct {
				Docker struct {
					Containers []struct {
						ID    string   `json:"id"`
						Names []string `json:"names"`
					} `json:"containers"`
				} `json:"docker"`
			}
			if err := json.Unmarshal(dataBytes, &containersResp); err != nil {
				return "", fmt.Errorf("failed to parse container list response: %w", err)
			}

			var targetID string
			for _, container := range containersResp.Docker.Containers {
				matched := false
				for _, name := range container.Names {
					if name == containerName || name == "/"+containerName || strings.TrimPrefix(name, "/") == containerName {
						matched = true
						break
					}
				}
				if matched {
					targetID = container.ID
					break
				}
			}

			if targetID == "" {
				return "", fmt.Errorf("container %s not found on Unraid (%s)", containerName, p.name)
			}

			// Step 2: Query the container logs using the target ID
			logsQuery := `query($id: String!) {
				docker {
					logs(id: $id, tail: 100) {
						lines {
							timestamp
							message
						}
					}
				}
			}`

			logsDataBytes, err := p.queryGraphQL(ctx, logsQuery, map[string]interface{}{"id": targetID})
			if err != nil {
				return "", fmt.Errorf("failed to fetch container logs: %w", err)
			}

			var logsResp struct {
				Docker struct {
					Logs struct {
						Lines []struct {
							Timestamp string `json:"timestamp"`
							Message   string `json:"message"`
						} `json:"lines"`
					} `json:"logs"`
				} `json:"docker"`
			}
			if err := json.Unmarshal(logsDataBytes, &logsResp); err != nil {
				return "", fmt.Errorf("failed to parse container logs response: %w", err)
			}

			// Step 3: Format the logs as plain text
			var buf bytes.Buffer
			for _, line := range logsResp.Docker.Logs.Lines {
				if line.Timestamp != "" {
					fmt.Fprintf(&buf, "[%s] %s\n", line.Timestamp, line.Message)
				} else {
					fmt.Fprintf(&buf, "%s\n", line.Message)
				}
			}
			return buf.String(), nil
			}
		}
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	dataBytes, err := p.queryGraphQL(ctx, query, nil)
	if err != nil {
		return "", err
	}
	return string(dataBytes), nil
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

func (p *Provider) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	return nil, fmt.Errorf("prompt not found: %s", name)
}

func (p *Provider) GetTools() ([]mcp.Tool, error) {
	return []mcp.Tool{
		{
			Name:        fmt.Sprintf("get_unraid_system_stats_%s", p.name),
			Description: fmt.Sprintf("Retrieves CPU, memory, server info, and OS details for the Unraid server '%s'.", p.name),
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        fmt.Sprintf("get_unraid_array_status_%s", p.name),
			Description: fmt.Sprintf("Retrieves array state, parity check status, and disk inventory for the Unraid server '%s'.", p.name),
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        fmt.Sprintf("get_unraid_containers_%s", p.name),
			Description: fmt.Sprintf("Retrieves docker container list including status, autostart, LAN port mappings, log size, and updates for the Unraid server '%s'.", p.name),
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        fmt.Sprintf("get_unraid_ups_status_%s", p.name),
			Description: fmt.Sprintf("Retrieves UPS status, charge, runtime, and load details for the Unraid server '%s'. Graces gracefully on communication errors.", p.name),
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}, nil
}

func (p *Provider) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	systemStatsToolName := fmt.Sprintf("get_unraid_system_stats_%s", p.name)
	arrayStatusToolName := fmt.Sprintf("get_unraid_array_status_%s", p.name)
	containersToolName := fmt.Sprintf("get_unraid_containers_%s", p.name)
	upsStatusToolName := fmt.Sprintf("get_unraid_ups_status_%s", p.name)

	pruneKeys := []string{"_id", "site_id", "oui", "fingerprint"}

	switch name {
	case systemStatsToolName:
		query := `query {
  metrics {
    cpu {
      percentTotal
    }
    memory {
      total
      used
      free
      percentTotal
    }
  }
  info {
    system {
      manufacturer
      model
    }
    os {
      distro
      release
      uptime
    }
  }
}`
		dataBytes, err := p.queryGraphQL(ctx, query, nil)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to fetch system stats: %v", err)},
				},
			}, nil
		}
		pruned, err := provider.PruneJSON(dataBytes, pruneKeys)
		if err == nil {
			dataBytes = pruned
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(dataBytes)},
			},
		}, nil

	case arrayStatusToolName:
		query := `query {
  array {
    state
    parityCheckStatus {
      status
      progress
      speed
      duration
    }
    disks {
      name
      size
      fsSize
      fsFree
      status
      temp
      numErrors
      isSpinning
    }
  }
}`
		dataBytes, err := p.queryGraphQL(ctx, query, nil)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to fetch array status: %v", err)},
				},
			}, nil
		}
		pruned, err := provider.PruneJSON(dataBytes, pruneKeys)
		if err == nil {
			dataBytes = pruned
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(dataBytes)},
			},
		}, nil

	case containersToolName:
		query := `query {
  docker {
    containers {
      names
      image
      state
      status
      autoStart
      lanIpPorts
      sizeLog
      isUpdateAvailable
    }
  }
}`
		dataBytes, err := p.queryGraphQL(ctx, query, nil)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to fetch docker containers: %v", err)},
				},
			}, nil
		}
		pruned, err := provider.PruneJSON(dataBytes, pruneKeys)
		if err == nil {
			dataBytes = pruned
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(dataBytes)},
			},
		}, nil

	case upsStatusToolName:
		query := `query {
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
      currentPower
    }
  }
}`
		dataBytes, err := p.queryGraphQL(ctx, query, nil)
		if err != nil {
			// Return a graceful JSON error instead of a protocol-level error so
			// the LLM sees context rather than an opaque failure.
			body, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("UPS status unavailable: %v", err)})
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(body)},
				},
			}, nil
		}
		pruned, err := provider.PruneJSON(dataBytes, pruneKeys)
		if err == nil {
			dataBytes = pruned
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(dataBytes)},
			},
		}, nil
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}
