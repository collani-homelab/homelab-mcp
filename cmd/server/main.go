package main

import (
	"context"
	"fmt"
	"homelab-mcp/internal/mcp"
	"homelab-mcp/internal/provider/hello"
	"homelab-mcp/internal/provider/unifi"
	"homelab-mcp/internal/provider/unraid"
	"log/slog"
	"os"
	"strings"
)

func setupServer() *mcp.Server {
	s := mcp.NewServer("homelab-mcp", "0.1.0")

	// Add providers
	s.AddProvider(&hello.HelloProvider{})

	// Scan environment variables for UNRAID_*_URL
	unraidConfigsFound := false
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]

		if strings.HasPrefix(key, "UNRAID_") && strings.HasSuffix(key, "_URL") {
			unraidConfigsFound = true

			// Extract the name part, e.g. from UNRAID_DIONYSUS_URL -> DIONYSUS -> dionysus
			namePart := strings.TrimSuffix(strings.TrimPrefix(key, "UNRAID_"), "_URL")
			name := strings.ToLower(namePart)

			// Look for matching KEY and SKIP_VERIFY
			apiKeyKey := fmt.Sprintf("UNRAID_%s_KEY", namePart)
			// also fallback to old API_KEY just in case
			apiKeyAltKey := fmt.Sprintf("UNRAID_%s_API_KEY", namePart)

			apiKey := os.Getenv(apiKeyKey)
			if apiKey == "" {
				apiKey = os.Getenv(apiKeyAltKey)
			}

			skipVerifyKey := fmt.Sprintf("UNRAID_%s_SKIP_VERIFY", namePart)
			skipVerify := os.Getenv(skipVerifyKey) == "true"

			unraidProvider, err := unraid.NewProvider(name, val, apiKey, skipVerify)
			if err != nil {
				slog.Error("Failed to initialize Unraid provider", "name", name, "error", err)
			} else {
				s.AddProvider(unraidProvider)
				slog.Info("Unraid provider added", "name", name, "url", val)
			}
		}
	}

	if !unraidConfigsFound {
		// Fallback to single UNRAID_URL if multiple are not configured
		unraidURL := os.Getenv("UNRAID_URL")
		if unraidURL != "" {
			apiKey := os.Getenv("UNRAID_API_KEY")
			skipVerify := os.Getenv("UNRAID_SKIP_VERIFY") == "true"

			unraidProvider, err := unraid.NewProvider("default", unraidURL, apiKey, skipVerify)
			if err != nil {
				slog.Error("Failed to initialize default Unraid provider", "error", err)
			} else {
				s.AddProvider(unraidProvider)
				slog.Info("Unraid provider added", "name", "default", "url", unraidURL)
			}
		} else {
			slog.Warn("No Unraid URLs configured, skipping Unraid providers")
		}
	}

	// Add UniFi Provider
	unifiURL := os.Getenv("UNIFI_API_URL")
	if unifiURL != "" {
		apiKey := os.Getenv("UNIFI_API_KEY")
		skipVerify := os.Getenv("UNIFI_SKIP_VERIFY") == "true"

		unifiProvider, err := unifi.NewProvider(unifiURL, apiKey, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize UniFi provider", "error", err)
		} else {
			s.AddProvider(unifiProvider)
			slog.Info("UniFi provider added", "url", unifiURL)
		}
	} else {
		slog.Warn("UNIFI_API_URL not set, skipping UniFi provider")
	}

	return s
}

func main() {
	// Configure slog to write to Stderr
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	s := setupServer()

	slog.Info("Starting Homelab MCP Server", "version", "0.1.0")

	// Run the server
	if err := s.Run(context.Background()); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
