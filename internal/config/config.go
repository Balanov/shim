package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const CurrentVersion = 1

type Model struct {
	Name      string            `json:"name"`
	Provider  string            `json:"provider"`
	API       string            `json:"api"`
	BaseURL   string            `json:"base_url"`
	Model     string            `json:"model"`
	APIKeyEnv string            `json:"api_key_env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type MCPServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type Config struct {
	Version      int                  `json:"version"`
	Language     string               `json:"language,omitempty"`
	DefaultModel string               `json:"default_model,omitempty"`
	RouterModel  string               `json:"router_model,omitempty"`
	Permissions  string               `json:"permissions"`
	Models       []Model              `json:"models,omitempty"`
	MCPServers   map[string]MCPServer `json:"mcp_servers,omitempty"`
	path         string
}

func Default() Config {
	return Config{Version: CurrentVersion, Language: "en", Permissions: "ask"}
}

func DefaultPath() (string, error) {
	if p := os.Getenv("SHIM_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "shim", "config.json"), nil
}

func Load(path string) (Config, error) {
	var err error
	if path == "" {
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}
	cfg := Default()
	cfg.path = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.path = path
	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}
	if cfg.Version != CurrentVersion {
		return Config{}, fmt.Errorf("unsupported config version %d", cfg.Version)
	}
	if cfg.Permissions == "" {
		cfg.Permissions = "ask"
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	return cfg, cfg.Validate()
}

func (c Config) Path() string { return c.path }

func (c Config) Dir() string { return filepath.Dir(c.path) }

func (c Config) Validate() error {
	switch c.Permissions {
	case "ask", "read-only", "auto":
	default:
		return fmt.Errorf("permissions must be ask, read-only, or auto")
	}
	seen := map[string]bool{}
	for _, m := range c.Models {
		if m.Name == "" || m.Model == "" || m.BaseURL == "" {
			return errors.New("each model needs name, model, and base_url")
		}
		if m.API != "openai" && m.API != "anthropic" {
			return fmt.Errorf("model %s: api must be openai or anthropic", m.Name)
		}
		if seen[m.Name] {
			return fmt.Errorf("duplicate model name %q", m.Name)
		}
		seen[m.Name] = true
	}
	if c.DefaultModel != "" && !seen[c.DefaultModel] {
		return fmt.Errorf("default model %q is not configured", c.DefaultModel)
	}
	if c.RouterModel != "" && !seen[c.RouterModel] {
		return fmt.Errorf("router model %q is not configured", c.RouterModel)
	}
	return nil
}

func (c *Config) Save() error {
	if c.path == "" {
		p, err := DefaultPath()
		if err != nil {
			return err
		}
		c.path = p
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

func (c Config) ModelByName(name string) (Model, bool) {
	for _, m := range c.Models {
		if m.Name == name {
			return m, true
		}
	}
	return Model{}, false
}

func (c *Config) UpsertModel(m Model) {
	for i := range c.Models {
		if c.Models[i].Name == m.Name {
			c.Models[i] = m
			return
		}
	}
	c.Models = append(c.Models, m)
}

func (c *Config) RemoveModel(name string) bool {
	for i := range c.Models {
		if c.Models[i].Name == name {
			c.Models = append(c.Models[:i], c.Models[i+1:]...)
			if c.DefaultModel == name {
				c.DefaultModel = ""
			}
			if c.RouterModel == name {
				c.RouterModel = ""
			}
			return true
		}
	}
	return false
}

func Preset(name string) (Model, error) {
	m := Model{Name: name, Provider: name, API: "openai"}
	switch strings.ToLower(name) {
	case "openai":
		m.BaseURL, m.APIKeyEnv = "https://api.openai.com/v1", "OPENAI_API_KEY"
	case "openrouter":
		m.BaseURL, m.APIKeyEnv = "https://openrouter.ai/api/v1", "OPENROUTER_API_KEY"
	case "anthropic":
		m.BaseURL, m.API, m.APIKeyEnv = "https://api.anthropic.com", "anthropic", "ANTHROPIC_API_KEY"
	case "gemini":
		m.BaseURL, m.APIKeyEnv = "https://generativelanguage.googleapis.com/v1beta/openai", "GEMINI_API_KEY"
	case "github":
		m.BaseURL, m.APIKeyEnv = "https://models.github.ai/inference", "GITHUB_TOKEN"
	case "huggingface", "hf":
		m.Name, m.Provider = "huggingface", "huggingface"
		m.BaseURL, m.APIKeyEnv = "https://router.huggingface.co/v1", "HF_TOKEN"
	case "ollama":
		m.BaseURL = "http://127.0.0.1:11434/v1"
	case "lmstudio", "lm-studio":
		m.Name, m.Provider = "lmstudio", "lmstudio"
		m.BaseURL = "http://127.0.0.1:1234/v1"
	case "llamacpp", "llama.cpp":
		m.Name, m.Provider = "llamacpp", "llamacpp"
		m.BaseURL = "http://127.0.0.1:8080/v1"
	default:
		return Model{}, fmt.Errorf("unknown preset %q", name)
	}
	return m, nil
}

func Presets() []string {
	return []string{"ollama", "lmstudio", "llamacpp", "openai", "anthropic", "openrouter", "gemini", "github", "huggingface"}
}
