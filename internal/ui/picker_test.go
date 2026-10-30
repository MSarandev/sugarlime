package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MSarandev/sugarlime/internal/config"
)

// newPickerApp builds an app sitting on the file picker, as a real start does.
func newPickerApp(t *testing.T, files map[string]string) *App {
	t.Helper()
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())

	dir := t.TempDir()
	// Map order is random; the tests index files by position, so sort them.
	ordered := make([]string, 0, len(files))
	for name := range files {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	var ctxs []config.Context
	for _, name := range ordered {
		body := files[name]
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		c := config.Context{Name: strings.TrimSuffix(name, ".json"), Path: path, Kind: config.KindUser}
		_ = c.Load()
		ctxs = append(ctxs, c)
	}
	a := New(ctxs).WithWorkdir(dir)
	a.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	return a
}

func TestStartsOnTheFilePicker(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": `{"model":"opus"}`})
	if a.screen != screenFiles {
		t.Fatal("sugarlime should open on the file picker")
	}
	view := a.View()
	for _, want := range []string{
		"Which settings file do you want to edit?",
		"work",
		"Add a settings file by path",
		"Scan a directory for settings files",
		"q exit",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the picker is missing %q:\n%s", want, view)
		}
	}
}

func TestPickerShowsTheLogo(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	if !strings.Contains(a.View(), "SUGARLIME") && !strings.Contains(a.View(), "___") {
		t.Error("the picker should show the logo")
	}
}

func TestPickerShowsKeyCounts(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": `{"model":"opus","theme":"dark"}`})
	if !strings.Contains(a.View(), "2 keys") {
		t.Errorf("expected a key count:\n%s", a.View())
	}
}

func TestPickerShowsAFileThatDoesNotExistYet(t *testing.T) {
	a := newPickerApp(t, map[string]string{"missing.json": ""})
	if !strings.Contains(a.View(), "new file") {
		t.Errorf("expected the file to be flagged as new:\n%s", a.View())
	}
}

func TestPickerOpensTheChosenFile(t *testing.T) {
	a := newPickerApp(t, map[string]string{
		"a-work.json":     `{"model":"opus"}`,
		"b-personal.json": `{"model":"sonnet"}`,
	})
	press(a, "down", "enter")
	if a.screen != screenBrowse {
		t.Fatal("enter should open the browser")
	}
	if a.ctxIdx != 1 {
		t.Errorf("expected the second file, got %d", a.ctxIdx)
	}
	if v, _ := a.Context().Doc.Str("model"); v != "sonnet" {
		t.Errorf("the wrong file is open: %q", v)
	}
}

func TestPickerDigitOpensAFileDirectly(t *testing.T) {
	a := newPickerApp(t, map[string]string{
		"a-work.json":     `{"model":"opus"}`,
		"b-personal.json": `{"model":"sonnet"}`,
	})
	press(a, "2")
	if a.screen != screenBrowse || a.ctxIdx != 1 {
		t.Errorf("a digit should open that file, got screen %v index %d", a.screen, a.ctxIdx)
	}
}

func TestCGoesBackToThePicker(t *testing.T) {
	a := newPickerApp(t, map[string]string{
		"a-work.json":     `{"model":"opus"}`,
		"b-personal.json": `{"model":"sonnet"}`,
	})
	press(a, "enter")
	if a.screen != screenBrowse {
		t.Fatal("setup failed")
	}
	press(a, "c")
	if a.screen != screenFiles {
		t.Fatal("c should go back to the picker")
	}
	if a.files.cursor != a.ctxIdx {
		t.Error("the picker should open on the file being edited")
	}
	press(a, "down", "enter")
	if a.screen != screenBrowse || a.ctxIdx != 1 {
		t.Error("the picker should switch files")
	}
}

func TestEscFromThePickerGoesBackOnceAFileIsOpen(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	press(a, "esc")
	if !a.quitting {
		t.Error("esc on the opening picker should quit")
	}

	a = newPickerApp(t, map[string]string{"work.json": "{}"})
	press(a, "enter", "c", "esc")
	if a.screen != screenBrowse {
		t.Error("esc should go back to the file being edited")
	}
	if a.quitting {
		t.Error("esc should not quit once a file is open")
	}
}

func TestPickerQuits(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	press(a, "q")
	if !a.quitting {
		t.Error("q should quit from the picker")
	}
}

func TestPickerWithNoFiles(t *testing.T) {
	a := newPickerApp(t, nil)
	view := a.View()
	if !strings.Contains(view, "No Claude settings files found") {
		t.Errorf("the picker should say so:\n%s", view)
	}
	if !strings.Contains(view, "Scan a directory") {
		t.Error("the picker should offer a scan")
	}
	press(a, "enter") // the first row is "add"
	if a.ov == nil || a.ov.kind != ovText {
		t.Error("expected the add prompt")
	}
}

func TestPickerAddsAFileAndRemembersIt(t *testing.T) {
	store := t.TempDir()
	t.Setenv("SUGARLIME_CONFIG_DIR", store)
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	t.Setenv("SUGARLIME_CONFIG_DIR", store)

	press(a, "a")
	if a.ov == nil {
		t.Fatal("expected the add prompt")
	}
	path := filepath.Join(t.TempDir(), "team", "settings.json")
	a.ov.input.SetValue(path)
	press(a, "enter")

	if len(a.ctxs) != 2 {
		t.Fatalf("expected the file to be added, got %d", len(a.ctxs))
	}
	if a.screen != screenFiles {
		t.Error("adding a file should leave you on the picker, to choose it")
	}
	saved := config.SavedPaths()
	if len(saved) != 1 || saved[0] != path {
		t.Errorf("expected the path to be remembered, got %v", saved)
	}

	press(a, "a")
	a.ov.input.SetValue(path)
	press(a, "enter")
	if len(a.ctxs) != 2 {
		t.Error("the same file should not be added twice")
	}
	if !strings.Contains(a.status, "Already in the list") {
		t.Errorf("status = %q", a.status)
	}
}

func TestPickerForgetsAHandAddedFile(t *testing.T) {
	store := t.TempDir()
	t.Setenv("SUGARLIME_CONFIG_DIR", store)
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	t.Setenv("SUGARLIME_CONFIG_DIR", store)

	press(a, "a")
	a.ov.input.SetValue(filepath.Join(t.TempDir(), "extra.json"))
	press(a, "enter")
	if len(a.ctxs) != 2 {
		t.Fatalf("setup failed: %d files", len(a.ctxs))
	}

	a.files.cursor = 1
	press(a, "d")
	if a.ov == nil {
		t.Fatal("expected a confirmation")
	}
	a.ov.cursor = 0
	press(a, "enter")
	if len(a.ctxs) != 1 {
		t.Errorf("expected the file to be dropped, got %d", len(a.ctxs))
	}
	if len(config.SavedPaths()) != 0 {
		t.Error("the stored path should be gone")
	}
}

func TestPickerWillNotForgetADiscoveredFile(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	a.files.cursor = 0
	press(a, "d")
	if a.ov != nil {
		t.Error("a discovered file has nothing to confirm")
	}
	if !strings.Contains(a.status, "finds this file on its own") {
		t.Errorf("status = %q", a.status)
	}
	if len(a.ctxs) != 1 {
		t.Error("the file should still be listed")
	}
}

func TestPickerScanFindsSettingsFiles(t *testing.T) {
	store := t.TempDir()
	t.Setenv("SUGARLIME_CONFIG_DIR", store)
	a := newPickerApp(t, nil)
	t.Setenv("SUGARLIME_CONFIG_DIR", store)

	root := t.TempDir()
	for _, p := range []string{"api/.claude/settings.json", "web/.claude/settings.local.json"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(`{"model":"opus"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	press(a, "s")
	if a.ov == nil {
		t.Fatal("expected the scan prompt")
	}
	a.ov.input.SetValue(root)
	press(a, "enter")

	if len(a.ctxs) != 2 {
		t.Fatalf("expected both files, got %d (%s)", len(a.ctxs), a.status)
	}
	if !strings.Contains(a.status, "Added 2 settings files") {
		t.Errorf("status = %q", a.status)
	}
	if len(config.SavedPaths()) != 2 {
		t.Error("scanned files should be remembered")
	}

	press(a, "s")
	a.ov.input.SetValue(root)
	press(a, "enter")
	if !strings.Contains(a.status, "Found nothing new") {
		t.Errorf("a second scan should add nothing, status = %q", a.status)
	}
}

func TestPickerScanReportsABadDirectory(t *testing.T) {
	a := newPickerApp(t, nil)
	press(a, "s")
	a.ov.input.SetValue(filepath.Join(t.TempDir(), "nope"))
	press(a, "enter")
	if a.ov == nil || a.ov.errMsg == "" {
		t.Error("expected the scan to report the missing directory")
	}
}

func TestPickerRescan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
	a := newPickerApp(t, nil)

	press(a, "r")
	if a.status == "" {
		t.Error("a rescan should report what it found")
	}
	before := len(a.ctxs)
	press(a, "r")
	if len(a.ctxs) != before {
		t.Error("a second rescan should not duplicate anything")
	}
}

func TestPickerReportsAnUnreadableFile(t *testing.T) {
	a := newPickerApp(t, map[string]string{"broken.json": `{`})
	if !strings.Contains(a.View(), "invalid JSON") {
		t.Errorf("the picker should flag the file:\n%s", a.View())
	}
	press(a, "enter")
	if a.screen != screenFiles {
		t.Error("an unreadable file should not be opened")
	}
	if a.status == "" {
		t.Error("expected an explanation")
	}
}

func TestPickerRendersAtSmallSizes(t *testing.T) {
	a := newPickerApp(t, map[string]string{"work.json": "{}"})
	for _, size := range []struct{ w, h int }{{40, 10}, {60, 18}, {80, 24}, {200, 60}} {
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		view := a.View()
		if view == "" {
			t.Fatalf("%dx%d rendered nothing", size.w, size.h)
		}
		for i, line := range strings.Split(view, "\n") {
			if w := lineWidth(line); w > size.w {
				t.Errorf("%dx%d: line %d is %d columns wide: %q", size.w, size.h, i, w, line)
			}
		}
	}
}

func TestBrowseHeaderShowsTheLogoWhenThereIsRoom(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if !a.showLogo() {
		t.Fatal("a 120x40 terminal has room for the logo")
	}
	view := a.View()
	if !strings.Contains(view, "___") {
		t.Errorf("the header should show the logo:\n%s", view)
	}
	if !strings.Contains(view, "to change file") {
		t.Error("the header should say how to switch files")
	}

	a.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	if a.showLogo() {
		t.Error("a small terminal should fall back to the one-line header")
	}
	if !strings.Contains(a.View(), "sugarlime") {
		t.Error("the compact header should still name the program")
	}
}

func TestFooterOffersExit(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	if !strings.Contains(a.View(), "q exit") {
		t.Error("the footer should say how to quit")
	}
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter")
	if !strings.Contains(a.View(), "q exit") {
		t.Error("the list editor's footer should say how to quit too")
	}
}

func TestHelpFitsTheWindow(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "?")
	for _, size := range []struct{ w, h int }{{40, 12}, {80, 24}, {120, 40}, {200, 60}} {
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		view := a.View()
		lines := strings.Split(view, "\n")
		if len(lines) > size.h {
			t.Errorf("%dx%d: help is %d lines, window is %d", size.w, size.h, len(lines), size.h)
		}
		for i, line := range lines {
			if w := lineWidth(line); w > size.w {
				t.Errorf("%dx%d: line %d is %d columns wide", size.w, size.h, i, w)
			}
		}
	}
}

func TestHelpMentionsTheEnvironments(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	press(a, "?")
	view := a.View()
	for _, want := range []string{"CLAUDE_CONFIG_DIR", "~/.claude-work", "file picker", "scan a directory"} {
		if !strings.Contains(view, want) {
			t.Errorf("help is missing %q", want)
		}
	}
}

func TestViewLeavesTheTopLineClear(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	for _, size := range []struct{ w, h int }{{120, 40}, {100, 30}, {80, 24}, {70, 20}, {50, 14}} {
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		lines := strings.Split(a.View(), "\n")

		// A view that fills the window exactly scrolls its first line away,
		// which cut the top off the logo.
		if len(lines) >= size.h {
			t.Errorf("%dx%d: %d lines fills the window", size.w, size.h, len(lines))
		}
		if a.showLogo() && strings.TrimSpace(lines[0]) != "" {
			t.Errorf("%dx%d: the logo should start below the first line, got %q", size.w, size.h, lines[0])
		}
	}
}

func TestHeaderMakesTheOpenFileObvious(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a.ctxs[0].Name = "claude-work"

	view := a.View()
	if !strings.Contains(view, "CLAUDE-WORK") {
		t.Errorf("the open file should be the headline:\n%s", view)
	}
	if !strings.Contains(view, "editing") {
		t.Error("the header should label what the name is")
	}
	if !strings.Contains(view, "to change file") {
		t.Error("the header should say how to switch files")
	}

	// The name keeps its emphasis in the compact header too.
	a.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	if !strings.Contains(a.View(), "CLAUDE-WORK") {
		t.Error("the compact header should still name the file")
	}
}

func TestHeaderWithoutAFileSaysHowToPickOne(t *testing.T) {
	a := New(nil)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a.screen = screenBrowse
	view := a.View()
	if !strings.Contains(view, "no settings file") {
		t.Errorf("expected the header to say so:\n%s", view)
	}
	if !strings.Contains(view, "press c to pick one") {
		t.Error("expected a way out")
	}
}

func TestEscAtTheTopOfATabGoesBackToThePicker(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")

	// Inside a list, esc walks back up first.
	focus(t, a, "allow")
	press(a, "enter")
	if len(a.stack()) != 2 {
		t.Fatal("setup failed")
	}
	press(a, "esc")
	if a.screen != screenBrowse {
		t.Fatal("esc should go up a level before leaving the browser")
	}

	press(a, "esc")
	if a.screen != screenFiles {
		t.Error("esc at the top of a tab should go back to the file picker")
	}
	if a.files.cursor != a.ctxIdx {
		t.Error("the picker should open on the file being edited")
	}
}

func TestEscClearsAFilterBeforeLeaving(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	press(a, "/")
	typeText(a, "allow")
	press(a, "enter", "esc")
	if a.screen != screenBrowse {
		t.Fatal("esc should clear the filter, not leave")
	}
	if a.level().filter != "" {
		t.Error("the filter should be cleared")
	}
	press(a, "esc")
	if a.screen != screenFiles {
		t.Error("a second esc should reach the picker")
	}
}

func TestLeftArrowDoesNotLeaveTheBrowser(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	press(a, "left", "left")
	if a.screen != screenBrowse {
		t.Error("← should walk levels, not jump to the picker")
	}
}

func TestFooterKeepsTheEssentialKeys(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	for _, width := range []int{200, 120, 100, 80, 60, 44} {
		a.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		footer := a.footerKeys()
		for _, want := range []string{"c change file", "s save", "q exit", "? help"} {
			if !strings.Contains(footer, want) {
				t.Errorf("width %d: footer %q is missing %q", width, footer, want)
			}
		}
		if lineWidth(footer) > width {
			t.Errorf("width %d: footer is %d columns", width, lineWidth(footer))
		}
	}
}

func TestFooterInTheListEditorKeepsTheEssentials(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter")
	footer := a.footerKeys()
	for _, want := range []string{"a add", "c change file", "q exit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer %q is missing %q", footer, want)
		}
	}
}

func TestFitHints(t *testing.T) {
	optional := []string{"one", "two", "three"}
	always := []string{"keep"}

	if got := fitHints(optional, always, 100); got != "one · two · three · keep" {
		t.Errorf("got %q", got)
	}
	if got := fitHints(optional, always, 18); got != "one · two · keep" {
		t.Errorf("got %q", got)
	}
	if got := fitHints(optional, always, 4); got != "keep" {
		t.Errorf("a line that cannot fit keeps what matters, got %q", got)
	}
	if got := fitHints(nil, always, 50); got != "keep" {
		t.Errorf("got %q", got)
	}
}
