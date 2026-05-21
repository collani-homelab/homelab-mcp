package main

import (
	"os"
	"strings"
	"testing"
)

func unsetEnv(key string) {
	_ = os.Unsetenv(key)
}

func setEnv(key, value string) {
	_ = os.Setenv(key, value)
}

func clearUnraidEnv() {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "UNRAID_") {
			parts := strings.SplitN(env, "=", 2)
			unsetEnv(parts[0])
		}
	}
}

func clearUnifiEnv() {
	unsetEnv("UNIFI_API_URL")
	unsetEnv("UNIFI_API_KEY")
	unsetEnv("UNIFI_SKIP_VERIFY")
}

func clearMediaEnv() {
	unsetEnv("TAUTULLI_API_URL")
	unsetEnv("TAUTULLI_API_KEY")
	unsetEnv("TAUTULLI_SKIP_VERIFY")
	unsetEnv("PLEX_API_URL")
	unsetEnv("PLEX_API_TOKEN")
	unsetEnv("PLEX_SKIP_VERIFY")
	unsetEnv("RADARR_API_URL")
	unsetEnv("RADARR_API_KEY")
	unsetEnv("RADARR_SKIP_VERIFY")
	unsetEnv("SONARR_API_URL")
	unsetEnv("SONARR_API_KEY")
	unsetEnv("SONARR_SKIP_VERIFY")
	unsetEnv("LIDARR_API_URL")
	unsetEnv("LIDARR_API_KEY")
	unsetEnv("LIDARR_SKIP_VERIFY")
	unsetEnv("NZBGET_API_URL")
	unsetEnv("NZBGET_API_USER")
	unsetEnv("NZBGET_API_PASS")
	unsetEnv("NZBGET_SKIP_VERIFY")
}


func TestSetupServer_ProvidersSkipped(t *testing.T) {
	clearUnraidEnv()
	clearUnifiEnv()
	clearMediaEnv()

	s := setupServer()
	providers := s.Providers()

	        // Expecting Hello, Monitoring, Alerting, and RAG context providers since other URLs are omitted
        if len(providers) != 4 {
                t.Fatalf("expected 4 providers, got %d", len(providers))
        }

	hasHello := false
	hasRAG := false
	for _, p := range providers {
		if p.Name() == "hello" {
			hasHello = true
		} else if p.Name() == "homelab-context" {
			hasRAG = true
		}
	}

	if !hasHello {
		t.Errorf("expected hello provider to be registered")
	}
	if !hasRAG {
		t.Errorf("expected homelab-context provider to be registered")
	}
}

func TestSetupServer_ProvidersAdded(t *testing.T) {
	clearUnraidEnv()
	clearUnifiEnv()
	clearMediaEnv()

	// Set URL to a dummy value
	setEnv("UNRAID_URL", "http://localhost")
	setEnv("UNIFI_API_URL", "https://unifi")
	setEnv("UNIFI_API_KEY", "dummy-key")
	defer func() {
		clearUnraidEnv()
		clearUnifiEnv()
		clearMediaEnv()
	}()

	s := setupServer()
	providers := s.Providers()

	        // Expecting Hello, RAG context, Monitoring, Alerting, Unraid, and UniFi providers
        if len(providers) != 6 {
                t.Fatalf("expected 6 providers, got %d", len(providers))
        }

	hasHello := false
	hasRAG := false
	hasUnraid := false
	hasUnifi := false

	for _, p := range providers {
		if p.Name() == "hello" {
			hasHello = true
		} else if p.Name() == "homelab-context" {
			hasRAG = true
		} else if len(p.Name()) >= 6 && p.Name()[:6] == "Unraid" {
			hasUnraid = true
		} else if p.Name() == "UniFi" {
			hasUnifi = true
		}
	}

	if !hasHello {
		t.Error("expected hello provider to be registered")
	}
	if !hasRAG {
		t.Error("expected homelab-context provider to be registered")
	}
	if !hasUnraid {
		t.Error("expected Unraid provider to be registered")
	}
	if !hasUnifi {
		t.Error("expected UniFi provider to be registered")
	}
}
