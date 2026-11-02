package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	model, _ := Preset("openrouter")
	model.Model = "example/model"
	model.Name = "main"
	cfg.UpsertModel(model)
	cfg.DefaultModel = "main"
	cfg.RouterModel = "main"
	cfg.Language = "ru"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultModel != "main" || loaded.Models[0].Model != "example/model" || loaded.Language != "ru" {
		t.Fatalf("unexpected config: %+v", loaded)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatal("config unexpectedly contains a secret")
	}
}

func TestValidation(t *testing.T) {
	cfg := Default()
	cfg.DefaultModel = "missing"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing default model to fail")
	}
}

func TestPresets(t *testing.T) {
	for _, name := range Presets() {
		model, err := Preset(name)
		if err != nil || model.BaseURL == "" {
			t.Fatalf("bad preset %s: %+v, %v", name, model, err)
		}
	}
}
