# Homelab MCP Roadmap

This document tracks the iterative goals and completed phases of the project. For high-level system design and coding patterns, refer to [ARCHITECTURE.md](ARCHITECTURE.md).

## Phase 1: Read-Only Foundation (Complete)
- [x] **Setup**: Initialize Go module and `mise.toml`.
- [x] **Skeleton**: Implement basic MCP server that responds with a "Hello World" resource.
- [x] **Testing Setup**: Verify the skeleton works using the MCP Inspector and write the first unit test.
- [x] **Unraid Integration**: Implement the Unraid Provider fetching the Docker list via GraphQL.
- [x] **UniFi Integration**: Implement the UniFi Provider fetching the Client list via the Controller API.
- [x] **Refinement**: Add logging and error handling using Go's `slog` package.

## Phase 2: Iteration & Feature Expansion (Complete)

### Goals
- Deepen the context available from both Unraid and UniFi.
- Implement MCP **Prompts** to guide AI discovery.
- Optimize data transmission for token efficiency.

### Roadmap
- [x] **Expansion**: Add System Stats, Array Health, VMs, UPS status, and Notifications to Unraid.
- [x] **Insights**: Add Network Health, Alarms, and PoE status to UniFi.
- [x] **Guidance**: Implement MCP Prompts natively (`homelab_status_report` and `troubleshoot_client`).
- [x] **Optimization**: Implement AI-friendly JSON pruning (`provider.PruneJSON`).

## Phase 3: Media Stack & Hosting (In Progress)

### Goals
- Maintain a Read-Only approach, expanding coverage to the homelab Media Stack.
- Enable running the MCP server as an "always-on" service on the homelab (Unraid), rather than a local binary on the workstation.
- Establish a KISS continuous deployment pipeline.

### Proposed Roadmap (Draft)
- [x] **Transport Layer**: Migrate (or add support for) SSE over HTTP to allow remote consumption of the MCP server.
- [x] **Media Providers**: Use the latest docs for their API or equivalent tools to get the information we need. Start with a small read-only implementation for each provider (implementing the `Provider` interface). After manual verification that the provider is working as expected and providing us with the desired level of detail, expand functionality. Add prompts and resources as needed.
  - [x] Add `Tautulli` and `Plex` for streaming information. (Tautulli and Plex implemented)
  - [x] Add `Starr` apps (`Radarr`, `Sonarr`, `Lidarr`) for queue and missing media contexts. (Radarr, Sonarr, Lidarr implemented)
  - [x] Add `nzbget` for Usenet download management. (NZBGet implemented)
  - [x] Manual verification
  - [x] **Standardization**: Implement a `ToolBuilder` pattern in `internal/mcp` to simplify wrapping SDK methods into MCP Tools.
  - [ ] Add prompts and resources as needed.
  - [x] Create a status prompt to give the current status of the media stack. 
- [x] **Dockerization**: Create a multi-stage `Dockerfile` to build a minimal container image. Use scratch as the base image and build up the image with the dependencies we need. 
  - [x] Create `docker-compose.yml` for local development.
- [x] **CI/CD**: Implemented `ci.yml` and `platform.json` for integration with the new `homelab-platform` "Golden Path".
  - [x] Add docker registry to the SRE machine configuration and update `homelab-mcp` to use the new local registry.
  - [ ] Verify the new CI/CD pipeline.
- [x] **Deployment**: Setup Self-Hosted Runners (transitioning to shared runner topology) and Local Registry on Unraid to serve the latest `main` branch.

## Phase 4: Telemetry & Memory Context (Complete)
- [x] **Deep Metrics**: Implement `monitoring` provider to execute PromQL/LogQL against the new Prometheus/Loki stack.
- [x] **RAG Context**: Implement `context` provider to fetch embedded architecture/golden-path documentation from Qdrant.
- [x] **Alerting**: Implement `alerting` provider to securely dispatch asynchronous push notifications via `ntfy.sh`.

## Phase 5: Client Compatibility & Interoperability (Next)

### Goals
- Ensure the MCP server features are fully usable across all major LLM frontends (e.g., Open WebUI, LibreChat), regardless of their client-side protocol implementation gaps.

### Roadmap
- [ ] **Prompt-to-Tool Mapping**: Implement a dynamic mapping layer in `internal/mcp/server.go` to expose all defined MCP Prompts (both global and provider-specific) as standard MCP Tools.
  - [ ] Intercept tool execution to invoke the underlying prompt handler.
  - [ ] Automatically translate prompt arguments into standard JSON schemas.
  - [ ] Validate implementation using Open WebUI integration.