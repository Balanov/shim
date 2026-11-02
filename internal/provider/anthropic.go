package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type anthropicClient struct{ *baseClient }

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

func (c *anthropicClient) Complete(ctx context.Context, r Request) (Response, error) {
	var system []string
	messages := make([]anthropicMessage, 0, len(r.Messages))
	for _, m := range r.Messages {
		if m.Role == "system" {
			system = append(system, m.Content)
			continue
		}
		role := m.Role
		var blocks []anthropicBlock
		switch m.Role {
		case "tool":
			role = "user"
			blocks = append(blocks, anthropicBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content})
		case "assistant":
			if m.Content != "" {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, anthropicBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: tc.Arguments})
			}
		default:
			role = "user"
			blocks = append(blocks, anthropicBlock{Type: "text", Text: m.Content})
		}
		if len(messages) > 0 && messages[len(messages)-1].Role == role {
			messages[len(messages)-1].Content = append(messages[len(messages)-1].Content, blocks...)
		} else {
			messages = append(messages, anthropicMessage{Role: role, Content: blocks})
		}
	}
	tools := make([]struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}, 0, len(r.Tools))
	for _, t := range r.Tools {
		schema := t.Parameters
		if len(schema) == 0 {
			schema = rawObject()
		}
		tools = append(tools, struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"input_schema"`
		}{t.Name, t.Description, schema})
	}
	maxTokens := r.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	payload := struct {
		Model     string             `json:"model"`
		System    string             `json:"system,omitempty"`
		Messages  []anthropicMessage `json:"messages"`
		Tools     any                `json:"tools,omitempty"`
		MaxTokens int                `json:"max_tokens"`
	}{r.Model, strings.Join(system, "\n\n"), messages, tools, maxTokens}
	headers := map[string]string{"anthropic-version": "2023-06-01"}
	if c.key != "" {
		headers["x-api-key"] = c.key
	}
	var raw struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := c.post(ctx, endpoint(c.model.BaseURL, "/v1/messages"), payload, headers, &raw); err != nil {
		return Response{}, err
	}
	out := Response{Usage: Usage{InputTokens: raw.Usage.InputTokens, OutputTokens: raw.Usage.OutputTokens}}
	for i, b := range raw.Content {
		switch b.Type {
		case "text":
			out.Text += b.Text
		case "tool_use":
			id := b.ID
			if id == "" {
				id = fmt.Sprintf("toolu_%d", i+1)
			}
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: id, Name: b.Name, Arguments: b.Input})
		}
	}
	return out, nil
}
