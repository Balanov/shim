package agent

import (
	"context"
	"encoding/json"
	"errors"
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

type completeFunc func(context.Context, provider.Request) (provider.Response, error)

func (f completeFunc) Complete(ctx context.Context, r provider.Request) (provider.Response, error) {
	return f(ctx, r)
}

func TestAgentToolResult(t *testing.T) {
	for _, tt := range []struct {
		name   string
		output string
		err    error
		want   string
	}{
		{"failure with output", "FAIL: TestExample\nexpected 42, got 0\n", errors.New("command failed: exit status 1"), "FAIL: TestExample\nexpected 42, got 0\n\nError: command failed: exit status 1"},
		{"failure without output", "", errors.New("permission denied"), "Error: permission denied"},
		{"success with output", "all tests passed", nil, "all tests passed"},
		{"success without output", "", nil, "(no output)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := completeFunc(func(_ context.Context, r provider.Request) (provider.Response, error) {
				calls++
				if calls == 1 {
					return provider.Response{ToolCalls: []provider.ToolCall{{ID: "one", Name: "check", Arguments: json.RawMessage(`{}`)}}}, nil
				}
				message := r.Messages[len(r.Messages)-1]
				if message.Role != "tool" || message.ToolCallID != "one" || message.Content != tt.want {
					t.Errorf("unexpected tool message: %+v; want content %q", message, tt.want)
				}
				return provider.Response{Text: "done"}, nil
			})
			registry := tools.NewRegistry()
			registry.Add(tools.Tool{Level: tools.Read, Spec: provider.ToolSpec{Name: "check"}, Run: func(_ context.Context, _ json.RawMessage) (string, error) {
				return tt.output, tt.err
			}})
			store := session.New(t.TempDir())
			runner := &Agent{Client: client, Model: "fake", Registry: registry, Executor: &tools.Executor{Registry: registry, Mode: "auto"}, Store: store}
			result, err := runner.Run(context.Background(), "run checks")
			if err != nil || result != "done" || calls != 2 {
				t.Fatalf("unexpected run: %q %v calls=%d", result, err, calls)
			}
			messages, err := session.New(store.Dir).Resume(store.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != 4 || messages[2].Role != "tool" || messages[2].ToolCallID != "one" || messages[2].Content != tt.want {
				t.Fatalf("unexpected transcript: %+v; want tool content %q", messages, tt.want)
			}
		})
	}
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
