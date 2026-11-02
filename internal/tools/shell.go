package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

type ShellTool struct {
	Root    string
	Timeout time.Duration
}

func (s ShellTool) Register(r *Registry) {
	r.Add(Tool{Level: Shell, Spec: providerSpec("shell", "Run a shell command in the workspace. Use for tests, builds, git inspection, and system commands.", `{"command":{"type":"string"}}`, "command"), Run: s.run})
}

type cappedBuffer struct {
	bytes.Buffer
	max int
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	remaining := w.max - w.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return w.Buffer.Write(p)
}

func (s ShellTool) run(ctx context.Context, raw json.RawMessage) (string, error) {
	var in struct{ Command string }
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	if in.Command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/D", "/S", "/C", in.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", in.Command)
	}
	cmd.Dir = s.Root
	out := &cappedBuffer{max: 1 << 20}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if ctx.Err() != nil {
		return out.String(), fmt.Errorf("command timed out after %s", timeout)
	}
	if err != nil {
		return out.String(), fmt.Errorf("command failed: %w", err)
	}
	return out.String(), nil
}
