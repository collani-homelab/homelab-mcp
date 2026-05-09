# Homelab MCP Server (Go)

A Model Context Protocol (MCP) server written in Go to provide AI assistants with context from my homelab environment, currently targeting Unraid and UniFi.

## Current Status

- [x] **Step 1: Setup** - Initialized Go module and `mise.toml`.
- [x] **Step 2: Skeleton** - Implemented basic MCP server with `stdio` transport and a "Hello World" provider.
- [x] **Step 3: Testing Setup** - Unit tests implemented; MCP Inspector verification working.
- [x] **Step 4: Unraid Integration** - GraphQL client for Docker containers.
- [x] **Step 5: UniFi Integration** - Network client for active clients and devices.
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
│       ├── unraid/     # Unraid GraphQL client
│       └── unifi/      # UniFi API client
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

The server dynamically loads providers based on your environment variables. 
Copy `.env.example` to `.env` and fill in your credentials.

### Unraid Configuration
The Unraid provider supports multiple servers. Prefix the variables with `UNRAID_<NAME>_`.

```env
# Example for a server named "dionysus"
UNRAID_DIONYSUS_URL=http://192.168.1.100/graphql
UNRAID_DIONYSUS_KEY=your_api_key
UNRAID_DIONYSUS_SKIP_VERIFY=true

# Example for a server named "archive"
UNRAID_ARCHIVE_URL=http://192.168.1.101/graphql
UNRAID_ARCHIVE_KEY=your_api_key
```

### UniFi Configuration
You must authenticate using a Local API Key. Do not use UI.com cloud credentials or legacy username/password.

```env
UNIFI_API_URL=https://192.168.1.1
UNIFI_API_KEY=your_local_api_key
UNIFI_SKIP_VERIFY=true
```
