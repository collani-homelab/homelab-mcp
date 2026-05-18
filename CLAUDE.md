# Homelab MCP Coding Guidelines (CLAUDE.md)

This repository defines the Model Context Protocol (MCP) server for local homelab integrations. It implements providers for Unraid, UniFi, Plex/Tautulli, Starr applications (Radarr, Sonarr, Lidarr), and NZBGet.

## 🔗 Global Guidelines Reference
All development in this repository must align with the top-level principles defined in the master rulebook:
👉 **[GLOBAL_CLAUDE.md](file:///home/wcollani/repos/homelab/src/meta/GLOBAL_CLAUDE.md)**

---

## 🛠️ Build and Development Commands
We use `mise` for environment and toolchain management.

### Build and Run Tasks
- **Build the server binary:**
  ```bash
  mise run build
  # Or: go build -o homelab-mcp ./cmd/server
  ```
- **Run the server locally (stdio transport):**
  ```bash
  mise run run
  ```
- **Run the server via MCP Inspector (visual E2E testing):**
  ```bash
  mise run inspector
  ```

### Linting and Testing Tasks
- **Run static analysis & linters:**
  ```bash
  mise run lint
  # Or: golangci-lint run
  ```
- **Run all tests (unit & integration) with race detection:**
  ```bash
  mise run test
  # Or: go test -v -race -cover ./...
  ```

### Docker Tasks (SSE Transport)
- **Build Docker image:**
  ```bash
  mise run docker-build
  ```
- **Run Docker container locally with SSE transport:**
  ```bash
  mise run docker-run
  ```

---

## 📐 Local Coding Standards & Rules

### 1. The Provider Pattern
Every new homelab integration must be implemented as a separate provider that implements the `Provider` interface defined in [provider.go](file:///home/wcollani/repos/homelab-mcp/internal/provider/provider.go):
- Expose state and read-only data as **MCP Resources**.
- Expose actions and state-mutating requests as **MCP Tools**.
- Register new providers inside the server setup in `internal/mcp/`.

### 2. API Best Practices
- **JSON Pruning (Mandatory):** Do not return large raw payloads to the MCP client. Always use the `provider.PruneJSON` utility in [util.go](file:///home/wcollani/repos/homelab-mcp/internal/provider/util.go) to strip token-wasting metadata (e.g., `_id`, `uuid`, `fingerprint`, internal timestamps) before returning response data.
- **Fail-Fast with Timeouts:** Always wrap external API calls in a `context.WithTimeout` block (default to 10 seconds) to prevent slow or offline homelab services from hanging the client.
- **Graceful Error Handling:** If a provider fails to initialize or experiences connection issues, log the error but **do not crash** the main server loop. Other providers must continue serving requests.

### 3. Stdio Transport Logging Rule (CRITICAL)
- **NEVER write standard logs to `os.Stdout`** when the server is operating under the `stdio` transport. Doing so will corrupt the JSON-RPC message stream and crash the AI client connection.
- Always use the `slog` package configured to log exclusively to `os.Stderr` or a dedicated log file.

### 4. Unraid Introspection Constraint
- Unraid's GraphQL endpoint has strict depth limits. Avoid initiating deep GraphQL introspection queries.
- Use helper scripts in the [scratch/](file:///home/wcollani/repos/homelab-mcp/scratch/) folder to test and iteratively discover valid schema fields.
