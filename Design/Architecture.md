# Homelab MCP Architecture & Design

## Overview
This document outlines the high-level architecture and design decisions for the Homelab Model Context Protocol (MCP) server.

## 1. Development Environment
Instead of Nix, we use `mise` for toolchain management.
- **Tools**: Go 1.22+, `golangci-lint`
- **Config**: `.mise.toml` to track versions and environment variables.

## 2. Project Architecture
```text
homelab-mcp/
├── cmd/
│   └── server/          # Main entry point
├── internal/
│   ├── mcp/            # MCP protocol implementation & handlers
│   └── provider/       # Homelab integrations
│       ├── provider.go # Interface definitions
│       ├── unraid/     # Unraid GraphQL client
│       └── unifi/      # UniFi API client
└── .mise.toml          # Tooling & Environment
```

## 3. Core Interface: The Provider
To keep the design "KISS", every integration must implement a simple interface:
```go
type Provider interface {
	Name() string
	GetResources() ([]mcp.Resource, error)
	GetResourceContent(uri string) (string, error)
	GetResourceTemplates() ([]mcp.ResourceTemplate, error)
	GetPrompts() ([]mcp.Prompt, error)
	GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error)
}
```

## 4. Security & Safety
- **Transport**: Stdio-based transport (local execution only) for initial phases. Planned migration to SSE.
- **Credentials**: Managed via `.env` (loaded by `mise`) or system environment variables.
- **Permissions**: Ensure API tokens used are restricted to read-only scopes where supported.

## 5. Key Considerations & Gotchas
- **Logging (`stdio` transport)**: **Never write logs to `stdout`**. Doing so corrupts the JSON-RPC stream. Configure `slog` to write exclusively to `os.Stderr` or a file.
- **Timeouts**: Wrap network calls to Unraid/UniFi with `context.WithTimeout`. If a homelab service is offline, the MCP resource request should fail fast rather than hanging the AI client.
- **Graceful Degradation**: If one provider (e.g., UniFi) fails to initialize or fetch data, the server should log the error but continue serving functional providers (e.g., Unraid).
- **JSON Pruning**: Raw APIs (like UniFi) return massive payloads with UUIDs and internal noise. Use `provider.PruneJSON` to filter out token-wasting fields before sending payloads to the AI.

## 6. Testing Strategy
- **Unit Tests (`internal/provider`)**: Standard Go `*testing.T` tests using mocked HTTP/GraphQL responses to verify that homelab data is correctly parsed into `mcp.Resource` structs.
- **Integration Tests (`internal/mcp`)**: Invoke the server's routing handlers programmatically with mock JSON-RPC requests.
- **MCP Inspector (Manual/E2E)**: Use Anthropic's official inspector (`npx @modelcontextprotocol/inspector`) to visually test the `stdio` transport.
