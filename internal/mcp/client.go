package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Client interface {
	Call(context.Context, string, any, any) error
	Notify(context.Context, string, any) error
	Close() error
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type stdioClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	stderr  *limitedBuffer
	mu      sync.Mutex
	nextID  int
}

func newStdio(ctx context.Context, command string, args []string, env map[string]string) (*stdioClient, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+os.ExpandEnv(v))
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &limitedBuffer{max: 16 << 10}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start MCP server: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	c := &stdioClient{cmd: cmd, stdin: stdin, scanner: scanner, stderr: stderr}
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	return c, nil
}

func (c *stdioClient) Call(ctx context.Context, method string, params any, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	if err := c.write(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return err
	}
	stopCancel := context.AfterFunc(ctx, func() {
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
	})
	defer stopCancel()
	for c.scanner.Scan() {
		var response rpcResponse
		if err := json.Unmarshal(c.scanner.Bytes(), &response); err != nil {
			continue
		}
		if response.ID != id {
			continue
		}
		return decodeResponse(response, out)
	}
	if err := c.scanner.Err(); err != nil {
		return fmt.Errorf("read MCP response: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("MCP server closed stdio: %s", strings.TrimSpace(c.stderr.String()))
}

func (c *stdioClient) Notify(_ context.Context, method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
}

func (c *stdioClient) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *stdioClient) Close() error {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return c.cmd.Wait()
}

type httpClient struct {
	url       string
	headers   map[string]string
	http      *http.Client
	mu        sync.RWMutex
	nextID    int
	sessionID string
}

func newHTTP(url string, headers map[string]string) *httpClient {
	return &httpClient{url: url, headers: headers, http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *httpClient) Call(ctx context.Context, method string, params any, out any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	response, err := c.roundTrip(ctx, rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return err
	}
	return decodeResponse(response, out)
}

func (c *httpClient) Notify(ctx context.Context, method string, params any) error {
	_, err := c.roundTrip(ctx, rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	return err
}

func (c *httpClient) roundTrip(ctx context.Context, request rpcRequest) (rpcResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return rpcResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(data))
	if err != nil {
		return rpcResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.headers {
		req.Header.Set(k, os.ExpandEnv(v))
	}
	c.mu.RLock()
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.RUnlock()
	resp, err := c.http.Do(req)
	if err != nil {
		return rpcResponse{}, err
	}
	defer resp.Body.Close()
	if session := resp.Header.Get("Mcp-Session-Id"); session != "" {
		c.mu.Lock()
		c.sessionID = session
		c.mu.Unlock()
	}
	if resp.StatusCode == http.StatusAccepted && request.ID == 0 {
		return rpcResponse{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return rpcResponse{}, fmt.Errorf("MCP HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var response rpcResponse
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, 16<<20))
		scanner.Buffer(make([]byte, 64*1024), 16<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if err := json.Unmarshal([]byte(payload), &response); err == nil && (request.ID == 0 || response.ID == request.ID) {
				return response, nil
			}
		}
		return rpcResponse{}, fmt.Errorf("MCP SSE stream ended without a response")
	}
	if request.ID == 0 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return rpcResponse{}, nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&response); err != nil {
		return rpcResponse{}, err
	}
	return response, nil
}

func (c *httpClient) Close() error { return nil }

func decodeResponse(response rpcResponse, out any) error {
	if response.Error != nil {
		return fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
	}
	if out == nil {
		return nil
	}
	if len(response.Result) == 0 {
		return fmt.Errorf("MCP response has no result")
	}
	return json.Unmarshal(response.Result, out)
}

type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - b.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return b.Buffer.Write(p)
}
