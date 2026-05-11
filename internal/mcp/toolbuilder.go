package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewTool constructs an mcp.Tool with the given name, description, and a
// minimal JSON Schema object for the input. The inputSchema map should follow
// JSON Schema draft conventions — e.g.:
//
//	mcp.NewTool("search_series", "Search Sonarr for a TV series", map[string]interface{}{
//	    "query": map[string]interface{}{
//	        "type":        "string",
//	        "description": "The title to search for",
//	    },
//	})
//
// To define a tool with no parameters, pass nil or an empty map for inputSchema.
func NewTool(name, description string, inputSchema map[string]interface{}) *mcp.Tool {
	properties := inputSchema
	if properties == nil {
		properties = map[string]interface{}{}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	// Collect required fields: any property that has "required": true at the
	// top level (non-standard shorthand we accept here for ergonomics).
	var required []string
	for k, v := range properties {
		if propMap, ok := v.(map[string]interface{}); ok {
			if req, ok := propMap["required"].(bool); ok && req {
				required = append(required, k)
				// Remove the non-standard key from the property definition so
				// the schema we emit is valid JSON Schema.
				delete(propMap, "required")
			}
		}
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return &mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
	}
}

// TextResult returns a successful *mcp.CallToolResult containing a single
// TextContent block. This is the most common return type for tool calls that
// produce human-readable or JSON string output.
func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// ErrorResult returns a *mcp.CallToolResult that signals a tool-level error
// (IsError = true). Per the MCP spec, tool errors should be returned inside
// the CallToolResult — NOT as protocol-level errors — so the LLM can see the
// failure and attempt to self-correct.
func ErrorResult(err error) *mcp.CallToolResult {
	result := &mcp.CallToolResult{}
	result.SetError(err)
	return result
}
