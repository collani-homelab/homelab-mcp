# AI Context & Contributing Guidelines

This document provides context for AI assistants (like Antigravity, Cursor, or Claude) modifying this codebase.

## 1. Provider Pattern
Every new integration MUST implement the `Provider` interface located in `internal/provider/provider.go`:
```go
type Provider interface {
	Name() string
	GetResources() ([]mcp.Resource, error)
	GetResourceContent(uri string) (string, error)
	GetResourceTemplates() ([]mcp.ResourceTemplate, error)
	GetPrompts() ([]mcp.Prompt, error)
	GetPrompt(name string, arguments map[string]string) (*mcp.GetPromptResult, error)
}
```

## 2. API Best Practices
- **JSON Pruning**: Do not return massive JSON payloads (like those from UniFi or Tautulli) directly to the MCP stream. Use the `provider.PruneJSON` utility located in `internal/provider/util.go` to filter out token-heavy keys (e.g., `_id`, `fingerprint`, `uuid`) before returning the content.
- **Fail-Fast**: Always wrap external API calls with `context.WithTimeout` (usually 10 seconds).
- **Graceful Errors**: If a provider cannot connect to its endpoint, it should log the error during instantiation or fetching, but it MUST NOT crash the main server loop. 

## 3. Logging Rules (CRITICAL)
- The MCP server currently uses `stdio` as a transport mechanism.
- **NEVER** write logs to `os.Stdout`. Doing so will corrupt the JSON-RPC stream.
- Always use the `slog` package configured to output to `os.Stderr`.

## 4. Unraid Introspection
- Unraid's GraphQL endpoint has severe depth limits. Do not attempt deep introspection queries.
- When expanding Unraid fields, use the `scratch/` python scripts to iteratively discover fields using `__type` queries.
