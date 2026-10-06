// Package state records which files kit has written, so it can back them up,
// update them, and remove them safely.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/valenciajoel/kit/internal/inventory"
)

// Entry records a single file kit owns.
type Entry struct {
	Path      string `yaml:"path"`
	Component string `yaml:"component"`
	Source    string `yaml:"source"`
	Hash      string `yaml:"hash"`
	Backup    string `yaml:"backup,omitempty"`
	WrittenAt string `yaml:"written_at"`
}

// Store is the persisted state document.
type Store struct {
	Version   int     `yaml:"version"`
	UpdatedAt string  `yaml:"updated_at"`
	Files     []Entry `yaml:"files"`
	dir       string
}

// Dir returns the directory that holds kit's state and backups.
func Dir(env inventory.Environment) string {
	base := env.ConfigDir
	if base == "" && env.Home != "" {
		base = filepath.Join(env.Home, ".config")
	}
	return filepath.Join(base, "kit")
}

// FilePath returns the state.json path for the environment.
func FilePath(env inventory.Environment) string {
	return filepath.Join(Dir(env), "state.json")
}

// Load reads state.json, returning an empty store when it does not exist yet.
func Load(env inventory.Environment) (*Store, error) {
	s := &Store{Version: 1, dir: Dir(env)}
	data, err := os.ReadFile(FilePath(env))
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", FilePath(env), err)
	}
	s.dir = Dir(env)
	return s, nil
}

// Save writes the store atomically.
func (s *Store) Save() error {
	s.Version = 1
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "state.json"), data, 0o644)
}

// Len reports how many files are managed.
func (s *Store) Len() int { return len(s.Files) }

// Find returns the entry for a path.
func (s *Store) Find(path string) (Entry, bool) {
	for _, e := range s.Files {
		if e.Path == path {
			return e, true
		}
	}
	return Entry{}, false
}

// Record inserts or updates the entry for a path.
func (s *Store) Record(e Entry) {
	for i := range s.Files {
		if s.Files[i].Path == e.Path {
			s.Files[i] = e
			return
		}
	}
	s.Files = append(s.Files, e)
}

// Remove drops the entry for a path.
func (s *Store) Remove(path string) {
	out := s.Files[:0]
	for _, e := range s.Files {
		if e.Path != path {
			out = append(out, e)
		}
	}
	s.Files = out
}

// BackupExisting copies an existing file into the backups tree and returns the
// backup path. It returns "" when there is nothing to back up.
func (s *Store) BackupExisting(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	dst := filepath.Join(s.dir, "backups", stamp, sanitizePath(path))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// Hash returns a stable content hash used to detect user edits.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func sanitizePath(p string) string {
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "/")
	return strings.NewReplacer("/", "_", ":", "_").Replace(p)
}
