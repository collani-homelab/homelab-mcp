# Implementation Plan: Homelab MCP Server (Go)

## Overview
This document outlines the concrete steps to build a Read-Only Model Context Protocol (MCP) server in Go to interface with Unraid and UniFi.

## 1. Development Environment (via `mise`)
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
}
```

## 4. Phase 1: Read-Only Implementation

### Unraid (GraphQL)
- **Goal**: Fetch server status and Docker container list.
- **Endpoint**: `/graphql` (Native in 7.3+; requires *Unraid Connect* plugin on 7.2.x).
- **Resource URI**: `unraid://containers`
- **Note**: Current servers are on 7.2.3. Unraid 7.3 is currently in **RC2**. Monitor for GA release to transition from the *Unraid Connect* plugin requirement to the native system API.

### UniFi (Network API)
- **Goal**: Fetch active client list and network health.
- **Endpoint**: `integration/v1` via UniFi Controller.
- **Resource URI**: `unifi://clients`

## 5. Security & Safety
- **Transport**: Stdio-based transport (local execution only).
- **Credentials**: Managed via `.env` (loaded by `mise`) or system environment variables.
- **Permissions**: Ensure API tokens used are restricted to read-only scopes where supported.

## 6. Key Considerations & Gotchas
- **Logging (`stdio` transport)**: **Never write logs to `stdout`**. Doing so corrupts the JSON-RPC stream. Configure `slog` to write exclusively to `os.Stderr` or a file.
- **Timeouts**: Wrap network calls to Unraid/UniFi with `context.WithTimeout`. If a homelab service is offline, the MCP resource request should fail fast rather than hanging the AI client.
- **Graceful Degradation**: If one provider (e.g., UniFi) fails to initialize or fetch data, the server should log the error but continue serving functional providers (e.g., Unraid).

## 7. Testing Strategy
- **Unit Tests (`internal/provider`)**: Standard Go `*testing.T` tests using mocked HTTP/GraphQL responses to verify that homelab data is correctly parsed into `mcp.Resource` structs.
- **Integration Tests (`internal/mcp`)**: Invoke the server's routing handlers programmatically with mock JSON-RPC requests to verify the protocol layer correctly calls the underlying Provider.
- **MCP Inspector (Manual/E2E)**: Use Anthropic's official `npx @modelcontextprotocol/inspector` to launch the Go binary and visually test the `stdio` transport and resource fetching before connecting an actual AI client. *(Note: Requires a Chromium-based browser; Firefox strict CORS blocks the proxy).*

## 8. Iteration Roadmap
1. **Setup**: Initialize Go module and `mise.toml`. [x]
2. **Skeleton**: Implement basic MCP server that responds with a "Hello World" resource. [x]
3. **Testing Setup**: Verify the skeleton works using the MCP Inspector and write the first unit test. [x]
4. **Unraid**: Implement the Unraid Provider fetching the Docker list. [x]
5. **UniFi**: Implement the UniFi Provider fetching the Client list. [x]
6. **Refinement**: Add logging and error handling using Go's `slog` package. [x]

## 9. Phase 2: Iteration & Feature Expansion

### Goals
- Deepen the context available from both Unraid and UniFi.
- Implement MCP **Prompts** to guide AI discovery.
- Optimize data transmission for token efficiency.

### Unraid Expansion
- **System Stats**: Add `unraid://system/stats` for CPU, RAM, and Uptime.
- **Array Health**: Add `unraid://array/status` for parity info and disk health.
- **Resource Templates**: Support `unraid://containers/{name}/logs` to fetch recent logs.

### UniFi Expansion
- **Network Health**: Add `unifi://network/health` for ISP speeds and latency.
- **PoE Status**: Add `unifi://switches/poe` to monitor power budget.
- **WiFi Experience**: Include experience scores in the client list.

### New MCP Capabilities
- **Prompts**: 
    - `Homelab Status Report`: A pre-defined prompt that fetches high-level health from all providers.
    - `Troubleshoot Client`: A prompt that helps diagnose why a specific MAC address might be having issues.
- **Token Pruning**: Implement a "Summary" view for JSON responses that removes internal noise (UUIDs, redundant flags) before sending to the AI.

## 10. Iteration Roadmap (Phase 2)
7. **Expansion**: Add System Stats and Array Health to Unraid. [x]
8. **Insights**: Add Network Health and PoE status to UniFi. [x]
9. **Guidance**: Implement the first set of MCP Prompts. [x]
10. **Optimization**: Implement AI-friendly JSON pruning. [x]