package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Balanov/shim/internal/config"
)

func TestOpenAIToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["model"] != "test-model" || len(request["tools"].([]any)) != 1 {
			t.Errorf("unexpected payload: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{
				"content":    nil,
				"tool_calls": []any{map[string]any{"id": "call_1", "function": map[string]any{"name": "read_file", "arguments": `{"path":"README.md"}`}}},
			}}},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 4},
		})
	}))
	defer server.Close()
	t.Setenv("SHIM_TEST_KEY", "test-key")
	client, err := New(config.Model{Name: "test", Provider: "custom", API: "openai", BaseURL: server.URL + "/v1", Model: "test-model", APIKeyEnv: "SHIM_TEST_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Complete(context.Background(), Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "read"}}, Tools: []ToolSpec{{Name: "read_file", Parameters: rawObject()}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "read_file" || response.Usage.InputTokens != 10 {
		t.Fatalf("unexpected response: %+v", response)
	}
}
