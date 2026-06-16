# Homelab MCP Roadmap

## Completed Work

Phases 1–6 are complete. The server is deployed as a daemonized Docker container using SSE transport, backed by a local Docker registry and a CI/CD Golden Path pipeline. Four autonomous agent workflows run via Dagu.

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

**Phase 5 — Orchestration & Observability**
- `dagu` provider: `list_dags`, `get_dag`, `trigger_dag`, `stop_dag`, `retry_dag` — exposes Dagu DAG state and control as MCP tools.
- `grafana` provider: `list_grafana_dashboards`, `get_grafana_dashboard`, `get_grafana_alerts` — surfaces dashboard panels and firing alerts as MCP resources.
- `deploy` provider: `redeploy_service` — triggers homelab-deploy webhook for systemd service restarts from MCP.

**Phase 7 (partial) — LLM Observability**
- `phoenix` provider: `phoenix://projects`, `phoenix://traces/{project}`, `phoenix://evaluations/{project}` resources; `query_phoenix_traces`, `get_phoenix_eval_scores`, `get_phoenix_span_errors` tools — surfaces Arize Phoenix span/trace/eval data (model, latency, status, annotation scores) so agents can introspect their own past LLM calls.

**Phase 6 — Agentic Workflows (Milestone D)**
- Four containerized agent images built and pushed to a local Docker registry:
  - `homelab-agent-sre-patrol` — polls Unraid, UPS, and Prometheus every 15 min; alerts on threshold breaches.
  - `homelab-agent-storage-report` — weekly capacity report with projected fill dates and LLM narrative.
  - `homelab-agent-media-health` — daily check of NZBGet, Sonarr, Radarr, Tautulli pipeline health.
  - `homelab-agent-network-sentinel` — 5-min UniFi poll against a known-device allowlist; alerts on new MACs.
- All four agents run as Dagu DAGs via `docker run` steps; custom `homelab-dagu` image includes docker CLI.
- `build-agents.sh` in `homelab/Tools/` builds and pushes all four images to the local registry.

---

## Planned — Phase 7: LLM Observability & UI Providers (remaining)

These providers extend MCP coverage to the local LLM inference stack and its tooling. The Phoenix piece of Phase 7 is done (see Completed Work above); `litellm` and `openwebui` remain.

**`internal/provider/litellm/`**
- Resources: `litellm://models` (routing table), `litellm://usage` (token spend by model, last 24h/7d).
- Tools: `query_model_usage` — token counts, latency percentiles, error rates per model.
- Value: lets agents and patrol scripts detect context-limit violations, routing failures, and cost anomalies.

**`internal/provider/openwebui/`** (Open WebUI)
- Resources: `openwebui://users`, `openwebui://models/active`.
- Tools: `query_usage` — per-user session counts, model selection patterns, recent conversations.
- Value: usage analytics without context-switching; useful for understanding which local models are actually being used.

All three follow the existing Provider interface in `internal/provider/provider.go`.

---

## Won't Do — Prompt-to-Tool Mapping

**Original goal:** Dynamically expose all MCP Prompts as standard MCP Tools so clients like Open WebUI and LibreChat (which lack native prompt UI support) could invoke them.

**Decision:** Skip. The two defined prompts (`homelab_status_report`, `media_stack_status`) are just canned text instructions with no parameters or logic — a capable LLM will self-direct to the same outcome without them. The dynamic mapping layer would be non-trivial engineering for negligible real-world gain. If Open WebUI becomes a primary client in the future, the simpler fix is to add two explicit no-argument tools directly in `server.go` rather than building a generic mapping layer.
