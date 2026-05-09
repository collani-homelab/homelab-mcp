# Homelab MCP Server (Go)

A Model Context Protocol (MCP) server written in Go to provide AI assistants with context from my homelab environment, currently targeting Unraid and UniFi.

## Current Status & Roadmap

We are currently working on **Phase 3** (Media Stack Expansion & Continuous Deployment). 

* For detailed task tracking and historical progress, see [Design/Roadmap.md](Design/Roadmap.md).
* For high-level design decisions and codebase structure, see [Design/Architecture.md](Design/Architecture.md).

## Capabilities

### Resources
- `unraid://{name}/containers` - Docker containers list and status.
- `unraid://{name}/system/stats` - Server uptime and memory stats.
- `unraid://{name}/array/status` - Parity disk status and array health.
- `unifi://clients` - Active network clients.
- `unifi://devices` - UniFi network infrastructure devices.
- `unifi://network/health` - High-level ISP and network health.
- `unifi://switches/poe` - PoE switch power and port status.

### Resource Templates
- `unraid://{name}/containers/{name}/logs` - *Upcoming*: Docker container logs (currently routing correctly but waiting on Unraid GraphQL support).

### Prompts
- `homelab_status_report`: A pre-defined prompt that instructs the AI to fetch and summarize the Unraid system stats and UniFi network health.
- `troubleshoot_client`: Requires a `mac` argument. Instructs the AI to query the UniFi clients list to help diagnose network issues for a specific device.


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
