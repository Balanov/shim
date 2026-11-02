package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/tools"
)

type Manager struct {
	configs map[string]config.MCPServer
	clients map[string]Client
}

func NewManager(configs map[string]config.MCPServer) *Manager {
	return &Manager{configs: configs, clients: map[string]Client{}}
}

func (m *Manager) Connect(ctx context.Context, name string) (Client, error) {
	if client, ok := m.clients[name]; ok {
		return client, nil
	}
	cfg, ok := m.configs[name]
	if !ok {
		return nil, fmt.Errorf("unknown MCP server %q", name)
	}
	var client Client
	var err error
	if cfg.URL != "" {
		client = newHTTP(cfg.URL, cfg.Headers)
	} else if cfg.Command != "" {
		client, err = newStdio(ctx, cfg.Command, cfg.Args, cfg.Env)
	} else {
		err = fmt.Errorf("MCP server %q needs command or url", name)
	}
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var initialized struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	err = client.Call(callCtx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "shim", "version": "0.1.0"},
	}, &initialized)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize MCP %s: %w", name, err)
	}
	if err := client.Notify(callCtx, "notifications/initialized", map[string]any{}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize MCP %s: %w", name, err)
	}
	m.clients[name] = client
	return client, nil
}

func (m *Manager) Register(ctx context.Context, name string, registry *tools.Registry) error {
	client, err := m.Connect(ctx, name)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := client.Call(callCtx, "tools/list", map[string]any{}, &result); err != nil {
		return err
	}
	for _, remote := range result.Tools {
		localName := toolName(name, remote.Name)
		if _, exists := registry.Get(localName); exists {
			return fmt.Errorf("MCP tool name collision: %s", localName)
		}
		serverName, remoteName := name, remote.Name
		schema := remote.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		registry.Add(tools.Tool{Level: tools.External, Spec: provider.ToolSpec{
			Name: localName, Description: fmt.Sprintf("MCP/%s: %s", serverName, remote.Description), Parameters: schema,
		}, Run: func(ctx context.Context, args json.RawMessage) (string, error) {
			return m.Call(ctx, serverName, remoteName, args)
		}})
	}
	return nil
}

func (m *Manager) RegisterAll(ctx context.Context, registry *tools.Registry) []error {
	var errs []error
	for name := range m.configs {
		if err := m.Register(ctx, name, registry); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (m *Manager) Call(ctx context.Context, server, tool string, args json.RawMessage) (string, error) {
	client, err := m.Connect(ctx, server)
	if err != nil {
		return "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent any  `json:"structuredContent"`
		IsError           bool `json:"isError"`
	}
	if err := client.Call(callCtx, "tools/call", map[string]any{"name": tool, "arguments": json.RawMessage(args)}, &result); err != nil {
		return "", err
	}
	var parts []string
	for _, content := range result.Content {
		if content.Type == "text" {
			parts = append(parts, content.Text)
		}
	}
	text := strings.Join(parts, "\n")
	if text == "" && result.StructuredContent != nil {
		data, _ := json.MarshalIndent(result.StructuredContent, "", "  ")
		text = string(data)
	}
	if result.IsError {
		return text, fmt.Errorf("MCP tool reported an error")
	}
	return text, nil
}

func (m *Manager) Close() {
	for name, client := range m.clients {
		_ = client.Close()
		delete(m.clients, name)
	}
}

func toolName(server, tool string) string {
	name := "mcp_" + server + "_" + tool
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}
