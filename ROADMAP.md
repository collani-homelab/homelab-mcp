# Homelab MCP Roadmap

## Completed Work

All four active phases are complete. The server is deployed as a daemonized Docker container on the SRE machine (192.168.99.178:8083) using SSE transport, backed by a local Docker registry and a CI/CD Golden Path pipeline.

**Phase 1 & 2 — Read-Only Foundation**
- Go module, `mise` toolchain, MCP skeleton with hello-world resource.
- Unraid provider: containers, system stats, array health, VMs, UPS, notifications, syslog, container-log resource templates.
- UniFi provider: active clients, devices, network health, alarms, PoE status.
- MCP Prompts (`homelab_status_report`), JSON pruning (`provider.PruneJSON`), slog-to-stderr logging discipline.

**Phase 3 — Media Stack & Hosting**
- SSE transport added alongside stdio.
- Media providers: Tautulli, Plex, Radarr, Sonarr, Lidarr, NZBGet.
- `media_stack_status` global prompt.
- `ToolBuilder` pattern (`internal/mcp/toolbuilder.go`) to reduce provider boilerplate.
- Multi-stage `scratch`-based Dockerfile, `docker-compose.yml`, `platform.json`, CI/CD via `homelab-platform` Golden Path.

**Phase 4 — Telemetry & Memory Context**
- `monitoring` provider: `query_promql` (Prometheus) and `query_logql` (Loki, with explicit `lookback` window).
- `context` (RAG) provider: `query_knowledge` and `index_document` against Qdrant via the homelab-context service.
- `alerting` provider: `send_notification` via ntfy.sh.

---

## Won't Do — Phase 5: Prompt-to-Tool Mapping

**Original goal:** Dynamically expose all MCP Prompts as standard MCP Tools so clients like Open WebUI and LibreChat (which lack native prompt UI support) could invoke them.

**Decision:** Skip. The two defined prompts (`homelab_status_report`, `media_stack_status`) are just canned text instructions with no parameters or logic — a capable LLM will self-direct to the same outcome without them. The dynamic mapping layer would be non-trivial engineering for negligible real-world gain. If Open WebUI becomes a primary client in the future, the simpler fix is to add two explicit no-argument tools directly in `server.go` rather than building a generic mapping layer.
