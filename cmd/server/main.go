package main

import (
	"context"
	"fmt"
	"homelab-mcp/internal/mcp"
	ragcontext "homelab-mcp/internal/provider/context"
	"homelab-mcp/internal/provider/hello"
	"homelab-mcp/internal/provider/lidarr"
	"homelab-mcp/internal/provider/monitoring"
	"homelab-mcp/internal/provider/nzbget"
	"homelab-mcp/internal/provider/plex"
	"homelab-mcp/internal/provider/radarr"
	"homelab-mcp/internal/provider/sonarr"
	"homelab-mcp/internal/provider/tautulli"
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

	// Add Monitoring Provider
	s.AddProvider(monitoring.NewProvider())
	slog.Info("Monitoring provider added")

	// Add RAG context Provider
	contextURL := os.Getenv("HOMELAB_CONTEXT_URL")
	s.AddProvider(ragcontext.NewProvider(contextURL))
	slog.Info("RAG context provider added", "url", contextURL)

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

	// Add Tautulli Provider
	tautulliURL := os.Getenv("TAUTULLI_API_URL")
	if tautulliURL != "" {
		apiKey := os.Getenv("TAUTULLI_API_KEY")
		skipVerify := os.Getenv("TAUTULLI_SKIP_VERIFY") == "true"

		tautulliProvider, err := tautulli.NewProvider(tautulliURL, apiKey, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize Tautulli provider", "error", err)
		} else {
			s.AddProvider(tautulliProvider)
			slog.Info("Tautulli provider added", "url", tautulliURL)
		}
	} else {
		slog.Warn("TAUTULLI_API_URL not set, skipping Tautulli provider")
	}

	// Add Plex Provider
	plexURL := os.Getenv("PLEX_API_URL")
	if plexURL != "" {
		token := os.Getenv("PLEX_API_TOKEN")
		skipVerify := os.Getenv("PLEX_SKIP_VERIFY") == "true"

		plexProvider, err := plex.NewProvider(plexURL, token, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize Plex provider", "error", err)
		} else {
			s.AddProvider(plexProvider)
			slog.Info("Plex provider added", "url", plexURL)
		}
	} else {
		slog.Warn("PLEX_API_URL not set, skipping Plex provider")
	}

	// Add Radarr Provider
	radarrURL := os.Getenv("RADARR_API_URL")
	if radarrURL != "" {
		apiKey := os.Getenv("RADARR_API_KEY")
		skipVerify := os.Getenv("RADARR_SKIP_VERIFY") == "true"

		radarrProvider, err := radarr.NewProvider(radarrURL, apiKey, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize Radarr provider", "error", err)
		} else {
			s.AddProvider(radarrProvider)
			slog.Info("Radarr provider added", "url", radarrURL)
		}
	} else {
		slog.Warn("RADARR_API_URL not set, skipping Radarr provider")
	}

	// Add Sonarr Provider
	sonarrURL := os.Getenv("SONARR_API_URL")
	if sonarrURL != "" {
		apiKey := os.Getenv("SONARR_API_KEY")
		skipVerify := os.Getenv("SONARR_SKIP_VERIFY") == "true"

		sonarrProvider, err := sonarr.NewProvider(sonarrURL, apiKey, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize Sonarr provider", "error", err)
		} else {
			s.AddProvider(sonarrProvider)
			slog.Info("Sonarr provider added", "url", sonarrURL)
		}
	} else {
		slog.Warn("SONARR_API_URL not set, skipping Sonarr provider")
	}

	// Add Lidarr Provider
	lidarrURL := os.Getenv("LIDARR_API_URL")
	if lidarrURL != "" {
		apiKey := os.Getenv("LIDARR_API_KEY")
		skipVerify := os.Getenv("LIDARR_SKIP_VERIFY") == "true"

		lidarrProvider, err := lidarr.NewProvider(lidarrURL, apiKey, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize Lidarr provider", "error", err)
		} else {
			s.AddProvider(lidarrProvider)
			slog.Info("Lidarr provider added", "url", lidarrURL)
		}
	} else {
		slog.Warn("LIDARR_API_URL not set, skipping Lidarr provider")
	}

	// Add NZBGet Provider
	nzbgetURL := os.Getenv("NZBGET_API_URL")
	if nzbgetURL != "" {
		user := os.Getenv("NZBGET_API_USER")
		pass := os.Getenv("NZBGET_API_PASS")
		skipVerify := os.Getenv("NZBGET_SKIP_VERIFY") == "true"

		nzbgetProvider, err := nzbget.NewProvider(nzbgetURL, user, pass, skipVerify)
		if err != nil {
			slog.Error("Failed to initialize NZBGet provider", "error", err)
		} else {
			s.AddProvider(nzbgetProvider)
			slog.Info("NZBGet provider added", "url", nzbgetURL)
		}
	} else {
		slog.Warn("NZBGET_API_URL not set, skipping NZBGet provider")
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
