# Homelab MCP Server (Go)

A Model Context Protocol (MCP) server written in Go to provide AI assistants with context from a homelab environment, specifically targeting Unraid and UniFi.

## Current Status

- [x] **Step 1: Setup** - Initialized Go module and `mise.toml`.
- [x] **Step 2: Skeleton** - Implemented basic MCP server with `stdio` transport and a "Hello World" provider.
- [x] **Step 3: Testing Setup** - Unit tests implemented; MCP Inspector verification working.
- [ ] **Step 4: Unraid Integration** - GraphQL client for Docker containers.
- [ ] **Step 5: UniFi Integration** - Network client for active clients.
- [ ] **Step 6: Refinement** - Enhanced logging and error handling.

## Architecture

The project follows a provider-based architecture to keep integrations modular and easy to test.

```text
homelab-mcp/
├── cmd/
│   └── server/          # Main entry point
├── internal/
│   ├── mcp/            # MCP protocol implementation & handlers
│   └── provider/       # Homelab integrations
│       ├── provider.go # Interface definitions
│       ├── hello/      # Simple test provider
│       ├── unraid/     # Unraid GraphQL client (Coming Soon)
│       └── unifi/      # UniFi API client (Coming Soon)
└── .mise.toml          # Tooling & Environment
```

## Getting Started

### Prerequisites

- Go 1.22+
- [mise](https://mise.jdx.sh/) (optional, for toolchain management)

### Building

```bash
mise run build
```

### Running

The server uses `stdio` transport. You can run it directly, but it expects JSON-RPC input.

```bash
mise run run
```

### Testing

Run unit tests:

```bash
mise run test
```

To test with the MCP Inspector:

```bash
mise run inspector
```

## Environment Variables

Copy `.env.example` to `.env` and fill in your credentials (required for future providers).

```bash
cp .env.example .env
```
