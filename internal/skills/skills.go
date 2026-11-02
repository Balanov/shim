package skills

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Balanov/shim/internal/provider"
	"github.com/Balanov/shim/internal/tools"
)

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type Manager struct {
	GlobalDir string
	Workspace string
}

func (m Manager) roots() []string {
	return []string{
		m.GlobalDir,
		filepath.Join(m.Workspace, ".shim", "skills"),
		filepath.Join(m.Workspace, ".agents", "skills"),
		filepath.Join(m.Workspace, ".claude", "skills"),
	}
}

func (m Manager) List() ([]Skill, error) {
	found := map[string]Skill{}
	for _, root := range m.roots() {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name(), "SKILL.md")
			if _, err := os.Stat(path); err != nil {
				continue
			}
			skill, err := Parse(path)
			if err != nil {
				continue
			}
			found[skill.Name] = skill
		}
	}
	out := make([]Skill, 0, len(found))
	for _, s := range found {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Parse(path string) (Skill, error) {
	file, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer file.Close()
	s := Skill{Name: filepath.Base(filepath.Dir(path)), Path: path}
	scanner := bufio.NewScanner(io.LimitReader(file, 1<<20))
	frontmatter, ended := false, false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if lineNo == 1 && line == "---" {
			frontmatter = true
			continue
		}
		if frontmatter && line == "---" {
			ended = true
			break
		}
		if frontmatter {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			switch strings.TrimSpace(key) {
			case "name":
				if value != "" {
					s.Name = value
				}
			case "description":
				s.Description = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Skill{}, err
	}
	if frontmatter && !ended {
		return Skill{}, fmt.Errorf("unclosed frontmatter in %s", path)
	}
	return s, nil
}

func (m Manager) Load(name string) (string, error) {
	list, err := m.List()
	if err != nil {
		return "", err
	}
	for _, s := range list {
		if s.Name == name {
			data, err := os.ReadFile(s.Path)
			if err != nil {
				return "", err
			}
			if len(data) > 1<<20 {
				return "", fmt.Errorf("skill %s exceeds 1 MiB", name)
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf("unknown skill %q", name)
}

func (m Manager) Register(registry *tools.Registry) {
	registry.Add(tools.Tool{Level: tools.Read, Spec: provider.ToolSpec{
		Name: "load_skill", Description: "Load the full instructions for an available skill by name. Skills are listed in the system prompt.",
		Parameters: tools.Schema(`{"name":{"type":"string"}}`, "name"),
	}, Run: func(_ context.Context, raw json.RawMessage) (string, error) {
		var in struct{ Name string }
		if err := json.Unmarshal(raw, &in); err != nil {
			return "", err
		}
		return m.Load(in.Name)
	}})
}

func (m Manager) Prompt() string {
	list, err := m.List()
	if err != nil || len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Available skills (load only when relevant with load_skill):\n")
	for _, s := range list {
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
	}
	return b.String()
}

func (m Manager) Install(ctx context.Context, source string) ([]string, error) {
	if source == "" {
		return nil, fmt.Errorf("source cannot be empty")
	}
	tmp, err := os.MkdirTemp("", "shim-skill-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	src := source
	if strings.HasPrefix(source, "https://github.com/") || strings.HasPrefix(source, "git@github.com:") || strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") || (strings.Count(source, "/") == 1 && !strings.HasPrefix(source, ".") && !filepath.IsAbs(source)) {
		if strings.Count(source, "/") == 1 && !strings.Contains(source, "://") {
			source = "https://github.com/" + source + ".git"
		}
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", source, tmp)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		src = tmp
	}
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if info.IsDir() && filepath.Base(src) == "SKILL.md" {
		src = filepath.Dir(src)
	}
	var installed []string
	if info.Name() == "SKILL.md" {
		return nil, fmt.Errorf("pass the skill directory, not SKILL.md")
	}
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		skill, err := Parse(path)
		if err != nil {
			return err
		}
		if !validName(skill.Name) {
			return fmt.Errorf("invalid skill name %q", skill.Name)
		}
		dest := filepath.Join(m.GlobalDir, skill.Name)
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("skill %s already exists", skill.Name)
		}
		if err := copyDir(filepath.Dir(path), dest); err != nil {
			return err
		}
		installed = append(installed, skill.Name)
		return nil
	})
	if len(installed) == 0 && err == nil {
		err = fmt.Errorf("no SKILL.md found in %s", source)
	}
	return installed, err
}

func (m Manager) Remove(name string) error {
	if !validName(name) {
		return fmt.Errorf("invalid skill name")
	}
	path := filepath.Join(m.GlobalDir, name)
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func validName(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
