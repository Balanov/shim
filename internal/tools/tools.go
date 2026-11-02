package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/Balanov/shim/internal/i18n"
	"github.com/Balanov/shim/internal/provider"
)

type Level string

const (
	Read     Level = "read"
	Write    Level = "write"
	Shell    Level = "shell"
	External Level = "external"
)

type RunFunc func(context.Context, json.RawMessage) (string, error)

type Tool struct {
	Spec  provider.ToolSpec
	Level Level
	Run   RunFunc
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry { return &Registry{tools: map[string]Tool{}} }

func (r *Registry) Add(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Spec.Name] = t
}

func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Specs() []provider.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]provider.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type Executor struct {
	Registry *Registry
	Mode     string
	Language string
	Input    *bufio.Reader
	Output   io.Writer
}

func (e *Executor) Execute(ctx context.Context, call provider.ToolCall) (string, error) {
	t, ok := e.Registry.Get(call.Name)
	if !ok {
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
	if t.Run == nil {
		return "", fmt.Errorf("tool %q has no handler", call.Name)
	}
	if e.Mode == "read-only" && t.Level != Read {
		return "", errors.New(i18n.T(e.Language, "denied"))
	}
	if e.Mode == "ask" && t.Level != Read {
		if e.Input == nil {
			return "", fmt.Errorf("%s requires approval in ask mode", call.Name)
		}
		fmt.Fprintf(e.Output, "\n%s %s\n%s", call.Name, summarize(call.Arguments), i18n.T(e.Language, "tool_allow"))
		answer, err := e.Input.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes", "д", "да":
		default:
			return "", errors.New(i18n.T(e.Language, "cancelled"))
		}
	}
	return t.Run(ctx, call.Arguments)
}

func summarize(raw json.RawMessage) string {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	for _, key := range []string{"path", "command", "name", "query"} {
		if v, ok := obj[key].(string); ok {
			if len(v) > 180 {
				v = v[:180] + "…"
			}
			return v
		}
	}
	return ""
}

func Schema(properties string, required ...string) json.RawMessage {
	if required == nil {
		required = []string{}
	}
	data, _ := json.Marshal(map[string]any{
		"type": "object", "properties": json.RawMessage(properties), "required": required,
		"additionalProperties": false,
	})
	return data
}
