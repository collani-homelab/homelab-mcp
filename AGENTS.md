# homelab-mcp AI Agent Context (AGENTS.md)

This document is the primary context layer for AI sub-agents operating in this repository. It supplements `ARCHITECTURE.md` with explicit execution rules and AI guardrails per the Spec-Driven Development standard in `GLOBAL_CLAUDE.md`.

## 1. Repository Purpose & Scope

This repository is the Model Context Protocol (MCP) server for the homelab. It exposes live homelab state to AI assistants via the MCP protocol, covering infrastructure, media stack, telemetry, and alerting.

- **Primary Language:** Go 1.22+
- **Toolchain:** `mise` for environment and task management
- **Deployment:** Docker container via SSE transport (default port 8083); registry and deploy paths configured via `IMAGE_TAG` and `DEPLOY_WEBHOOK_URL` env vars

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
| Deploy | `provider/deploy` | homelab-deploy webhook at `DEPLOY_WEBHOOK_URL` (port 8084) |

## 4. Platform & Deployment (The Golden Path)

- **`platform.json`:** Intended as service metadata for `homelab-platform`, but as of 2026-06-16 nothing in either repo actually reads it — grep turns up no consumer. Treat its `environment` list as aspirational/stale, not a source of truth (see `task.md`).
- **CI/CD:** Push to `main` → GitHub Actions self-hosted runner on the SRE machine → `mise run docker-build` → `mise run docker-push` → `homelab-deploy -deploy homelab-mcp`. No manual steps needed — **when it's working.** As of 2026-06-16 the Deploy workflow queues forever (public repo + a runner group with `allows_public_repositories: false`, plus missing `REGISTRY_URL`/`PLATFORM_REPO_PATH` repo variables). See ROADMAP.md "Known Issues" before assuming a push deployed anything.
- **Secrets:** Bound via `.env` on the host. Never committed. Must include `IMAGE_TAG` (registry-qualified, e.g. `192.168.99.178:5000/homelab-mcp:latest`) — without it the systemd-managed `docker compose up --pull always` falls back to the bare `homelab-mcp:latest` tag and fails to pull from Docker Hub.
- **Compose file:** `docker-compose.yml` in the repo root. The SRE machine's copy is updated automatically via `git pull` on service restart (defined in `sre-machine.json`).
- **Agents must not do manual deploy flows** when CI/CD is actually working. Make changes, commit, push — the pipeline owns the deploy. Use `redeploy_service` only for force-restarts without a code change. While the Deploy workflow above is broken, manual deploy (`mise run docker-build && mise run docker-push` with `IMAGE_TAG` set, then `homelab-deploy -deploy homelab-mcp -repo <homelab-platform repo path>`) is the only path — confirm with the user before doing this, since it bypasses the intended pipeline.

## 5. Using the MCP Server from Claude Code

When exercising the homelab-mcp server from a Claude Code session:

- **Always warm the session first.** After loading tool schemas (ToolSearch) or starting a fresh session, make a single tool call before firing parallel batches. The SSE handshake may not be complete yet — firing a large batch cold causes all calls to fail with "Tool result missing due to internal error", which then disrupts the session state.
- **Parallel batches of 13+ are safe once warm.** The go-sdk SSE server handles high concurrency fine. The failure mode is a client-side timing issue, not a server concurrency limit. Confirmed via controlled escalation testing (1→2→4→6→8→10→13 simultaneous calls).
- **Use system stats as the warmup call.** `get_unraid_system_stats_dionysus` is fast and has no side effects — ideal for confirming the session is live.

## 6. Triggering Redeploys

The `redeploy_service` tool (Deploy provider) allows agents to restart any service defined in `sre-machine.json` without SSH access or a code push.

```
redeploy_service(service_name="homelab-mcp")
```

This calls the deploy webhook at `DEPLOY_WEBHOOK_URL/deploy/<service>`, which runs `homelab-deploy -deploy <service>` on the SRE machine. Use it when:
- A config file was changed but no code changed
- A service crashed and needs a forced restart
- A new image was manually pushed to the registry outside of CI/CD

**The deploy webhook must be running** on the SRE machine (systemd service `homelab-deploy-webhook`, port 8084). If `DEPLOY_WEBHOOK_URL` is not set, the tool returns a clear error rather than silently failing.

## 7. Current State

All planned phases are complete (see `ROADMAP.md`). The server is stable and in maintenance/expansion mode. New work should be incremental provider additions or tool improvements, not architectural changes.

**Do not** implement a dynamic Prompt-to-Tool mapping layer (Phase 5 was explicitly rejected — see ROADMAP.md for reasoning).
