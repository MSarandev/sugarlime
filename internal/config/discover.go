package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover returns the well-known Claude settings files plus any the user added.
func Discover(workdir string) []Context {
	var out []Context
	seen := map[string]bool{}

	add := func(c Context) {
		if c.Path == "" || seen[c.Path] {
			return
		}
		seen[c.Path] = true
		out = append(out, c)
	}

	for _, c := range userContexts() {
		add(c)
	}

	if root := projectRoot(workdir); root != "" {
		add(Context{
			Name: "project",
			Path: filepath.Join(root, ".claude", "settings.json"),
			Kind: KindProject,
		})
		add(Context{
			Name: "local",
			Path: filepath.Join(root, ".claude", "settings.local.json"),
			Kind: KindLocal,
		})
	}

	if p := managedPath(); p != "" {
		if _, err := os.Stat(p); err == nil {
			add(Context{Name: "managed", Path: p, Kind: KindManaged, ReadOnly: true})
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".claude.json")
		if _, err := os.Stat(p); err == nil {
			add(Context{Name: "global config", Path: p, Kind: KindGlobal})
		}
	}

	for _, p := range SavedPaths() {
		add(Context{Name: contextName(p), Path: p, Kind: KindCustom})
	}

	for i := range out {
		_ = out[i].Load()
	}
	return out
}

// userContexts finds the user-level settings files, including the extra Claude
// configuration directories people keep for separate environments: whatever
// CLAUDE_CONFIG_DIR points at, and any ~/.claude* or ~/.config/claude* sibling.
func userContexts() []Context {
	var out []Context
	seen := map[string]bool{}

	addDir := func(dir, name string, primary bool) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true

		settings := filepath.Join(dir, "settings.json")
		local := filepath.Join(dir, "settings.local.json")
		_, hasSettings := statFile(settings)
		_, hasLocal := statFile(local)
		if !primary && !hasSettings && !hasLocal {
			return
		}

		if primary || hasSettings {
			out = append(out, Context{Name: name, Path: settings, Kind: KindUser})
		}
		if hasLocal {
			out = append(out, Context{Name: name + " local", Path: local, Kind: KindUser})
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	// CLAUDE_CONFIG_DIR moves the whole Claude directory, so it is the one a
	// session is actually using when it is set.
	if env := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); env != "" {
		if abs, err := ExpandPath(env); err == nil {
			addDir(abs, envDirName(abs, home), true)
		}
	}

	if home == "" {
		return out
	}
	addDir(filepath.Join(home, ".claude"), "user", true)

	for _, dir := range append(
		globDirs(filepath.Join(home, ".claude*")),
		globDirs(filepath.Join(home, ".config", "claude*"))...,
	) {
		addDir(dir, envDirName(dir, home), false)
	}
	return out
}

// envDirName labels a Claude directory by its own name, so ~/.claude-work
// reads as "claude-work" rather than another "user".
func envDirName(dir, home string) string {
	base := strings.TrimPrefix(filepath.Base(dir), ".")
	if home != "" && dir == filepath.Join(home, ".claude") {
		return "user"
	}
	if base == "" {
		return "user"
	}
	return base
}

func globDirs(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range matches {
		if isDir(m) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

func statFile(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, false
	}
	return info, true
}

// scanLimits keep a scan of someone's home directory from taking all day.
const (
	scanMaxDepth   = 5
	scanMaxResults = 200
)

var scanSkipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, ".venv": true,
	"venv": true, "target": true, "dist": true, "build": true,
	"Library": true, ".cache": true, ".Trash": true, "Pictures": true,
	"go": true, ".npm": true, ".cargo": true, ".rustup": true,
}

// ScanDir walks root for Claude settings files, so a directory of repositories
// can be added in one go. It follows no symlinks and stops at a bounded depth.
func ScanDir(root string) ([]string, error) {
	start, err := ExpandPath(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(start)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "scan", Path: start, Err: os.ErrInvalid}
	}

	var found []string
	baseDepth := strings.Count(filepath.Clean(start), string(filepath.Separator))

	err = filepath.WalkDir(start, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable directory is skipped, not fatal
		}
		if len(found) >= scanMaxResults {
			return filepath.SkipAll
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != start && (scanSkipDirs[name] || (strings.HasPrefix(name, ".") && name != ".claude" && name != ".config")) {
			return filepath.SkipDir
		}
		if strings.Count(filepath.Clean(path), string(filepath.Separator))-baseDepth > scanMaxDepth {
			return filepath.SkipDir
		}
		for _, candidate := range []string{"settings.json", "settings.local.json"} {
			if name == ".claude" || strings.HasPrefix(name, "claude") {
				if p := filepath.Join(path, candidate); fileExists(p) {
					found = append(found, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		return found, err
	}
	sort.Strings(found)
	return found, nil
}

func fileExists(path string) bool {
	_, ok := statFile(path)
	return ok
}

// projectRoot walks up to the .claude directory or repository root.
func projectRoot(dir string) string {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	cur := abs
	for {
		if isDir(filepath.Join(cur, ".claude")) || isDir(filepath.Join(cur, ".git")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		cur = parent
	}
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func managedPath() string {
	switch runtimeGOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode/managed-settings.json"
	case "windows":
		return filepath.Join(os.Getenv("PROGRAMDATA"), "ClaudeCode", "managed-settings.json")
	default:
		return "/etc/claude-code/managed-settings.json"
	}
}

// contextName labels a file by its parent directory, not "settings.json".
func contextName(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	parent := filepath.Base(filepath.Dir(path))
	if parent == ".claude" {
		parent = filepath.Base(filepath.Dir(filepath.Dir(path)))
	}
	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		return base
	}
	return parent + "/" + base
}
