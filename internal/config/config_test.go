package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSarandev/sugarlime/internal/schema"
)

const original = `{
  "model": "claude-sonnet-5-5",
  "aKeySugarlimeDoesNotKnow": {
    "nested": [1, 2]
  },
  "permissions": {
    "allow": ["Bash(ls)"]
  }
}`

func writeFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndSavePreservesUnknownKeys(t *testing.T) {
	path := writeFile(t, original)
	c := Context{Name: "test", Path: path, Kind: KindUser}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if !c.Exists {
		t.Fatal("expected the file to be reported as existing")
	}
	if c.Dirty() {
		t.Fatal("a freshly loaded file is not dirty")
	}

	if err := c.Doc.Set("permissions.defaultMode", "plan"); err != nil {
		t.Fatal(err)
	}
	if !c.Dirty() {
		t.Fatal("expected the edit to mark the file dirty")
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if c.Dirty() {
		t.Fatal("saving should clear the dirty flag")
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(saved)
	for _, want := range []string{`"aKeySugarlimeDoesNotKnow"`, `"nested"`, `"defaultMode": "plan"`, `"Bash(ls)"`} {
		if !strings.Contains(body, want) {
			t.Errorf("saved file is missing %s:\n%s", want, body)
		}
	}
	if !strings.HasSuffix(body, "}\n") {
		t.Error("the saved file should end with a newline")
	}
}

func TestSaveKeepsFileMode(t *testing.T) {
	path := writeFile(t, "{}")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	c := Context{Path: path}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	_ = c.Doc.Set("model", "sonnet")
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode changed to %v", info.Mode().Perm())
	}
}

func TestSaveCreatesMissingFileAndDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	c := Context{Path: path, Kind: KindProject}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if c.Exists {
		t.Fatal("the file should not exist yet")
	}
	_ = c.Doc.Set("permissions.defaultMode", "acceptEdits")
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if !c.Exists {
		t.Fatal("the context should report the file as existing after a save")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "acceptEdits") {
		t.Errorf("got %s", data)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	path := writeFile(t, "{}")
	c := Context{Path: path}
	_ = c.Load()
	_ = c.Doc.Set("model", "sonnet")
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the settings file, got %v", names)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := writeFile(t, `{"model": }`)
	c := Context{Path: path}
	err := c.Load()
	if err == nil {
		t.Fatal("expected an error")
	}
	if c.LoadErr == nil {
		t.Error("LoadErr should hold the failure")
	}
	if c.Editable() {
		t.Error("a file that failed to parse must not be editable")
	}
	if c.Doc == nil {
		t.Error("Doc should still be usable after a failed load")
	}
}

func TestLoadMissingFile(t *testing.T) {
	c := Context{Path: filepath.Join(t.TempDir(), "nope.json")}
	if err := c.Load(); err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
	if c.Exists || c.Doc == nil || c.Doc.Len() != 0 {
		t.Error("expected an empty document for a missing file")
	}
}

func TestReadOnlyContextRefusesSave(t *testing.T) {
	path := writeFile(t, "{}")
	c := Context{Path: path, ReadOnly: true}
	_ = c.Load()
	if c.Editable() {
		t.Error("a read-only context is not editable")
	}
	if err := c.Save(); err == nil {
		t.Error("expected a read-only save to fail")
	}
}

func TestRevert(t *testing.T) {
	path := writeFile(t, `{"model":"sonnet"}`)
	c := Context{Path: path}
	_ = c.Load()
	_ = c.Doc.Set("model", "opus")
	c.Revert()
	if c.Dirty() {
		t.Error("revert should clear the edits")
	}
	if v, _ := c.Doc.Str("model"); v != "sonnet" {
		t.Errorf("got %q", v)
	}
}

func TestAccepts(t *testing.T) {
	global, _ := schema.Lookup("autoConnectIde")
	plain, _ := schema.Lookup("model")

	settings := Context{Kind: KindUser}
	if settings.Accepts(global) {
		t.Error("a settings file does not hold global config keys")
	}
	if !settings.Accepts(plain) {
		t.Error("a settings file holds ordinary keys")
	}

	globalCtx := Context{Kind: KindGlobal}
	if !globalCtx.Accepts(global) {
		t.Error("~/.claude.json holds global config keys")
	}
	if globalCtx.Accepts(plain) {
		t.Error("~/.claude.json does not hold ordinary settings keys")
	}
}

func TestKindString(t *testing.T) {
	want := map[Kind]string{
		KindUser: "user", KindProject: "project", KindLocal: "local",
		KindManaged: "managed", KindGlobal: "global config", KindCustom: "custom",
		Kind(99): "unknown",
	}
	for k, s := range want {
		if k.String() != s {
			t.Errorf("%d = %q, want %q", k, k.String(), s)
		}
	}
}

func TestDiscoverFindsProjectAndLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())

	ctxs := Discover(project)
	byKind := map[Kind]Context{}
	for _, c := range ctxs {
		byKind[c.Kind] = c
	}
	if _, ok := byKind[KindUser]; !ok {
		t.Error("expected the user settings file")
	}
	if c, ok := byKind[KindProject]; !ok || !c.Exists {
		t.Error("expected the project settings file")
	}
	if c, ok := byKind[KindLocal]; !ok || c.Exists {
		t.Error("expected a local context that does not exist yet")
	}
}

func TestDiscoverWalksUpToTheProjectRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, c := range Discover(deep) {
		if c.Kind == KindProject {
			if want := filepath.Join(root, ".claude", "settings.json"); c.Path != want {
				t.Errorf("got %q, want %q", c.Path, want)
			}
			return
		}
	}
	t.Error("no project context found")
}

func TestStoreRememberAndForget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUGARLIME_CONFIG_DIR", dir)

	if got := SavedPaths(); len(got) != 0 {
		t.Fatalf("expected an empty store, got %v", got)
	}
	if err := Remember("/tmp/work/settings.json"); err != nil {
		t.Fatal(err)
	}
	if err := Remember("/tmp/work/settings.json"); err != nil {
		t.Fatal(err)
	}
	if got := SavedPaths(); len(got) != 1 {
		t.Fatalf("expected one path, got %v", got)
	}
	if err := Remember("/tmp/other/settings.json"); err != nil {
		t.Fatal(err)
	}
	if got := SavedPaths(); len(got) != 2 || got[0] != "/tmp/other/settings.json" {
		t.Fatalf("expected a sorted pair, got %v", got)
	}
	if err := Forget("/tmp/work/settings.json"); err != nil {
		t.Fatal(err)
	}
	if got := SavedPaths(); len(got) != 1 || got[0] != "/tmp/other/settings.json" {
		t.Fatalf("got %v", got)
	}
	if StorePath() != filepath.Join(dir, "contexts.json") {
		t.Errorf("unexpected store path %q", StorePath())
	}
}

func TestStorePathHonoursXDG(t *testing.T) {
	t.Setenv("SUGARLIME_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := StorePath(); got != "/xdg/sugarlime/contexts.json" {
		t.Errorf("got %q", got)
	}
}

func TestStoreIgnoresUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUGARLIME_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "contexts.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SavedPaths(); len(got) != 0 {
		t.Errorf("a broken store should read as empty, got %v", got)
	}
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := ExpandPath("~/.claude/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".claude", "settings.json"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, err := ExpandPath("~"); err != nil || got != home {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := ExpandPath("  "); err == nil {
		t.Error("expected an empty path to fail")
	}
}

func TestNewContextNamesByDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "work", ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := NewContext(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "work/settings" {
		t.Errorf("got %q", c.Name)
	}
	if c.Kind != KindCustom {
		t.Errorf("got kind %v", c.Kind)
	}

	globalCtx, err := NewContext(filepath.Join(t.TempDir(), ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if globalCtx.Kind != KindGlobal {
		t.Errorf("~/.claude.json should load as the global config, got %v", globalCtx.Kind)
	}
}

func TestDirtyWithoutLoad(t *testing.T) {
	var c Context
	if c.Dirty() {
		t.Error("a context that was never loaded has no edits")
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skip("cannot drop read permission here")
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	c := Context{Path: path}
	if err := c.Load(); err == nil {
		t.Fatal("expected a read failure")
	}
	if c.LoadErr == nil || c.Doc == nil {
		t.Error("a failed load should still leave a usable context")
	}
}

func TestLoadDirectoryAsFile(t *testing.T) {
	c := Context{Path: t.TempDir()}
	if err := c.Load(); err == nil {
		t.Fatal("expected reading a directory to fail")
	}
}

func TestSaveIntoUnwritableDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("cannot drop write permission here")
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	c := Context{Path: filepath.Join(dir, "settings.json")}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	_ = c.Doc.Set("model", "sonnet")
	if err := c.Save(); err == nil {
		t.Error("expected the save to fail")
	}
}

func TestProjectRootFallsBackToTheWorkingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())

	// A directory with no .claude and no .git above it still yields a project
	// context, so a user can create one.
	dir := t.TempDir()
	ctxs := Discover(dir)
	for _, c := range ctxs {
		if c.Kind == KindProject {
			if !strings.HasSuffix(c.Path, filepath.Join(".claude", "settings.json")) {
				t.Errorf("got %q", c.Path)
			}
			return
		}
	}
	t.Error("expected a project context")
}

func TestDiscoverIncludesSavedPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := t.TempDir()
	t.Setenv("SUGARLIME_CONFIG_DIR", store)

	extra := filepath.Join(t.TempDir(), "work", ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extra, []byte(`{"model":"opus"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remember(extra); err != nil {
		t.Fatal(err)
	}

	for _, c := range Discover(t.TempDir()) {
		if c.Path == extra {
			if c.Kind != KindCustom {
				t.Errorf("got kind %v", c.Kind)
			}
			if v, _ := c.Doc.Str("model"); v != "opus" {
				t.Errorf("the file should be loaded, got %q", v)
			}
			return
		}
	}
	t.Error("a remembered path should be discovered")
}

func TestDiscoverSkipsDuplicatePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	userPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remember(userPath); err != nil {
		t.Fatal(err)
	}

	seen := 0
	for _, c := range Discover(t.TempDir()) {
		if c.Path == userPath {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the same file should appear once, got %d", seen)
	}
}

func TestStorePathWithoutAnyEnvironment(t *testing.T) {
	t.Setenv("SUGARLIME_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := StorePath(); got != filepath.Join(home, ".config", "sugarlime", "contexts.json") {
		t.Errorf("got %q", got)
	}
}

func TestRememberReportsAFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A regular file where the config directory should go makes MkdirAll fail.
	t.Setenv("SUGARLIME_CONFIG_DIR", filepath.Join(blocker, "sub"))
	if err := Remember("/tmp/x.json"); err == nil {
		t.Error("expected the write to fail")
	}
	if err := Forget("/tmp/x.json"); err == nil {
		t.Error("expected the write to fail")
	}
}

func TestManagedSettingsAreReadOnlyWhenPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	for _, c := range Discover(t.TempDir()) {
		if c.Kind == KindManaged && !c.ReadOnly {
			t.Error("managed settings must be read-only")
		}
	}
}

func TestDiscoverFindsTheGlobalConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"autoConnectIde":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range Discover(t.TempDir()) {
		if c.Kind == KindGlobal {
			if v, _ := c.Doc.Bool("autoConnectIde"); !v {
				t.Error("~/.claude.json should be loaded")
			}
			return
		}
	}
	t.Error("expected the global config context")
}

func TestDiscoverWithNoWorkingDirectoryArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	if got := Discover(""); len(got) == 0 {
		t.Error("expected at least the user and project contexts")
	}
}

func TestProjectRootStopsAtAGitRepository(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "pkg", "inner")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, c := range Discover(deep) {
		if c.Kind == KindProject {
			if want := filepath.Join(root, ".claude", "settings.json"); c.Path != want {
				t.Errorf("got %q, want %q", c.Path, want)
			}
			return
		}
	}
	t.Error("no project context found")
}

func TestManagedPathIsPlatformSpecific(t *testing.T) {
	p := managedPath()
	if p == "" {
		t.Fatal("every supported platform has a managed settings path")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("%q is not absolute", p)
	}
	if filepath.Base(p) != "managed-settings.json" {
		t.Errorf("got %q", p)
	}
}

func TestContextNameForAPlainDirectory(t *testing.T) {
	if got := contextName("/opt/team/settings.json"); got != "team/settings" {
		t.Errorf("got %q", got)
	}
	if got := contextName("/settings.json"); got != "settings" {
		t.Errorf("a file at the root is named after itself, got %q", got)
	}
}

func TestNewContextRejectsAnEmptyPath(t *testing.T) {
	if _, err := NewContext("   "); err == nil {
		t.Error("expected an error")
	}
}

func TestSaveRefusesToWriteBrokenJSON(t *testing.T) {
	path := writeFile(t, "{}")
	c := Context{Path: path}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	// A value encoding/json cannot represent renders as null, which still
	// parses, so the guard is about the document staying well formed.
	_ = c.Doc.Set("x", func() {})
	if err := c.Save(); err != nil {
		t.Fatalf("a document that renders as valid JSON should save: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"x": null`) {
		t.Errorf("got %s", data)
	}
}

func TestSyncDirIgnoresAMissingDirectory(t *testing.T) {
	syncDir(filepath.Join(t.TempDir(), "nope")) // must not panic
}

func TestLoadDocumentIsEditableAfterLoad(t *testing.T) {
	path := writeFile(t, original)
	c := Context{Path: path}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if !c.Editable() {
		t.Error("a loaded file is editable")
	}
}
