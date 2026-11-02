package provider

import (
	"context"
	"encoding/json"
	"fmt"
)

type openAIClient struct{ *baseClient }

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func (c *openAIClient) Complete(ctx context.Context, r Request) (Response, error) {
	messages := make([]openAIMessage, 0, len(r.Messages))
	for _, m := range r.Messages {
		om := openAIMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			otc := openAIToolCall{ID: tc.ID, Type: "function"}
			otc.Function.Name = tc.Name
			otc.Function.Arguments = string(tc.Arguments)
			om.ToolCalls = append(om.ToolCalls, otc)
		}
		messages = append(messages, om)
	}
	tools := make([]openAITool, 0, len(r.Tools))
	for _, t := range r.Tools {
		ot := openAITool{Type: "function"}
		ot.Function.Name, ot.Function.Description = t.Name, t.Description
		ot.Function.Parameters = t.Parameters
		if len(ot.Function.Parameters) == 0 {
			ot.Function.Parameters = rawObject()
		}
		tools = append(tools, ot)
	}
	payload := struct {
		Model     string          `json:"model"`
		Messages  []openAIMessage `json:"messages"`
		Tools     []openAITool    `json:"tools,omitempty"`
		MaxTokens int             `json:"max_tokens,omitempty"`
	}{r.Model, messages, tools, r.MaxTokens}
	headers := map[string]string{}
	if c.key != "" {
		headers["Authorization"] = "Bearer " + c.key
	}
	var raw struct {
		Choices []struct {
			Message struct {
				Content   json.RawMessage `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := c.post(ctx, endpoint(c.model.BaseURL, "/chat/completions"), payload, headers, &raw); err != nil {
		return Response{}, err
	}
	if len(raw.Choices) == 0 {
		return Response{}, fmt.Errorf("model returned no choices")
	}
	msg := raw.Choices[0].Message
	text, err := parseContent(msg.Content)
	if err != nil {
		return Response{}, err
	}
	out := Response{Text: text, Usage: Usage{InputTokens: raw.Usage.PromptTokens, OutputTokens: raw.Usage.CompletionTokens}}
	for i, tc := range msg.ToolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i+1)
		}
		args := json.RawMessage(tc.Function.Arguments)
		if !json.Valid(args) {
			args = json.RawMessage(`{}`)
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: id, Name: tc.Function.Name, Arguments: args})
	}
	return out, nil
}

func parseContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("unsupported message content: %w", err)
	}
	for _, p := range parts {
		if p.Type == "text" || p.Type == "output_text" {
			text += p.Text
		}
	}
	return text, nil
}
