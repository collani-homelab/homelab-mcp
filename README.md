# Homelab MCP Server

A Model Context Protocol (MCP) server written in Go that gives AI assistants live context from the homelab: infrastructure state, media stack activity, metrics, logs, and notifications.

The server runs as a Docker container on the SRE machine (`192.168.99.178:8083`) using SSE transport. For design decisions and codebase structure, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Capabilities

### Infrastructure — Unraid (multi-server)

Resources and tools are namespaced per server (e.g. `dionysus`, `archive`).

| Resource / Tool | Description |
|---|---|
| `unraid://{name}/containers` | Docker container list, state, autostart |
| `unraid://{name}/system/stats` | CPU, memory, OS, uptime |
| `unraid://{name}/array/status` | Array state, parity check, disk inventory |
| `unraid://{name}/vms` | Virtual machine list and state |
| `unraid://{name}/system/ups` | UPS battery, runtime, load |
| `unraid://{name}/notifications` | Unread system notifications |
| `unraid://{name}/system/syslog` | System log tail |
| `unraid://{name}/containers/{name}/logs` | Container log tail (resource template) |
| `get_unraid_system_stats_{name}` tool | Richer CPU/memory/OS snapshot |
| `get_unraid_array_status_{name}` tool | Array state + per-disk temp/errors/spin |
| `get_unraid_containers_{name}` tool | Containers with ports, log size, update flags |
| `get_unraid_ups_status_{name}` tool | UPS with current power draw |

### Infrastructure — UniFi

| Resource / Tool | Description |
|---|---|
| `unifi://clients` | Active network clients |
| `unifi://devices` | Network infrastructure devices |
| `unifi://network/health` | ISP and network health summary |
| `unifi://switches/poe` | PoE port power and status |
| `unifi://network/alarms` | Active network alarms |
| `get_unifi_clients` tool | Pruned active-client snapshot |
| `get_unifi_network_health` tool | Network health snapshot |

### Media Stack

| Resource | Description |
|---|---|
| `tautulli://activity` | Current Plex sessions via Tautulli |
| `tautulli://history` | Recent play history |
| `plex://sessions` | Active Plex streams |
| `plex://servers` | Connected Plex servers |
| `radarr://queue` | Movie download queue |
| `radarr://system/status` | Radarr health |
| `radarr://movie/missing` | Missing movie subset |
| `sonarr://queue` | TV episode download queue |
| `sonarr://system/status` | Sonarr health |
| `sonarr://series` | Full tracked series list |
| `lidarr://queue` | Music download queue |
| `lidarr://system/status` | Lidarr health |
| `lidarr://artist` | Full tracked artist list |
| `nzbget://status` | NZBGet speed, pause state, remaining data |
| `nzbget://listgroups` | Active download items |
| `nzbget://history` | Recent download history |

### Telemetry & Observability

| Tool | Description |
|---|---|
| `query_promql` | Execute a PromQL query against Prometheus |
| `query_logql` | Execute a LogQL query against Loki (optional `lookback` duration, default `1h`) |

### Context & Alerting

| Tool | Description |
|---|---|
| `query_knowledge` | Semantic search over homelab architecture docs in Qdrant |
| `index_document` | Index a document chunk into the Qdrant knowledge base |
| `send_notification` | Push a notification via ntfy.sh |

### Prompts

| Prompt | Description |
|---|---|
| `homelab_status_report` | Instructs the AI to fetch Unraid system stats and UniFi network health and summarize |
| `media_stack_status` | Instructs the AI to read all media stack resources and produce a 3-section report |

---

## Getting Started

### Prerequisites

- Go 1.22+
- [mise](https://mise.jdx.sh/) for toolchain management
- Docker (for container builds and the SSE deployment)

### Build & Run

```bash
# Build the binary
mise run build

# Run locally via stdio transport
mise run run

# Run locally via SSE transport
MCP_TRANSPORT=sse PORT=8080 mise run run

# Build and run Docker container (SSE on port 8080)
mise run docker-run

# Run tests
mise run test

# Visual E2E test with MCP Inspector
mise run inspector
```

---

## Configuration

Copy `.env.example` to `.env` and fill in credentials. Providers are enabled dynamically — if a variable is not set, that provider is skipped with a warning at startup.

### Unraid (multi-server)

```env
UNRAID_DIONYSUS_URL=http://192.168.1.100/graphql
UNRAID_DIONYSUS_KEY=your_api_key
UNRAID_DIONYSUS_SKIP_VERIFY=true

UNRAID_ARCHIVE_URL=http://192.168.1.101/graphql
UNRAID_ARCHIVE_KEY=your_api_key
```

### UniFi

```env
UNIFI_API_URL=https://192.168.1.1
UNIFI_API_KEY=your_local_api_key   # Local API key only — not UI.com credentials
UNIFI_SKIP_VERIFY=true
```

### Media Stack

```env
TAUTULLI_API_URL=http://host:8181
TAUTULLI_API_KEY=your_key

PLEX_API_URL=http://host:32400
PLEX_API_TOKEN=your_token

RADARR_API_URL=http://host:7878
RADARR_API_KEY=your_key

SONARR_API_URL=http://host:8989
SONARR_API_KEY=your_key

LIDARR_API_URL=http://host:8686
LIDARR_API_KEY=your_key

NZBGET_API_URL=http://host:6789
NZBGET_API_USER=your_user
NZBGET_API_PASS=your_pass
```

### Telemetry & Context

```env
PROMETHEUS_URL=http://host:9090
LOKI_URL=http://host:3100
HOMELAB_CONTEXT_URL=http://host:8081
```

### Meaningless section to trigger CI
Hello