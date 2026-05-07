package unifi

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Provider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func NewProvider(baseURL, apiKey string, skipVerify bool) (*Provider, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("apiKey is required")
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
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}, nil
}

func (p *Provider) Name() string {
	return "UniFi"
}

func (p *Provider) GetResources() ([]mcp.Resource, error) {
	return []mcp.Resource{
		{
			URI:      "unifi://clients",
			Name:     "UniFi Active Clients",
			MIMEType: "application/json",
		},
	}, nil
}

func (p *Provider) GetResourceContent(uri string) (string, error) {
	if uri != "unifi://clients" {
		return "", fmt.Errorf("unsupported resource URI: %s", uri)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch from the integration endpoint
	// Note: For UniFi OS consoles using local API, this might need to be prefixed with /proxy/network
	// depending on firmware version. We'll try the proxy one first, then fallback.
	endpoint := fmt.Sprintf("%s/proxy/network/api/s/default/stat/sta", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-KEY", p.apiKey)

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
		// Fallback to non-proxied endpoint if the proxy one fails
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
			fallbackEndpoint := fmt.Sprintf("%s/api/s/default/stat/sta", p.baseURL)
			fallbackReq, _ := http.NewRequestWithContext(ctx, "GET", fallbackEndpoint, nil)
			fallbackReq.Header.Set("Content-Type", "application/json")
			fallbackReq.Header.Set("Accept", "application/json")
			fallbackReq.Header.Set("X-API-KEY", p.apiKey)
			
			fallbackResp, err := p.client.Do(fallbackReq)
			if err == nil {
				defer fallbackResp.Body.Close()
				if fallbackResp.StatusCode == http.StatusOK {
					fallbackBodyBytes, _ := io.ReadAll(fallbackResp.Body)
					return string(fallbackBodyBytes), nil
				}
			}
		}

		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	return string(bodyBytes), nil
}
