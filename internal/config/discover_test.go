package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claudeEnv lays out a home directory with several Claude environments in it.
func claudeEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	write := func(parts ...string) {
		path := filepath.Join(append([]string{home}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"model":"opus"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude", "settings.json")
	write(".claude-work", "settings.json")
	write(".claude-work", "settings.local.json")
	write(".claude-personal", "settings.json")
	write(".config", "claude-client", "settings.json")
	write(".claude.json")
	if err := os.MkdirAll(filepath.Join(home, ".claude-empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func names(ctxs []Context) []string {
	out := make([]string, len(ctxs))
	for i, c := range ctxs {
		out[i] = c.Name
	}
	return out
}

func find(ctxs []Context, name string) (Context, bool) {
	for _, c := range ctxs {
		if c.Name == name {
			return c, true
		}
	}
	return Context{}, false
}

func TestDiscoverFindsSeveralClaudeEnvironments(t *testing.T) {
	home := claudeEnv(t)
	ctxs := Discover(t.TempDir())

	for _, want := range []string{"user", "claude-work", "claude-work local", "claude-personal", "claude-client"} {
		c, ok := find(ctxs, want)
		if !ok {
			t.Errorf("%q was not discovered; got %v", want, names(ctxs))
			continue
		}
		if c.Kind != KindUser {
			t.Errorf("%q should be a user-level file, got %v", want, c.Kind)
		}
		if !c.Exists {
			t.Errorf("%q should exist", want)
		}
	}

	if c, ok := find(ctxs, "user"); !ok || c.Path != filepath.Join(home, ".claude", "settings.json") {
		t.Errorf("the plain ~/.claude file should be named user, got %+v", c)
	}
	if _, ok := find(ctxs, "claude-empty"); ok {
		t.Error("a Claude directory with no settings file should not be listed")
	}
}

func TestDiscoverPutsClaudeConfigDirFirst(t *testing.T) {
	home := claudeEnv(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude-work"))

	ctxs := Discover(t.TempDir())
	if len(ctxs) == 0 {
		t.Fatal("nothing discovered")
	}
	if ctxs[0].Path != filepath.Join(home, ".claude-work", "settings.json") {
		t.Errorf("the environment a session would use should come first, got %q", ctxs[0].Path)
	}
	if _, ok := find(ctxs, "user"); !ok {
		t.Error("the other environments should still be listed")
	}
}

func TestDiscoverHonoursTildeInClaudeConfigDir(t *testing.T) {
	home := claudeEnv(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "~/.claude-personal")

	ctxs := Discover(t.TempDir())
	if ctxs[0].Path != filepath.Join(home, ".claude-personal", "settings.json") {
		t.Errorf("got %q", ctxs[0].Path)
	}
}

func TestDiscoverWithClaudeConfigDirThatDoesNotExist(t *testing.T) {
	claudeEnv(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "nowhere"))

	ctxs := Discover(t.TempDir())
	if len(ctxs) == 0 {
		t.Fatal("the other files should still be discovered")
	}
	if !strings.HasSuffix(ctxs[0].Path, filepath.Join("nowhere", "settings.json")) {
		t.Errorf("the named directory should be offered so it can be created, got %q", ctxs[0].Path)
	}
	if ctxs[0].Exists {
		t.Error("it should be reported as not existing yet")
	}
}

func TestDiscoverListsEachFileOnce(t *testing.T) {
	claudeEnv(t)
	seen := map[string]int{}
	for _, c := range Discover(t.TempDir()) {
		seen[c.Path]++
	}
	for path, n := range seen {
		if n != 1 {
			t.Errorf("%s appears %d times", path, n)
		}
	}
}

func TestEnvDirName(t *testing.T) {
	home := "/home/alice"
	cases := map[string]string{
		"/home/alice/.claude":          "user",
		"/home/alice/.claude-work":     "claude-work",
		"/home/alice/.config/claude-x": "claude-x",
		"/opt/claude":                  "claude",
	}
	for dir, want := range cases {
		if got := envDirName(dir, home); got != want {
			t.Errorf("envDirName(%q) = %q, want %q", dir, got, want)
		}
	}
	if got := envDirName("/home/alice/.claude", ""); got != "claude" {
		t.Errorf("without a home directory the name comes from the path, got %q", got)
	}
}

func TestScanDirFindsSettingsFiles(t *testing.T) {
	root := t.TempDir()
	make := func(parts ...string) string {
		path := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	api := make("api", ".claude", "settings.json")
	web := make("web", ".claude", "settings.local.json")
	make("web", ".claude", "CLAUDE.md")
	make("node_modules", "pkg", ".claude", "settings.json")
	make("api", "src", "main.go")

	found, err := ScanDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("got %v", found)
	}
	if found[0] != api || found[1] != web {
		t.Errorf("got %v, want %v and %v", found, api, web)
	}
	for _, f := range found {
		if strings.Contains(f, "node_modules") {
			t.Error("node_modules should be skipped")
		}
	}
}

func TestScanDirHonoursTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "code", ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := ScanDir("~/code")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != path {
		t.Errorf("got %v", found)
	}
}

func TestScanDirRejectsNonDirectories(t *testing.T) {
	if _, err := ScanDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("expected a missing directory to fail")
	}

	file := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ScanDir(file); err == nil {
		t.Error("expected a file to fail")
	}
	if _, err := ScanDir("  "); err == nil {
		t.Error("expected an empty path to fail")
	}
}

func TestScanDirStopsAtTheDepthLimit(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c", "d", "e", "f", "g", ".claude")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := ScanDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("a file below the depth limit should not be found, got %v", found)
	}
}

func TestScanDirFindsAClaudeDirectoryAtTheRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := ScanDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Errorf("got %v", found)
	}
}

func TestScanDirOnTheRootOfAClaudeDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Errorf("scanning a .claude directory itself should find its settings, got %v", found)
	}
}

func TestManagedPathPerPlatform(t *testing.T) {
	saved := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = saved })

	runtimeGOOS = "darwin"
	if got := managedPath(); !strings.HasPrefix(got, "/Library/Application Support/ClaudeCode") {
		t.Errorf("darwin: %q", got)
	}
	runtimeGOOS = "linux"
	if got := managedPath(); got != "/etc/claude-code/managed-settings.json" {
		t.Errorf("linux: %q", got)
	}
	runtimeGOOS = "windows"
	t.Setenv("PROGRAMDATA", filepath.Join("C:", "ProgramData"))
	if got := managedPath(); !strings.Contains(got, "ClaudeCode") {
		t.Errorf("windows: %q", got)
	}
}

func TestGlobDirsSkipsFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := globDirs(filepath.Join(root, ".claude*"))
	if len(got) != 1 || filepath.Base(got[0]) != ".claude-a" {
		t.Errorf("got %v", got)
	}
	if got := globDirs("[["); got != nil {
		t.Errorf("a bad pattern should yield nothing, got %v", got)
	}
}
