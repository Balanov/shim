package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Balanov/shim/internal/config"
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
