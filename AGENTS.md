# homelab-mcp AI Agent Context (AGENTS.md)

This document is the primary context layer for AI sub-agents operating in this repository. It supplements `ARCHITECTURE.md` with explicit execution rules and AI guardrails per the Spec-Driven Development standard in `GLOBAL_CLAUDE.md`.

## 1. Repository Purpose & Scope

This repository is the Model Context Protocol (MCP) server for the homelab. It exposes live homelab state to AI assistants via the MCP protocol, covering infrastructure, media stack, telemetry, and alerting.

- **Primary Language:** Go 1.22+
- **Toolchain:** `mise` for environment and task management
- **Deployment:** Docker container on the SRE machine (`192.168.99.178:8083`), SSE transport, local registry at `192.168.99.178:5000`

## 2. High-Level Architecture

- **Provider Pattern:** Every integration implements the `Provider` interface in `internal/provider/provider.go`. Resources = read-only state. Tools = actions/mutations.
- **Tool Builder:** Use `internal/mcp/toolbuilder.go` (`NewTool`, `TextResult`, `ErrorResult`) for all new tools — do not hand-roll `mcp.Tool` structs.
- **JSON Pruning:** Strip noise fields from large API payloads using `provider.PruneJSON` before returning data to the LLM.
- **Fail-Fast Timeouts:** All external calls must use `context.WithTimeout` (default 10s).
- **Graceful Degradation:** Provider init failures are logged but must not crash the server. Other providers continue serving.
- **No stdout Logging:** Under `stdio` transport, `os.Stdout` is the JSON-RPC stream. All logs go to `os.Stderr` via `slog`.

## 3. Active Providers

| Provider | Package | Backends |
|---|---|---|
| Unraid (multi-server) | `provider/unraid` | GraphQL API, dynamic from `UNRAID_<NAME>_*` env vars |
| UniFi | `provider/unifi` | REST API via `UNIFI_API_*` env vars |
| Tautulli | `provider/tautulli` | REST API |
| Plex | `provider/plex` | REST API |
| Radarr | `provider/radarr` | REST API |
| Sonarr | `provider/sonarr` | REST API |
| Lidarr | `provider/lidarr` | REST API |
| NZBGet | `provider/nzbget` | JSON-RPC API |
| Monitoring | `provider/monitoring` | Prometheus (PromQL) + Loki (LogQL) |
| Context (RAG) | `provider/context` | homelab-context service (Qdrant) via `HOMELAB_CONTEXT_URL` |
| Alerting | `provider/alerting` | ntfy.sh |

## 4. Platform & Deployment (The Golden Path)

- **`platform.json`:** Service metadata consumed by `homelab-platform` for automated deployment.
- **CI/CD:** GitHub Actions workflows build and push the Docker image to the local private registry.
- **Secrets:** Bound via `.env` on the host. Never committed.
- **Compose file:** `docker-compose.yml` in the repo root. The server-side copy lives at `/home/wcollani/repos/homelab-mcp/docker-compose.yml` on the SRE machine.

## 5. Using the MCP Server from Claude Code

When exercising the homelab-mcp server from a Claude Code session:

- **Always warm the session first.** After loading tool schemas (ToolSearch) or starting a fresh session, make a single tool call before firing parallel batches. The SSE handshake may not be complete yet — firing a large batch cold causes all calls to fail with "Tool result missing due to internal error", which then disrupts the session state.
- **Parallel batches of 13+ are safe once warm.** The go-sdk SSE server handles high concurrency fine. The failure mode is a client-side timing issue, not a server concurrency limit. Confirmed via controlled escalation testing (1→2→4→6→8→10→13 simultaneous calls).
- **Use system stats as the warmup call.** `get_unraid_system_stats_dionysus` is fast and has no side effects — ideal for confirming the session is live.

## 6. Current State

All planned phases are complete (see `ROADMAP.md`). The server is stable and in maintenance/expansion mode. New work should be incremental provider additions or tool improvements, not architectural changes.

**Do not** implement a dynamic Prompt-to-Tool mapping layer (Phase 5 was explicitly rejected — see ROADMAP.md for reasoning).
