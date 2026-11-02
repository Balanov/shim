package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Balanov/shim/internal/config"
)

func TestAnthropicMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "test-key" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("unexpected headers/path")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["system"] != "be useful" || request["model"] != "claude-test" {
			t.Errorf("unexpected payload: %+v", request)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "working"}, map[string]any{"type": "tool_use", "id": "toolu_1", "name": "list_files", "input": map[string]any{"path": "."}}},
			"usage":   map[string]any{"input_tokens": 8, "output_tokens": 3},
		})
	}))
	defer server.Close()
	t.Setenv("SHIM_TEST_ANTHROPIC", "test-key")
	client, err := New(config.Model{Name: "claude", Provider: "anthropic", API: "anthropic", BaseURL: server.URL, Model: "claude-test", APIKeyEnv: "SHIM_TEST_ANTHROPIC"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Complete(context.Background(), Request{Model: "claude-test", Messages: []Message{{Role: "system", Content: "be useful"}, {Role: "user", Content: "list"}}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "working" || len(response.ToolCalls) != 1 || response.ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
