package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/schema"
)

// TestEveryKeyRenders walks every key in the reference on both an empty and a
// populated document, rendering the row and the detail pane for each one.
func TestEveryKeyRenders(t *testing.T) {
	for _, body := range []string{"{}", sampleSettings} {
		a, _ := newTestApp(t, body)
		for _, f := range schema.All() {
			a.jumpTo(f)
			if view := a.View(); view == "" {
				t.Fatalf("%s rendered nothing", f.Path)
			}
		}
	}
}

// TestEveryKeyOpensAnEditor opens each key's editor and closes it again, so a
// kind with no handler cannot slip through.
func TestEveryKeyOpensAnEditor(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	for _, f := range schema.All() {
		if f.GlobalConfig() {
			continue // belongs in ~/.claude.json, which this file is not
		}
		a.stacks = map[int][]level{}
		a.jumpTo(f)
		r, ok := a.level().current()
		if !ok || r.field.Path != f.Path {
			continue // nested under a parent the level does not list
		}
		depth := len(a.stack())
		press(a, "enter")
		if a.ov == nil && len(a.stack()) == depth {
			t.Errorf("%s (%s) opened neither an editor nor a level", f.Path, f.Kind)
			continue
		}
		press(a, "esc")
	}
}

// TestRealSettingsFileLoads reads the machine's own settings file when there is
// one, which catches shapes the fixtures do not cover. It never writes.
func TestRealSettingsFileLoads(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	path := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("no user settings file on this machine")
	}

	c := config.Context{Name: "user", Path: path, Kind: config.KindUser, ReadOnly: true}
	if err := c.Load(); err != nil {
		t.Fatalf("the real settings file does not load: %v", err)
	}
	a := New([]config.Context{c})
	a.Update(sizeMsg(120, 40))

	for i := range tabs {
		a.tabIdx = i
		if a.View() == "" {
			t.Fatalf("tab %q rendered nothing", tabs[i].Title)
		}
		for j := 0; j < len(a.level().visible()); j++ {
			a.level().cursor = j
			if a.View() == "" {
				t.Fatalf("tab %q row %d rendered nothing", tabs[i].Title, j)
			}
		}
	}

	// A read-only load must round-trip byte for byte.
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(c.Doc.Encode()); got != string(onDisk) {
		t.Errorf("re-encoding the real settings file changed it; a save would rewrite more than the edit")
	}
}
