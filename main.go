package main

import (
	"context"
	"log/slog"
	"os"

	"homelab-mcp/internal/mcp"
	"homelab-mcp/internal/provider"
)

func main() {
	// Initialize the MCP server implementation
	srv := mcp.NewServer("homelab-mcp", "0.1.0")

	// Wire up the Hello World provider
	// (Note: You may need to adjust hello.go's method signatures to drop 
	// context.Context to strictly match how server.go calls them)
	srv.AddProvider(&provider.HelloProvider{})

	// Run the server using stdio transport
	if err := srv.Run(context.Background()); err != nil {
		slog.Error("MCP Server failed to run", "error", err)
		os.Exit(1)
	}
}
