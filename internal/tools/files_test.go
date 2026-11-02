package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Balanov/shim/internal/provider"
)

func TestFileToolsAndWorkspaceBoundary(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := NewFileTools(root)
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	files.Register(registry)
	read, _ := registry.Get("read_file")
	out, err := read.Run(context.Background(), json.RawMessage(`{"path":"note.txt"}`))
	if err != nil || !strings.Contains(out, "hello world") {
		t.Fatalf("read failed: %q %v", out, err)
	}
	edit, _ := registry.Get("edit_file")
	if _, err := edit.Run(context.Background(), json.RawMessage(`{"path":"note.txt","old_string":"world","new_string":"shim"}`)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "note.txt"))
	if string(data) != "hello shim" {
		t.Fatalf("edit failed: %s", data)
	}
	if _, err := read.Run(context.Background(), json.RawMessage(`{"path":"../outside"}`)); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestExecutorPermissions(t *testing.T) {
	registry := NewRegistry()
	called := false
	registry.Add(Tool{Level: Write, Spec: provider.ToolSpec{Name: "change"}, Run: func(context.Context, json.RawMessage) (string, error) {
		called = true
		return "done", nil
	}})
	call := provider.ToolCall{Name: "change", Arguments: json.RawMessage(`{}`)}
	denied := &Executor{Registry: registry, Mode: "read-only"}
	if _, err := denied.Execute(context.Background(), call); err == nil || called {
		t.Fatal("read-only execution was not denied")
	}
	var output bytes.Buffer
	allowed := &Executor{Registry: registry, Mode: "ask", Input: bufio.NewReader(strings.NewReader("yes\n")), Output: &output}
	if _, err := allowed.Execute(context.Background(), call); err != nil || !called {
		t.Fatalf("approved execution failed: %v", err)
	}
}
