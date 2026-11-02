package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Balanov/shim/internal/provider"
)

type FileTools struct{ root string }

func NewFileTools(root string) (*FileTools, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &FileTools{root: filepath.Clean(abs)}, nil
}

func (f *FileTools) Register(r *Registry) {
	r.Add(Tool{Level: Read, Spec: providerSpec("read_file", "Read a UTF-8 text file from the workspace.", `{"path":{"type":"string"},"start_line":{"type":"integer","minimum":1},"end_line":{"type":"integer","minimum":1}}`, "path"), Run: f.read})
	r.Add(Tool{Level: Write, Spec: providerSpec("write_file", "Create or completely replace a UTF-8 text file inside the workspace.", `{"path":{"type":"string"},"content":{"type":"string"}}`, "path", "content"), Run: f.write})
	r.Add(Tool{Level: Write, Spec: providerSpec("edit_file", "Replace one unique exact string in a UTF-8 text file.", `{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"}}`, "path", "old_string", "new_string"), Run: f.edit})
	r.Add(Tool{Level: Read, Spec: providerSpec("list_files", "List workspace files, skipping dependencies and VCS metadata.", `{"path":{"type":"string"}}`), Run: f.list})
	r.Add(Tool{Level: Read, Spec: providerSpec("search_files", "Search workspace text files for a literal case-insensitive string.", `{"query":{"type":"string"},"path":{"type":"string"}}`, "query"), Run: f.search})
}

func providerSpec(name, description, properties string, required ...string) provider.ToolSpec {
	return provider.ToolSpec{Name: name, Description: description, Parameters: Schema(properties, required...)}
}

func (f *FileTools) secure(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(f.root, abs)
	}
	abs = filepath.Clean(abs)
	if !within(f.root, abs) {
		return "", fmt.Errorf("path escapes workspace: %s", path)
	}
	root := f.root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	probe := abs
	for {
		if _, err := os.Lstat(probe); err == nil {
			resolved, err := filepath.EvalSymlinks(probe)
			if err != nil {
				return "", err
			}
			if !within(root, resolved) {
				return "", fmt.Errorf("path escapes workspace through symlink: %s", path)
			}
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("cannot resolve path: %s", path)
		}
		probe = parent
	}
	return abs, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (f *FileTools) read(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	path, err := f.secure(in.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 2<<20 {
		return "", fmt.Errorf("file is larger than 2 MiB; use start_line/end_line via a smaller source file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("binary files are not supported")
	}
	lines := strings.Split(string(data), "\n")
	start, end := in.StartLine, in.EndLine
	if start == 0 {
		start = 1
	}
	if end == 0 || end > len(lines) {
		end = len(lines)
	}
	if start < 1 || start > end || start > len(lines) {
		return "", fmt.Errorf("invalid line range")
	}
	if end-start > 2000 {
		end = start + 2000
	}
	var out strings.Builder
	for i := start; i <= end; i++ {
		fmt.Fprintf(&out, "%d\t%s\n", i, lines[i-1])
	}
	return out.String(), nil
}

func (f *FileTools) write(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct{ Path, Content string }
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	path, err := f.secure(in.Path)
	if err != nil {
		return "", err
	}
	if len(in.Content) > 2<<20 {
		return "", fmt.Errorf("content exceeds 2 MiB limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %s (%d bytes)", in.Path, len(in.Content)), nil
}

func (f *FileTools) edit(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	path, err := f.secure(in.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 2<<20 {
		return "", fmt.Errorf("file exceeds 2 MiB limit")
	}
	count := strings.Count(string(data), in.OldString)
	if in.OldString == "" || count != 1 {
		return "", fmt.Errorf("old_string must occur exactly once; found %d", count)
	}
	updated := strings.Replace(string(data), in.OldString, in.NewString, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return "Edited " + in.Path, nil
}

func (f *FileTools) list(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct{ Path string }
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	path, err := f.secure(in.Path)
	if err != nil {
		return "", err
	}
	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skipDir(d.Name()) && p != path {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(f.root, p)
			files = append(files, filepath.ToSlash(rel))
			if len(files) >= 500 {
				return fs.SkipAll
			}
		}
		return nil
	})
	sort.Strings(files)
	return strings.Join(files, "\n"), err
}

func (f *FileTools) search(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct{ Path, Query string }
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	if in.Query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}
	path, err := f.secure(in.Path)
	if err != nil {
		return "", err
	}
	query := strings.ToLower(in.Query)
	var matches []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) && p != path {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if strings.Contains(strings.ToLower(text), query) {
				rel, _ := filepath.Rel(f.root, p)
				matches = append(matches, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), line, strings.TrimSpace(text)))
				if len(matches) >= 100 {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	return strings.Join(matches, "\n"), err
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".venv", "venv", "vendor", "dist", "build", "target", "__pycache__":
		return true
	}
	return false
}
