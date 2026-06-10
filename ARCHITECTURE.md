# Homelab MCP Architecture & Design

## 1. Development Environment

- **Toolchain:** `mise` (`.mise.toml`) manages Go 1.22+, Node (for MCP Inspector), and task aliases.
- **Build tasks:** `mise run build`, `mise run test`, `mise run lint`, `mise run docker-build`.

## 2. Project Structure

```
homelab-mcp/
├── cmd/server/          # main() — wires providers into the MCP server
├── internal/
│   ├── mcp/
│   │   ├── server.go        # Server struct, provider registration, SSE/stdio transport
│   │   └── toolbuilder.go   # NewTool / TextResult / ErrorResult helpers
│   └── provider/
│       ├── provider.go      # Provider interface
│       ├── util.go          # PruneJSON utility
│       ├── hello/           # Smoke-test provider
│       ├── unraid/          # Unraid GraphQL client (multi-server)
│       ├── unifi/           # UniFi REST client
│       ├── tautulli/        # Tautulli REST client
│       ├── plex/            # Plex REST client
│       ├── radarr/          # Radarr REST client
│       ├── sonarr/          # Sonarr REST client
│       ├── lidarr/          # Lidarr REST client
│       ├── nzbget/          # NZBGet JSON-RPC client
│       ├── monitoring/      # Prometheus + Loki query tools
│       ├── context/         # RAG context (Qdrant via homelab-context service)
│       └── alerting/        # Push notifications via ntfy.sh
├── scratch/             # One-off scripts for GraphQL schema discovery
├── Dockerfile           # Multi-stage scratch image
├── docker-compose.yml   # Local and server deployment
└── platform.json        # homelab-platform Golden Path metadata
```

## 3. The Provider Pattern

Every integration implements a single interface:

```go
type Provider interface {
    Name() string
    GetResources() ([]mcp.Resource, error)
    GetResourceContent(uri string) (string, error)
    GetResourceTemplates() ([]mcp.ResourceTemplate, error)
    GetPrompts() ([]mcp.Prompt, error)
    GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error)
    GetTools() ([]mcp.Tool, error)
    CallTool(name string, arguments map[string]interface{}) (*mcp.CallToolResult, error)
}
```

- **Resources** expose read-only state polled on demand.
- **Tools** expose actions or richer queries that benefit from LLM-directed invocation.
- Register new providers in `cmd/server/main.go` using `s.AddProvider(...)`.

## 4. Tool Builder

`internal/mcp/toolbuilder.go` provides three helpers to reduce boilerplate:

- `NewTool(name, description, inputSchema)` — constructs an `mcp.Tool` and hoists `"required": true` properties into the JSON Schema `required` array.
- `TextResult(text)` — wraps a string in a successful `*mcp.CallToolResult`.
- `ErrorResult(err)` — wraps an error in a tool-level `*mcp.CallToolResult` (not a protocol error, so the LLM can self-correct).

## 5. Transport

The server supports two transports, selected via the `MCP_TRANSPORT` environment variable:

| Value | Behaviour |
|---|---|
| `sse` (default in Docker) | HTTP server on `$PORT` (default 8080). SSE GET `/mcp` negotiates a session, POST delivers messages. CORS is open. |
| unset / `stdio` | JSON-RPC over stdin/stdout. **Never write to stdout** — it corrupts the stream. All logs go to stderr. |

## 6. Key Constraints

- **Timeouts:** All external API calls use `context.WithTimeout` (10s default). Slow or offline services fail fast.
- **Graceful degradation:** Provider init failures are logged but do not crash the server. Other providers remain available.
- **JSON pruning:** Use `provider.PruneJSON` to strip UUIDs, fingerprints, and internal metadata from large payloads before returning to the LLM.
- **Unraid GraphQL depth:** Unraid's GraphQL endpoint has strict depth limits. Use the scripts in `scratch/` to iteratively discover valid fields rather than running deep introspection queries.

## 7. Deployment

The production instance runs as a Docker container on the SRE machine:

- **Image:** `192.168.99.178:5000/homelab-mcp:v2` (local registry)
- **Port:** 8083 on host → 8080 in container
- **Compose file:** `docker-compose.yml` (server copy at `/home/wcollani/repos/homelab-mcp/`)
- **CI/CD:** GitHub Actions (`deploy.yml`) → `mise run docker-build && docker-push` → `homelab-deploy -deploy homelab-mcp`

## 9. PR Workflow

All changes reach `main` via PR. Three workflow files implement the full loop:

| File | Trigger | Runner | Purpose |
|------|---------|--------|---------|
| `ci.yml` | `pull_request`, `push` to `main` | `ubuntu-latest` | Tests + lint — required status check |
| `pr-review.yml` | `pull_request` to `main` | `[self-hosted, sre]` | Claude code-review + security-review (advisory) + ntfy notify |
| `deploy.yml` | `push` to `main` | `[self-hosted, sre]` | Docker build → local registry → `homelab-deploy -deploy homelab-mcp` |

The `review` job in `pr-review.yml` is gated to `wcollani`-authored PRs only. Both review steps use `continue-on-error: true`; the notify step fires via `if: always()`.

> Full spec, security risk notes, runner label table, and LiteLLM swap instructions:
> **[homelab-platform/ARCHITECTURE.md §6 — PR Workflow Golden Path](../homelab-platform/ARCHITECTURE.md#6-pr-workflow-golden-path)**

## 8. Testing

- **Unit tests:** `go test -v -race -cover ./...` — mock HTTP responses, verify resource/tool parsing.
- **Integration tests:** `internal/mcp/server_test.go` — programmatic JSON-RPC invocation against a live server instance.
- **Manual/E2E:** `mise run inspector` launches the MCP Inspector against the compiled binary.
