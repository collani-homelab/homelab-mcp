package main

import (
	"context"
	"homelab-mcp/internal/mcp"
	"homelab-mcp/internal/provider/hello"
	"log/slog"
	"os"
)

func main() {
	// Configure slog to write to Stderr
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	s := mcp.NewServer("homelab-mcp", "0.1.0")

	// Add providers
	s.AddProvider(&hello.HelloProvider{})

	slog.Info("Starting Homelab MCP Server", "version", "0.1.0")

	// Run the server
	if err := s.Run(context.Background()); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
