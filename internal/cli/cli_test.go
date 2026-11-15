package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Balanov/shim/internal/config"
	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/session"
)

func TestModelCommandsAndChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var out, errOut bytes.Buffer
	code := Run([]string{"--config", path, "model", "add", "ollama", "qwen3:4b", "--name", "local", "--use"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatalf("add failed: %d %s", code, errOut.String())
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.DefaultModel != "local" {
		t.Fatalf("config not saved: %+v %v", cfg, err)
	}
	out.Reset()
	errOut.Reset()
	code = Run([]string{"--config", path, "chat"}, strings.NewReader("покажи модели\n/exit\n"), &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "qwen3:4b") {
		t.Fatalf("chat command failed: %d %s %s", code, out.String(), errOut.String())
	}
}

func TestOneShotAgainstOpenAICompatibleServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": "pong"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := config.Load(path)
	cfg.Models = []config.Model{{Name: "fake", Provider: "custom", API: "openai", BaseURL: server.URL, Model: "fake-model"}}
	cfg.DefaultModel = "fake"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Run([]string{"--config", path, "run", "ping"}, strings.NewReader(""), &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "pong") {
		t.Fatalf("run failed: %d %s %s", code, out.String(), errOut.String())
	}
}

func TestResumeSendsSystemPrompt(t *testing.T) {
	var mu sync.Mutex
	var sent []provider.Message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []provider.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		sent = body.Messages
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": "pong"}}},
		})
	}))
	defer server.Close()
	tests := []struct {
		name   string
		stored []provider.Message
	}{
		{name: "session from a previous run"},
		{name: "stored stale system message", stored: []provider.Message{
			{Role: "system", Content: "stale"}, {Role: "user", Content: "ping"}, {Role: "assistant", Content: "pong"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, workspace := t.TempDir(), t.TempDir()
			path := filepath.Join(dir, "config.json")
			cfg, _ := config.Load(path)
			cfg.Models = []config.Model{{Name: "fake", Provider: "custom", API: "openai", BaseURL: server.URL, Model: "fake-model"}}
			cfg.DefaultModel = "fake"
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			args := []string{"--config", path, "--workspace", workspace, "run"}
			store := session.New(filepath.Join(dir, "sessions"))
			for _, m := range tt.stored {
				if err := store.Append(m); err != nil {
					t.Fatal(err)
				}
			}
			var out, errOut bytes.Buffer
			if tt.stored == nil {
				if code := Run(append(args, "ping"), strings.NewReader(""), &out, &errOut); code != 0 {
					t.Fatalf("first run failed: %d %s", code, errOut.String())
				}
			}
			list, err := store.List()
			if err != nil || len(list) != 1 {
				t.Fatalf("unexpected sessions: %+v %v", list, err)
			}
			file := filepath.Join(store.Dir, list[0].ID+".jsonl")
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if code := Run(append(args, "--resume", list[0].ID, "continue"), strings.NewReader(""), &out, &errOut); code != 0 {
				t.Fatalf("resumed run failed: %d %s", code, errOut.String())
			}
			mu.Lock()
			defer mu.Unlock()
			var roles []string
			for _, m := range sent {
				roles = append(roles, m.Role)
			}
			if strings.Join(roles, ",") != "system,user,assistant,user" {
				t.Fatalf("unexpected roles sent on resume: %v", roles)
			}
			if !strings.Contains(sent[0].Content, workspace) || !strings.Contains(sent[0].Content, "untrusted data") {
				t.Fatalf("system prompt not rebuilt on resume: %q", sent[0].Content)
			}
			after, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(after, before) || bytes.Count(after, []byte(`"role":"system"`)) != bytes.Count(before, []byte(`"role":"system"`)) {
				t.Fatalf("session file rewritten:\n%s", after)
			}
		})
	}
}
