package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SSE data fields belong to an event, not to independent JSON-RPC messages.
func TestHTTPSSEEventFraming(t *testing.T) {
	tests := []struct{ name, stream, wantError string }{
		{name: "multiline", stream: "data: {\n" + "data: \"jsonrpc\":\"2.0\",\n" + "data: \"id\":1,\n" + "data: \"result\":{\"ok\":true}}\n\n"},
		{name: "CRLF", stream: "data:{\r\ndata: \"jsonrpc\":\"2.0\",\"id\":1,\r\ndata: \"result\":{\"ok\":true}}\r\n\r\n"},
		{name: "notification then response", stream: "data: {\"jsonrpc\":\"2.0\",\n" + "data: \"method\":\"notifications/message\"}\n\n: keepalive\nevent: message\nid: cursor-1\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\n" + "data: \"result\":{\"ok\":true}}\n\n"},
		{name: "unrelated error does not leak", stream: "data: {\"jsonrpc\":\"2.0\",\"id\":2,\"error\":{\"code\":-1,\"message\":\"other call\"}}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n"},
		{name: "multiline error", stream: "data: {\"jsonrpc\":\"2.0\",\"id\":1,\n" + "data: \"error\":{\"code\":-32603,\"message\":\"failed\"}}\n\n", wantError: "MCP error -32603: failed"},
		{name: "single line", stream: "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n"},
		{name: "separate events are not joined", stream: "data: {\"jsonrpc\":\"2.0\",\"id\":1,\n\ndata: \"result\":{\"ok\":true}}\n\n", wantError: "MCP SSE stream ended without a response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var request rpcRequest
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.ID != 1 || request.Method != "tools/list" {
					t.Errorf("unexpected request: %+v", request)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tt.stream)
			}))
			defer server.Close()
			var out struct {
				OK bool `json:"ok"`
			}
			err := newHTTP(server.URL, nil).Call(context.Background(), "tools/list", nil, &out)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want %q", err, tt.wantError)
				}
			} else if err != nil || !out.OK {
				t.Fatalf("result = %+v, error = %v", out, err)
			}
		})
	}
}
