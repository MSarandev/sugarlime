package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// store is sugarlime's own config: the extra settings files a user added.
type store struct {
	Paths []string `json:"paths"`
}

// StorePath is where sugarlime keeps its own list of added contexts.
func StorePath() string {
	if dir := os.Getenv("SUGARLIME_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "contexts.json")
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sugarlime", "contexts.json")
}

func loadStore() store {
	var s store
	path := StorePath()
	if path == "" {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

func saveStore(s store) error {
	path := StorePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// SavedPaths lists the settings files the user added by hand.
func SavedPaths() []string {
	paths := loadStore().Paths
	sort.Strings(paths)
	return paths
}

// Remember adds a path to sugarlime's own list of contexts.
func Remember(path string) error {
	s := loadStore()
	for _, p := range s.Paths {
		if p == path {
			return nil
		}
	}
	s.Paths = append(s.Paths, path)
	return saveStore(s)
}

// Forget drops a path from sugarlime's own list of contexts.
func Forget(path string) error {
	s := loadStore()
	kept := s.Paths[:0]
	for _, p := range s.Paths {
		if p != path {
			kept = append(kept, p)
		}
	}
	s.Paths = kept
	return saveStore(s)
}
