package main

import (
	"os"
	"strings"
	"testing"
)

func clearUnraidEnv() {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "UNRAID_") {
			parts := strings.SplitN(env, "=", 2)
			os.Unsetenv(parts[0])
		}
	}
}

func clearUnifiEnv() {
	os.Unsetenv("UNIFI_API_URL")
	os.Unsetenv("UNIFI_API_KEY")
	os.Unsetenv("UNIFI_SKIP_VERIFY")
}

func clearMediaEnv() {
	os.Unsetenv("TAUTULLI_API_URL")
	os.Unsetenv("TAUTULLI_API_KEY")
	os.Unsetenv("TAUTULLI_SKIP_VERIFY")
	os.Unsetenv("PLEX_API_URL")
	os.Unsetenv("PLEX_API_TOKEN")
	os.Unsetenv("PLEX_SKIP_VERIFY")
	os.Unsetenv("RADARR_API_URL")
	os.Unsetenv("RADARR_API_KEY")
	os.Unsetenv("RADARR_SKIP_VERIFY")
	os.Unsetenv("SONARR_API_URL")
	os.Unsetenv("SONARR_API_KEY")
	os.Unsetenv("SONARR_SKIP_VERIFY")
	os.Unsetenv("LIDARR_API_URL")
	os.Unsetenv("LIDARR_API_KEY")
	os.Unsetenv("LIDARR_SKIP_VERIFY")
	os.Unsetenv("NZBGET_API_URL")
	os.Unsetenv("NZBGET_API_USER")
	os.Unsetenv("NZBGET_API_PASS")
	os.Unsetenv("NZBGET_SKIP_VERIFY")
}


func TestSetupServer_ProvidersSkipped(t *testing.T) {
	clearUnraidEnv()
	clearUnifiEnv()
	clearMediaEnv()

	s := setupServer()
	providers := s.Providers()

	// Expecting only the Hello provider since URL is omitted
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}

	if providers[0].Name() != "hello" {
		t.Errorf("expected hello provider, got %s", providers[0].Name())
	}
}

func TestSetupServer_ProvidersAdded(t *testing.T) {
	clearUnraidEnv()
	clearUnifiEnv()
	clearMediaEnv()

	// Set URL to a dummy value
	os.Setenv("UNRAID_URL", "http://localhost")
	os.Setenv("UNIFI_API_URL", "https://unifi")
	os.Setenv("UNIFI_API_KEY", "dummy-key")
	defer func() {
		clearUnraidEnv()
		clearUnifiEnv()
		clearMediaEnv()
	}()

	s := setupServer()
	providers := s.Providers()

	// Expecting Hello, Unraid, and UniFi providers
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(providers))
	}

	hasHello := false
	hasUnraid := false
	hasUnifi := false

	for _, p := range providers {
		if p.Name() == "hello" {
			hasHello = true
		} else if len(p.Name()) >= 6 && p.Name()[:6] == "Unraid" {
			hasUnraid = true
		} else if p.Name() == "UniFi" {
			hasUnifi = true
		}
	}

	if !hasHello {
		t.Error("expected hello provider to be registered")
	}
	if !hasUnraid {
		t.Error("expected Unraid provider to be registered")
	}
	if !hasUnifi {
		t.Error("expected UniFi provider to be registered")
	}
}
