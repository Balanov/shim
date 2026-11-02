package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/Balanov/shim/internal/config"
)

type Discovered struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
}

func DiscoverLocal(ctx context.Context) []Discovered {
	presets := []string{"ollama", "lmstudio", "llamacpp"}
	results := make(chan []Discovered, len(presets))
	var wg sync.WaitGroup
	for _, name := range presets {
		preset, _ := config.Preset(name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
			defer cancel()
			results <- discoverOne(ctx, preset)
		}()
	}
	wg.Wait()
	close(results)
	var out []Discovered
	for group := range results {
		out = append(out, group...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].Model < out[j].Model
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}

func discoverOne(ctx context.Context, preset config.Model) []Discovered {
	url := endpoint(preset.BaseURL, "/models")
	ollama := preset.Provider == "ollama"
	if ollama {
		url = endpoint(preset.BaseURL, "/api/tags")
		// Ollama's native endpoint is one level above /v1.
		url = "http://127.0.0.1:11434/api/tags"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var raw struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if json.NewDecoder(resp.Body).Decode(&raw) != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Discovered
	if ollama {
		for _, m := range raw.Models {
			if m.Name != "" && !seen[m.Name] {
				seen[m.Name] = true
				out = append(out, Discovered{Name: preset.Provider + "/" + m.Name, Provider: preset.Provider, Model: m.Name, BaseURL: preset.BaseURL})
			}
		}
	} else {
		for _, m := range raw.Data {
			if m.ID != "" && !seen[m.ID] {
				seen[m.ID] = true
				out = append(out, Discovered{Name: preset.Provider + "/" + m.ID, Provider: preset.Provider, Model: m.ID, BaseURL: preset.BaseURL})
			}
		}
	}
	return out
}
