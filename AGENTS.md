# homelab-mcp AI Agent Context (AGENTS.md)

This document serves as the primary context layer for AI sub-agents operating within this repository. It supplements the human-readable `ARCHITECTURE.md` and `ROADMAP.md` files by providing explicit execution rules and AI guardrails as part of the Spec-Driven Development standard defined in `GLOBAL_CLAUDE.md`.

## 1. Repository Purpose & Scope
This repository contains the Model Context Protocol (MCP) server for the homelab. It acts as the bridge exposing homelab state (Unraid, UniFi, Media Stack) to the AI orchestrator layer.

- **Primary Language:** Go 1.22+
- **Toolchain:** `mise` for environment management.
- **Role:** Data provider for the "Mob of Experts" pipeline.

## 2. High-Level Architecture
- **Provider Pattern:** Every integration (Unraid, UniFi, Plex, Starr apps) implements the `Provider` interface located in `internal/provider/provider.go`. 
  - *Resources:* Read-only state.
  - *Tools:* Actions/Mutations.
- **Safety First:** We enforce a strict "Fail-Fast" timeout policy (default 10s) on homelab service calls to avoid hanging the AI client.
- **JSON Pruning:** Massive raw JSON payloads (especially from UniFi) are stripped of unnecessary UUIDs and metrics using `provider.PruneJSON` to preserve token limits.
- **No stdout Logging:** When running the `stdio` transport, the application strictly avoids logging to `os.Stdout` to prevent JSON-RPC stream corruption. Logs go to `stderr`.

## 3. Platform & Deployment (The Golden Path)
This repository cannot magically deploy itself. It is gated by the declarative infrastructure standard in `homelab-platform`.
- **`platform.json`:** Defines the service metadata (port 8080, language, required environment variables).
- **CI/CD:** `ci.yml` and `deploy.yml` GitHub action workflows push Docker images to the local private registry (`192.168.99.178:5000`).
- **Secrets:** Bound via `/etc/homelab/homelab-mcp.env` on the SRE machine. We do not commit secrets.

## 4. Current Objectives & Roadmap
As defined by the global `ROADMAP.md`, we are currently transitioning from local CLI-only execution to a central deployed service.

**Completed Phases:**
- **Phase 1 & 2:** Read-only foundation for Unraid and UniFi. Native MCP Prompts.
- **Phase 4:** Telemetry (PromQL/LogQL) and Alerting (ntfy.sh).

**Active/Next Phases:**
- **Phase 3 (Continuous Deployment):** Migrate `homelab-mcp` to run daemonized on the SRE machine (192.168.99.178) using SSE transport.
- **Phase 5 (Client Interoperability):** Map MCP Prompts to standard Tools to support frontends (like Open WebUI) that lack full native prompt UI support.
