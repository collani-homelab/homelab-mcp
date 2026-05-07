package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"homelab-mcp/internal/provider"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// rpcRequest defines the structure for a JSON-RPC request.
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// rpcResponse defines the structure for a JSON-RPC response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// listResourcesResult is used to unmarshal the result of a resources/list call.
type listResourcesResult struct {
	Resources []mcp.Resource `json:"resources"`
}

// writeRequest is a helper to marshal and send a JSON-RPC request to the server's stdin pipe.
func writeRequest(t *testing.T, w *os.File, req interface{}) {
	t.Helper()
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	if _, err := w.Write(append(reqBytes, '\n')); err != nil {
		t.Fatalf("failed to write to server stdin: %v", err)
	}
}

// readResponse is a helper to read a single newline-delimited JSON-RPC response from the server's stdout pipe.
func readResponse(t *testing.T, r *os.File) string {
	t.Helper()
	reader := bufio.NewReader(r)
	lineChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errChan <- err
		} else {
			lineChan <- line
		}
	}()

	select {
	case line := <-lineChan:
		return line
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server response")
	}
	return ""
}

func TestServer_StdioIntegration(t *testing.T) {
	// 1. Hijack os.Stdin and os.Stdout to control I/O in-memory.
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	// Create pipes. The server will read from rIn and write to wOut.
	rIn, wIn, _ := os.Pipe()
	rOut, wOut, _ := os.Pipe()
	os.Stdin = rIn
	os.Stdout = wOut

	// 2. Setup and run the server in a background goroutine.
	srv := NewServer("homelab-mcp-test", "0.1.0")
	srv.AddProvider(&provider.HelloProvider{})

	serverErrChan := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		serverErrChan <- srv.Run(ctx)
	}()

	// 3. Perform the MCP initialization handshake.
	initReq := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{"protocolVersion": "2024-11-05"},
	}
	writeRequest(t, wIn, initReq)

	// Read and verify the initialize response.
	initRespLine := readResponse(t, rOut)
	if !strings.Contains(initRespLine, `"id":1`) {
		t.Fatalf("Expected init response for ID 1, got: %s", initRespLine)
	}

	// Send the final part of the handshake.
	writeRequest(t, wIn, rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"})

	// 4. Now that the session is initialized, send the actual test request.
	listReq := rpcRequest{JSONRPC: "2.0", ID: 2, Method: "resources/list"}
	writeRequest(t, wIn, listReq)

	// 5. Read and assert the final response.
	listRespLine := readResponse(t, rOut)
	if !strings.Contains(listRespLine, `"uri":"hello://world"`) {
		t.Errorf("Expected response to contain hello://world resource, but got: %s", listRespLine)
	}
	if !strings.Contains(listRespLine, `"id":2`) {
		t.Errorf("Expected response to have ID 2, but got: %s", listRespLine)
	}
}