package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/tools"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("SHIM_MCP_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == 0 {
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "Echo text", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			var params struct {
				Arguments struct{ Text string } `json:"arguments"`
			}
			_ = json.Unmarshal(request.Params, &params)
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": params.Arguments.Text}}}
		default:
			result = map[string]any{}
		}
		_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}
	os.Exit(0)
}

func TestStdioMCPTool(t *testing.T) {
	manager := NewManager(map[string]config.MCPServer{
		"test": {Command: os.Args[0], Args: []string{"-test.run=TestHelperProcess"}, Env: map[string]string{"SHIM_MCP_HELPER": "1"}},
	})
	defer manager.Close()
	registry := tools.NewRegistry()
	if err := manager.Register(context.Background(), "test", registry); err != nil {
		t.Fatal(err)
	}
	specs := registry.Specs()
	if len(specs) != 1 || specs[0].Name != "mcp_test_echo" {
		t.Fatalf("unexpected tools: %+v", specs)
	}
	tool, _ := registry.Get("mcp_test_echo")
	result, err := tool.Run(context.Background(), json.RawMessage(`{"text":"hello"}`))
	if err != nil || result != "hello" {
		t.Fatalf("unexpected result: %q %v", result, err)
	}
}

func TestHTTPSSEMCPTool(t *testing.T) {
	t.Setenv("SHIM_MCP_TOKEN", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("environment expansion failed")
		}
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch request.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "session-1")
			result = map[string]any{"protocolVersion": "2025-03-26"}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "ping", "description": "Ping", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "pong"}}}
		}
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", payload)
	}))
	defer server.Close()
	manager := NewManager(map[string]config.MCPServer{
		"remote": {URL: server.URL, Headers: map[string]string{"Authorization": "Bearer ${SHIM_MCP_TOKEN}"}},
	})
	registry := tools.NewRegistry()
	if err := manager.Register(context.Background(), "remote", registry); err != nil {
		t.Fatal(err)
	}
	result, err := manager.Call(context.Background(), "remote", "ping", json.RawMessage(`{}`))
	if err != nil || result != "pong" {
		t.Fatalf("unexpected HTTP MCP result: %q %v", result, err)
	}
}
