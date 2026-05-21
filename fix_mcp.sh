sed -i 's/mcp.ToolInputSchema/map[string]interface{}/g' internal/provider/alerting/alerting.go
sed -i 's/mcp.ToolInputSchema/map[string]interface{}/g' internal/provider/monitoring/monitoring.go

sed -i 's/return mcp.NewToolResultError(fmt.Sprintf(\(.*\))), nil/return \&mcp.CallToolResult{IsError: true, Content: []mcp.Content{\&mcp.TextContent{Text: fmt.Sprintf(\1)}}}, nil/g' internal/provider/alerting/alerting.go
sed -i 's/return mcp.NewToolResultError(\(.*\)), nil/return \&mcp.CallToolResult{IsError: true, Content: []mcp.Content{\&mcp.TextContent{Text: \1}}}, nil/g' internal/provider/alerting/alerting.go
sed -i 's/return mcp.NewToolResultText(\(.*\)), nil/return \&mcp.CallToolResult{Content: []mcp.Content{\&mcp.TextContent{Text: \1}}}, nil/g' internal/provider/alerting/alerting.go

sed -i 's/return mcp.NewToolResultError(fmt.Sprintf(\(.*\))), nil/return \&mcp.CallToolResult{IsError: true, Content: []mcp.Content{\&mcp.TextContent{Text: fmt.Sprintf(\1)}}}, nil/g' internal/provider/monitoring/monitoring.go
sed -i 's/return mcp.NewToolResultError(\(.*\)), nil/return \&mcp.CallToolResult{IsError: true, Content: []mcp.Content{\&mcp.TextContent{Text: \1}}}, nil/g' internal/provider/monitoring/monitoring.go
sed -i 's/return mcp.NewToolResultText(\(.*\)), nil/return \&mcp.CallToolResult{Content: []mcp.Content{\&mcp.TextContent{Text: \1}}}, nil/g' internal/provider/monitoring/monitoring.go

