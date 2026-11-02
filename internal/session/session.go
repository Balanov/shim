package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Balanov/shim/internal/provider"
)

type Store struct {
	Dir     string
	ID      string
	path    string
	started bool
}

func New(dir string) *Store { return &Store{Dir: dir} }

func (s *Store) Create() error {
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	s.ID = time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(random[:])
	s.path = filepath.Join(s.Dir, s.ID+".jsonl")
	s.started = true
	return os.MkdirAll(s.Dir, 0o700)
}

func (s *Store) Append(m provider.Message) error {
	if !s.started {
		if err := s.Create(); err != nil {
			return err
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func (s *Store) Resume(id string) ([]provider.Message, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid session id")
	}
	path := filepath.Join(s.Dir, id+".jsonl")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var messages []provider.Message
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		var m provider.Message
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("corrupt session %s: %w", id, err)
		}
		messages = append(messages, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	s.ID, s.path, s.started = id, path, true
	return messages, nil
}

type Info struct {
	ID       string    `json:"id"`
	Messages int       `json:"messages"`
	Updated  time.Time `json:"updated"`
	Bytes    int64     `json:"bytes"`
}

func (s *Store) List() ([]Info, error) {
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		stat, err := e.Info()
		if err != nil {
			continue
		}
		count := 0
		file, err := os.Open(filepath.Join(s.Dir, e.Name()))
		if err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				count++
			}
			file.Close()
		}
		out = append(out, Info{ID: strings.TrimSuffix(e.Name(), ".jsonl"), Messages: count, Updated: stat.ModTime(), Bytes: stat.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

func validID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
