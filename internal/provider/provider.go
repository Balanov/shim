package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Balanov/shim/internal/config"
)

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type Request struct {
	Model     string
	Messages  []Message
	Tools     []ToolSpec
	MaxTokens int
}

type Response struct {
	Text      string
	ToolCalls []ToolCall
	Usage     Usage
}

type Client interface {
	Complete(context.Context, Request) (Response, error)
}

func New(m config.Model) (Client, error) {
	key := ""
	if m.APIKeyEnv != "" {
		key = os.Getenv(m.APIKeyEnv)
		if key == "" && m.Provider != "ollama" && m.Provider != "lmstudio" && m.Provider != "llamacpp" {
			return nil, fmt.Errorf("%s is not set (needed by model %q)", m.APIKeyEnv, m.Name)
		}
	}
	base := &baseClient{model: m, key: key, http: &http.Client{Timeout: 180 * time.Second}}
	switch m.API {
	case "openai":
		return &openAIClient{baseClient: base}, nil
	case "anthropic":
		return &anthropicClient{baseClient: base}, nil
	default:
		return nil, fmt.Errorf("unsupported model API %q", m.API)
	}
}

type baseClient struct {
	model config.Model
	key   string
	http  *http.Client
}

func (c *baseClient) post(ctx context.Context, url string, payload any, headers map[string]string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "shim/0.1.0")
	for k, v := range c.model.Headers {
		req.Header.Set(k, os.ExpandEnv(v))
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return fmt.Errorf("model API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode model response: %w", err)
	}
	return nil
}

func endpoint(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func rawObject() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
