package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverLoadAndInstall(t *testing.T) {
	root := t.TempDir()
	global := filepath.Join(root, "global")
	workspace := filepath.Join(root, "work")
	skillDir := filepath.Join(workspace, ".agents", "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: review\ndescription: Review changes carefully\n---\n\nAlways run tests.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := Manager{GlobalDir: global, Workspace: workspace}
	list, err := manager.List()
	if err != nil || len(list) != 1 || list[0].Name != "review" {
		t.Fatalf("unexpected skills: %+v %v", list, err)
	}
	loaded, err := manager.Load("review")
	if err != nil || !strings.Contains(loaded, "Always run tests") {
		t.Fatalf("load failed: %v", err)
	}
	if !strings.Contains(manager.Prompt(), "Review changes carefully") {
		t.Fatal("skill metadata missing from prompt")
	}
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: local-skill\ndescription: Local\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	installed, err := manager.Install(context.Background(), source)
	if err != nil || len(installed) != 1 || installed[0] != "local-skill" {
		t.Fatalf("install failed: %v %+v", err, installed)
	}
}
