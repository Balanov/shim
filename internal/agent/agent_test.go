package agent

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/session"
	"github.com/Balanov/shim/internal/tools"
)

type sequenceClient struct{ calls int }

func (c *sequenceClient) Complete(_ context.Context, r provider.Request) (provider.Response, error) {
	c.calls++
	if c.calls == 1 {
		return provider.Response{ToolCalls: []provider.ToolCall{{ID: "one", Name: "lookup", Arguments: json.RawMessage(`{"value":42}`)}}}, nil
	}
	if len(r.Messages) == 0 || r.Messages[len(r.Messages)-1].Content != "value=42" {
		return provider.Response{Text: "missing tool result"}, nil
	}
	return provider.Response{Text: "done"}, nil
}

func TestAgentToolLoop(t *testing.T) {
	client := &sequenceClient{}
	registry := tools.NewRegistry()
	registry.Add(tools.Tool{Level: tools.Read, Spec: provider.ToolSpec{Name: "lookup"}, Run: func(_ context.Context, raw json.RawMessage) (string, error) {
		return "value=42", nil
	}})
	store := session.New(t.TempDir())
	runner := &Agent{Client: client, Model: "fake", Messages: []provider.Message{{Role: "system", Content: "test"}}, Registry: registry, Executor: &tools.Executor{Registry: registry, Mode: "auto"}, Store: store, Output: io.Discard}
	result, err := runner.Run(context.Background(), "calculate")
	if err != nil || result != "done" || client.calls != 2 {
		t.Fatalf("unexpected run: %q %v calls=%d", result, err, client.calls)
	}
	if store.ID == "" {
		t.Fatal("session was not persisted")
	}
	messages, err := session.New(store.Dir).Resume(store.ID)
	if err != nil || len(messages) != 4 || messages[1].Role != "assistant" || messages[2].Role != "tool" || messages[3].Content != "done" {
		t.Fatalf("unexpected transcript: %+v %v", messages, err)
	}
}
