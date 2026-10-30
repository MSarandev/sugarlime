package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/MSarandev/sugarlime/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNavigatesTabs(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)

	if tabs[a.tabIdx].Title != "Overview" {
		t.Fatalf("expected to start on Overview, got %q", tabs[a.tabIdx].Title)
	}
	press(a, "tab")
	if tabs[a.tabIdx].Title != "Models" {
		t.Errorf("tab should move forward, got %q", tabs[a.tabIdx].Title)
	}
	press(a, "shift+tab", "shift+tab")
	if tabs[a.tabIdx].Title != "More" {
		t.Errorf("tab should wrap backwards, got %q", tabs[a.tabIdx].Title)
	}
	press(a, "3")
	if tabs[a.tabIdx].Title != "Permissions" {
		t.Errorf("a digit should jump to that tab, got %q", tabs[a.tabIdx].Title)
	}
}

func TestCursorStaysInRange(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	for i := 0; i < 200; i++ {
		press(a, "down")
	}
	l := a.level()
	if l.cursor != len(l.visible())-1 {
		t.Errorf("cursor %d, last row %d", l.cursor, len(l.visible())-1)
	}
	for i := 0; i < 200; i++ {
		press(a, "up")
	}
	if a.level().cursor != 0 {
		t.Errorf("cursor should stop at the top, got %d", a.level().cursor)
	}
	press(a, "G")
	if a.level().cursor != len(a.level().visible())-1 {
		t.Error("G should jump to the last row")
	}
	press(a, "g")
	if a.level().cursor != 0 {
		t.Error("g should jump to the first row")
	}
}

func TestDrillIntoSectionAndBack(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	focus(t, a, "Permission settings")
	press(a, "enter")
	if len(a.stack()) != 2 {
		t.Fatalf("expected to be one level deep, got %d", len(a.stack()))
	}
	if !strings.Contains(a.crumbs(), "Permission settings") {
		t.Errorf("breadcrumb = %q", a.crumbs())
	}
	press(a, "esc")
	if len(a.stack()) != 1 {
		t.Error("esc should go back a level")
	}
	press(a, "esc")
	if len(a.stack()) != 1 {
		t.Error("esc at the root should do nothing")
	}
}

func TestToggleBooleanCyclesThroughUnset(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")

	if v, _ := c.Doc.Bool("sandbox.enabled"); !v {
		t.Fatal("the sample file sets sandbox.enabled to true")
	}
	press(a, " ")
	if v, ok := c.Doc.Bool("sandbox.enabled"); !ok || v {
		t.Errorf("space should turn it false, got %v %v", v, ok)
	}
	press(a, " ")
	if c.Doc.Has("sandbox.enabled") {
		t.Error("space again should unset the key")
	}
	press(a, " ")
	if v, ok := c.Doc.Bool("sandbox.enabled"); !ok || !v {
		t.Errorf("space should set it true again, got %v %v", v, ok)
	}
}

func TestEnumPickerSetsValue(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "defaultMode")
	press(a, "enter")

	if a.ov == nil || a.ov.kind != ovChoice {
		t.Fatal("expected a choice overlay")
	}
	// The overlay opens on the current value.
	if a.ov.choices[a.ov.cursor].value != "acceptEdits" {
		t.Errorf("opened on %q", a.ov.choices[a.ov.cursor].value)
	}
	press(a, "1") // "default"
	if a.ov != nil {
		t.Error("the overlay should close after a pick")
	}
	if v, _ := c.Doc.Str("permissions.defaultMode"); v != "default" {
		t.Errorf("got %q", v)
	}
}

func TestEnumPickerCanUnset(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "defaultMode")
	press(a, "enter")

	last := len(a.ov.choices) - 1
	a.ov.cursor = last
	press(a, "enter")
	if c.Doc.Has("permissions.defaultMode") {
		t.Error("picking \"not set\" should remove the key")
	}
}

func TestEnumPickerCustomValue(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Models")
	focus(t, a, "model")
	press(a, "enter")

	for i, ch := range a.ov.choices {
		if ch.value == valueCustom {
			a.ov.cursor = i
		}
	}
	press(a, "enter")
	if a.ov == nil || a.ov.kind != ovText {
		t.Fatal("expected the text editor to replace the picker")
	}
	press(a, "ctrl+a")
	a.ov.input.SetValue("my-gateway/opus")
	press(a, "enter")
	if v, _ := c.Doc.Str("model"); v != "my-gateway/opus" {
		t.Errorf("got %q", v)
	}
}

func TestStringListAddEditDeleteReorder(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter")

	if a.level().listPath != "permissions.allow" {
		t.Fatalf("expected the list editor, got %+v", a.level())
	}

	// Add.
	press(a, "a")
	if a.ov == nil || a.ov.kind != ovText {
		t.Fatal("expected a text overlay")
	}
	a.ov.input.SetValue("Bash(npm test)")
	press(a, "enter")
	vals, _ := c.Doc.Strings("permissions.allow")
	if len(vals) != 3 || vals[2] != "Bash(npm test)" {
		t.Fatalf("got %v", vals)
	}

	// A duplicate is refused and the overlay stays open.
	press(a, "a")
	a.ov.input.SetValue("Bash(npm test)")
	press(a, "enter")
	if a.ov == nil {
		t.Fatal("the overlay should stay open after a rejected entry")
	}
	if a.ov.errMsg == "" {
		t.Error("expected an error message about the duplicate")
	}
	press(a, "esc")

	// An invalid rule is refused.
	press(a, "a")
	a.ov.input.SetValue("Bash(")
	press(a, "enter")
	if a.ov == nil || a.ov.errMsg == "" {
		t.Fatal("expected the invalid rule to be rejected")
	}
	press(a, "esc")

	// Edit the first entry.
	a.level().cursor = 0
	press(a, "enter")
	a.ov.input.SetValue("Bash(git status)")
	press(a, "enter")
	vals, _ = c.Doc.Strings("permissions.allow")
	if vals[0] != "Bash(git status)" {
		t.Fatalf("got %v", vals)
	}

	// Reorder.
	press(a, "J")
	vals, _ = c.Doc.Strings("permissions.allow")
	if vals[1] != "Bash(git status)" {
		t.Fatalf("J should move the entry down: %v", vals)
	}
	press(a, "K")
	vals, _ = c.Doc.Strings("permissions.allow")
	if vals[0] != "Bash(git status)" {
		t.Fatalf("K should move it back: %v", vals)
	}

	// Delete.
	before := len(vals)
	press(a, "d")
	vals, _ = c.Doc.Strings("permissions.allow")
	if len(vals) != before-1 {
		t.Fatalf("expected one fewer entry, got %v", vals)
	}
}

func TestDeletingTheLastListEntryRemovesTheKey(t *testing.T) {
	a, c := newTestApp(t, `{"permissions":{"deny":["Read(./.env)"]}}`)
	gotoTab(t, a, "Permissions")
	focus(t, a, "deny")
	press(a, "enter", "d")
	if c.Doc.Has("permissions.deny") {
		t.Error("an emptied list should remove the key")
	}
	if c.Doc.Has("permissions") {
		t.Error("the emptied parent object should be pruned")
	}
}

func TestUnsetAsksFirst(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Models")
	focus(t, a, "model")

	press(a, "x")
	if a.ov == nil || a.ov.kind != ovChoice {
		t.Fatal("expected a confirmation")
	}
	a.ov.cursor = 1 // "Keep it"
	press(a, "enter")
	if !c.Doc.Has("model") {
		t.Fatal("declining should leave the key alone")
	}

	press(a, "x")
	a.ov.cursor = 0 // "Remove the key"
	press(a, "enter")
	if c.Doc.Has("model") {
		t.Error("confirming should remove the key")
	}
}

func TestUnsetOnAnUnsetKeySaysSo(t *testing.T) {
	a, _ := newTestApp(t, `{}`)
	gotoTab(t, a, "Models")
	focus(t, a, "model")
	press(a, "x")
	if a.ov != nil {
		t.Error("there is nothing to confirm")
	}
	if !strings.Contains(a.status, "not set") {
		t.Errorf("status = %q", a.status)
	}
}

func TestNumberEditorValidates(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "network")
	press(a, "enter")
	focus(t, a, "httpProxyPort")
	press(a, "enter")

	a.ov.input.SetValue("not a number")
	press(a, "enter")
	if a.ov == nil || a.ov.errMsg == "" {
		t.Fatal("expected the editor to refuse the value")
	}
	a.ov.input.SetValue("8080")
	press(a, "enter")
	if n, _ := c.Doc.Num("sandbox.network.httpProxyPort"); n.String() != "8080" {
		t.Errorf("got %v", n)
	}

	// An empty value unsets the key.
	press(a, "enter")
	a.ov.input.SetValue("")
	press(a, "enter")
	if c.Doc.Has("sandbox.network.httpProxyPort") {
		t.Error("an empty number should unset the key")
	}
}

func TestJSONEditorAppliesAndRejects(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Hooks")
	focus(t, a, "hooks")
	press(a, "enter")

	if a.ov == nil || a.ov.kind != ovJSON {
		t.Fatal("expected the JSON editor")
	}
	a.ov.area.SetValue("{not json}")
	press(a, "ctrl+s")
	if a.ov == nil || a.ov.errMsg == "" {
		t.Fatal("expected invalid JSON to be refused")
	}
	a.ov.area.SetValue(`{"PreToolUse":[{"matcher":"Bash"}]}`)
	press(a, "ctrl+s")
	if a.ov != nil {
		t.Fatal("the editor should close once the JSON parses")
	}
	if !c.Doc.Has("hooks.PreToolUse") {
		t.Error("expected the hook to be written")
	}
}

func TestMapEditorAddsAndDeletesPairs(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "More")
	focus(t, a, "Memory and context")
	press(a, "enter")
	focus(t, a, "env")
	press(a, "enter")

	if a.level().mapPath != "env" {
		t.Fatalf("expected the pair editor, got %+v", a.level())
	}
	if len(a.level().rows) != 1 {
		t.Fatalf("expected the one pair from the sample, got %d", len(a.level().rows))
	}

	press(a, "a")
	a.ov.input.SetValue("missing-equals")
	press(a, "enter")
	if a.ov == nil || a.ov.errMsg == "" {
		t.Fatal("expected name=value to be required")
	}
	a.ov.input.SetValue("PAGER=less")
	press(a, "enter")
	if v, _ := c.Doc.Str("env.PAGER"); v != "less" {
		t.Errorf("got %q", v)
	}

	focus(t, a, "EDITOR")
	press(a, "enter")
	a.ov.input.SetValue("nvim")
	press(a, "enter")
	if v, _ := c.Doc.Str("env.EDITOR"); v != "nvim" {
		t.Errorf("got %q", v)
	}

	press(a, "d")
	if c.Doc.Has("env.EDITOR") {
		t.Error("expected the pair to be deleted")
	}
}

func TestFilterNarrowsTheList(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Interface")
	before := len(a.level().visible())

	press(a, "/")
	typeText(a, "theme")
	if len(a.level().visible()) >= before {
		t.Error("the filter should narrow the list as you type")
	}
	press(a, "enter")
	press(a, "esc")
	if a.level().filter != "" {
		t.Error("esc should clear the filter")
	}
	if len(a.level().visible()) != before {
		t.Error("clearing the filter should restore the list")
	}
}

func TestFilterWithNoMatchesSaysSo(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Interface")
	press(a, "/")
	typeText(a, "zzzznothing")
	press(a, "enter")
	if !strings.Contains(a.status, "Nothing matches") {
		t.Errorf("status = %q", a.status)
	}
	if !strings.Contains(a.View(), "Nothing matches") {
		t.Error("the empty list should explain itself")
	}
}

func TestFindJumpsToAnyKey(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "f")
	typeText(a, "denyRead")
	if len(a.ov.results) == 0 {
		t.Fatal("expected live results")
	}
	press(a, "enter")

	r, ok := a.level().current()
	if !ok || r.field.Path != "sandbox.filesystem.denyRead" {
		t.Fatalf("landed on %+v", r)
	}
	if tabs[a.tabIdx].Title != "Sandbox" {
		t.Errorf("expected the Sandbox tab, got %q", tabs[a.tabIdx].Title)
	}
	if !strings.Contains(a.crumbs(), "filesystem") {
		t.Errorf("breadcrumb = %q", a.crumbs())
	}
}

func TestFindReportsNoMatch(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "f")
	a.ov.input.SetValue("zzzznothing")
	press(a, "enter")
	if a.ov == nil || a.ov.errMsg == "" {
		t.Error("expected the search to report that nothing matched")
	}
}

func TestFindInATabWithGroupedCategories(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "f")
	typeText(a, "cleanupPeriodDays")
	press(a, "enter")
	r, ok := a.level().current()
	if !ok || r.field.Path != "cleanupPeriodDays" {
		t.Fatalf("landed on %+v", r)
	}
}

func TestSaveWritesTheFile(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")
	press(a, " ")

	if !a.Dirty() {
		t.Fatal("the edit should mark the app dirty")
	}
	if !strings.Contains(a.View(), "unsaved") {
		t.Error("the header should show unsaved work")
	}

	press(a, "s")
	if a.Dirty() {
		t.Fatal("saving should clear the dirty state")
	}
	data, err := os.ReadFile(c.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"enabled": false`) {
		t.Errorf("the file was not updated:\n%s", data)
	}
	if !strings.Contains(string(data), "someKeyWeDoNotKnow") {
		t.Error("saving must keep keys sugarlime does not know")
	}
	if !strings.Contains(a.status, "Saved") {
		t.Errorf("status = %q", a.status)
	}
}

func TestSaveWithNothingToDo(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "s")
	if !strings.Contains(a.status, "Nothing to save") {
		t.Errorf("status = %q", a.status)
	}
}

func TestRevertAsksAndRestores(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")
	press(a, " ")

	press(a, "r")
	if a.ov == nil {
		t.Fatal("expected a confirmation")
	}
	a.ov.cursor = 1
	press(a, "enter")
	if !a.Dirty() {
		t.Error("declining should keep the edits")
	}

	press(a, "r")
	a.ov.cursor = 0
	press(a, "enter")
	if a.Dirty() {
		t.Error("confirming should discard the edits")
	}
	if v, _ := c.Doc.Bool("sandbox.enabled"); !v {
		t.Error("the original value should be back")
	}
}

func TestReloadRereadsTheFile(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	if err := os.WriteFile(c.Path, []byte(`{"model":"opus"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	press(a, "R")
	if v, _ := a.Context().Doc.Str("model"); v != "opus" {
		t.Errorf("got %q", v)
	}
}

func TestReadOnlyContextRefusesEdits(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.ctxs[0].ReadOnly = true
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")

	press(a, " ")
	if v, _ := a.Context().Doc.Bool("sandbox.enabled"); !v {
		t.Error("a read-only file must not change")
	}
	if !strings.Contains(a.status, "read-only") {
		t.Errorf("status = %q", a.status)
	}
	press(a, "s")
	if !strings.Contains(a.status, "read-only") {
		t.Errorf("status = %q", a.status)
	}
}

func TestInvalidFileIsReportedNotEdited(t *testing.T) {
	a, _ := newTestApp(t, `{"model": }`)
	if a.Context().LoadErr == nil {
		t.Fatal("expected a load error")
	}
	if !strings.Contains(a.View(), "invalid JSON") {
		t.Error("the header should flag the unreadable file")
	}
	gotoTab(t, a, "Models")
	focus(t, a, "model")
	press(a, "enter")
	if a.ov != nil {
		t.Error("an unreadable file must not open an editor")
	}
}

func TestQuitAsksWhenDirty(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")
	press(a, " ")

	press(a, "q")
	if a.ov == nil {
		t.Fatal("expected a confirmation before quitting")
	}
	a.ov.cursor = 2 // keep editing
	press(a, "enter")
	if a.quitting {
		t.Error("the app should still be running")
	}

	press(a, "q")
	a.ov.cursor = 0 // save and quit
	press(a, "enter")
	if !a.quitting {
		t.Error("expected the app to quit")
	}
	data, _ := os.ReadFile(c.Path)
	if !strings.Contains(string(data), `"enabled": false`) {
		t.Error("save and quit should write the file")
	}
}

func TestQuitDiscardsOnRequest(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")
	press(a, " ")
	press(a, "q")
	a.ov.cursor = 1 // quit without saving
	press(a, "enter")
	if !a.quitting {
		t.Error("expected the app to quit")
	}
	data, _ := os.ReadFile(c.Path)
	if !strings.Contains(string(data), `"enabled": true`) {
		t.Error("the file should be untouched")
	}
	if !strings.Contains(a.QuitMessage(), "unsaved") {
		t.Errorf("quit message = %q", a.QuitMessage())
	}
}

func TestQuitImmediatelyWhenClean(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "q")
	if !a.quitting {
		t.Error("a clean app should quit at once")
	}
	if a.QuitMessage() != "" {
		t.Errorf("quit message = %q", a.QuitMessage())
	}
	if a.View() != "" {
		t.Error("a quitting app renders nothing")
	}
}

func TestHelpOpensAndCloses(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "?")
	view := a.View()
	if !strings.Contains(view, "Moving around") || !strings.Contains(view, "cycle a Boolean") {
		t.Error("the help screen should list the keys")
	}
	press(a, "?")
	if a.showHelp {
		t.Error("? should close the help screen")
	}
}

func TestUnknownKeysAreVisibleAndEditable(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	focus(t, a, "Keys not in the reference")
	press(a, "enter")

	r, ok := a.level().current()
	if !ok || r.field.Path != "someKeyWeDoNotKnow" {
		t.Fatalf("got %+v", r)
	}
	if !strings.Contains(a.View(), "someKeyWeDoNotKnow") {
		t.Error("the key should be listed")
	}
	press(a, "enter")
	if a.ov == nil || a.ov.kind != ovJSON {
		t.Fatal("an unknown key opens the JSON editor")
	}
	a.ov.area.SetValue("43")
	press(a, "ctrl+s")
	if n, _ := c.Doc.Num("someKeyWeDoNotKnow"); n.String() != "43" {
		t.Errorf("got %v", n)
	}
}

func TestNoContextsInTheBrowser(t *testing.T) {
	a := New(nil)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.screen = screenBrowse
	if a.Context() != nil {
		t.Error("expected no active file")
	}
	if !strings.Contains(a.View(), "no settings file") {
		t.Errorf("the header should say so:\n%s", a.View())
	}
	press(a, "enter", " ", "x", "s", "r", "R", "d")
	if a.quitting {
		t.Error("none of those keys should quit")
	}
}

func TestGlobalConfigContextShowsOnlyItsKeys(t *testing.T) {
	a, _ := newTestApp(t, `{"autoConnectIde":true}`)
	a.ctxs[0].Kind = config.KindGlobal
	a.stacks = map[int][]level{}

	for _, r := range a.level().visible() {
		if r.title == "Model and responses" {
			t.Error("~/.claude.json should not offer settings-file categories")
		}
	}
	gotoTab(t, a, "More")
	found := false
	for _, r := range a.level().visible() {
		if r.title == "Global config settings" {
			found = true
		}
	}
	if !found {
		t.Error("expected the global config category")
	}
}

func TestSettingsFileHidesGlobalConfigKeys(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "More")
	for _, r := range a.level().visible() {
		if r.title == "Global config settings" {
			t.Error("a settings file should not offer ~/.claude.json keys")
		}
	}
}

func TestOtherFilesThatSetTheSameKeyAreShown(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	other := config.Context{Name: "project", Path: a.ctxs[0].Path + ".project", Kind: config.KindProject}
	_ = other.Load()
	_ = other.Doc.Set("model", "claude-opus-5-5")
	a.ctxs = append(a.ctxs, other)

	gotoTab(t, a, "Models")
	focus(t, a, "model")
	if !strings.Contains(a.View(), "Also set in") {
		t.Error("the detail pane should mention the other file")
	}
}

func TestRendersAtSmallSizes(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	for _, size := range []struct{ w, h int }{{40, 12}, {60, 20}, {80, 24}, {200, 60}} {
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		gotoTab(t, a, "Permissions")
		view := a.View()
		if view == "" {
			t.Fatalf("%dx%d rendered nothing", size.w, size.h)
		}
		for i, line := range strings.Split(view, "\n") {
			if w := lineWidth(line); w > size.w {
				t.Errorf("%dx%d: line %d is %d columns wide", size.w, size.h, i, w)
			}
		}
	}
}

func TestOverlayRendersAtSmallSizes(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "defaultMode")
	press(a, "enter")
	for _, size := range []struct{ w, h int }{{40, 12}, {80, 24}, {160, 50}} {
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		for i, line := range strings.Split(a.View(), "\n") {
			if w := lineWidth(line); w > size.w {
				t.Errorf("%dx%d: line %d is %d columns wide", size.w, size.h, i, w)
			}
		}
	}
}

func TestTabCompletionInRuleEditor(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter", "a")
	typeText(a, "WebS")
	press(a, "tab")
	if got := a.ov.input.Value(); got != "WebSearch" {
		t.Errorf("got %q", got)
	}
}

func TestStringEditorSetsAndUnsets(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	press(a, "f")
	typeText(a, "apiKeyHelper")
	press(a, "enter")

	press(a, "enter")
	if a.ov == nil || a.ov.kind != ovText {
		t.Fatal("expected a text editor")
	}
	a.ov.input.SetValue("/usr/local/bin/get-key")
	press(a, "enter")
	if v, _ := c.Doc.Str("apiKeyHelper"); v != "/usr/local/bin/get-key" {
		t.Errorf("got %q", v)
	}

	press(a, "enter")
	a.ov.input.SetValue("")
	press(a, "enter")
	if c.Doc.Has("apiKeyHelper") {
		t.Error("an empty value should unset the key")
	}
}

func TestListDetailShowsWarningsAndPrefixes(t *testing.T) {
	a, _ := newTestApp(t, `{"sandbox":{"filesystem":{"denyRead":["~/.aws/"]},"network":{"allowedDomains":["*"]}}}`)
	press(a, "f")
	typeText(a, "denyRead")
	press(a, "enter", "enter")

	view := a.View()
	if !strings.Contains(view, "trailing slash") {
		t.Error("the detail pane should warn about the trailing slash")
	}
	if !strings.Contains(view, "relative to your home directory") {
		t.Error("the detail pane should document the path prefixes")
	}
	if !strings.Contains(view, "!") {
		t.Error("the row should be flagged")
	}

	press(a, "esc")
	press(a, "f")
	typeText(a, "allowedDomains")
	press(a, "enter", "enter")
	if !strings.Contains(a.View(), "network isolation") {
		t.Error("the detail pane should warn about the catch-all domain")
	}
}

func TestListDetailFlagsAnInvalidEntry(t *testing.T) {
	a, _ := newTestApp(t, `{"permissions":{"allow":["Bsah(ls)"]}}`)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter")
	view := a.View()
	if !strings.Contains(view, "not a built-in tool name") {
		t.Errorf("expected advice about the typo:\n%s", view)
	}
}

func TestMapEntryDetail(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "f")
	typeText(a, "env")
	press(a, "enter", "enter")
	if a.level().mapPath != "env" {
		t.Fatalf("expected the pair editor, got %+v", a.level())
	}
	view := a.View()
	if !strings.Contains(view, "EDITOR") || !strings.Contains(view, "in env") {
		t.Errorf("the detail pane should describe the pair:\n%s", view)
	}
}

func TestEmptyMapEditorExplainsItself(t *testing.T) {
	a, _ := newTestApp(t, `{}`)
	press(a, "f")
	typeText(a, "modelOverrides")
	press(a, "enter", "enter")
	if a.level().mapPath != "modelOverrides" {
		t.Fatalf("expected the pair editor, got %+v", a.level())
	}
	if !strings.Contains(a.View(), "No keys yet") {
		t.Error("an empty pair editor should say how to add one")
	}
}

func TestEmptyListEditorExplainsItself(t *testing.T) {
	a, _ := newTestApp(t, `{}`)
	gotoTab(t, a, "Permissions")
	focus(t, a, "ask")
	press(a, "enter")
	if !strings.Contains(a.View(), "Press a to add an entry") {
		t.Error("an empty list should say how to add an entry")
	}
}

func TestListWithANonStringValueOpensTheJSONEditor(t *testing.T) {
	a, _ := newTestApp(t, `{"permissions":{"allow":[1,2]}}`)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter")
	if a.ov == nil || a.ov.kind != ovJSON {
		t.Fatal("a list sugarlime cannot edit entry by entry opens as JSON")
	}
}

func TestReorderAtTheEndsDoesNothing(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter")

	before, _ := c.Doc.Strings("permissions.allow")
	press(a, "K") // already first
	after, _ := c.Doc.Strings("permissions.allow")
	if after[0] != before[0] {
		t.Error("moving the first entry up should do nothing")
	}
	press(a, "G", "J") // already last
	after, _ = c.Doc.Strings("permissions.allow")
	if after[len(after)-1] != before[len(before)-1] {
		t.Error("moving the last entry down should do nothing")
	}
}

func TestReorderIsDisabledWhileFiltering(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "allow")
	press(a, "enter", "/")
	typeText(a, "zsh")
	press(a, "enter")

	before, _ := c.Doc.Strings("permissions.allow")
	press(a, "J")
	after, _ := c.Doc.Strings("permissions.allow")
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("reordering a filtered list would move the wrong entry")
		}
	}
}

func TestAddOutsideAListSaysWhatToDo(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")
	press(a, "a")
	if !strings.Contains(a.status, "Nothing to add here") {
		t.Errorf("status = %q", a.status)
	}
}

func TestAddOnAListKeyOpensItFirst(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "ask")
	press(a, "a")
	if a.level().listPath != "permissions.ask" {
		t.Fatalf("expected the list to open, got %+v", a.level())
	}
	if a.ov == nil {
		t.Fatal("expected the add prompt")
	}
	a.ov.input.SetValue("Bash(git push *)")
	press(a, "enter")
	if !strings.Contains(a.status, "Added") {
		t.Errorf("status = %q", a.status)
	}
}

func TestAddReportsAWarningAlongsideTheChange(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "deny")
	press(a, "enter", "a")
	a.ov.input.SetValue("Bsah(ls)")
	press(a, "enter")
	if !strings.Contains(a.status, "not a built-in tool name") {
		t.Errorf("status = %q", a.status)
	}
}

func TestPageKeysMoveTheCursor(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Interface")
	press(a, "pgdown")
	if a.level().cursor == 0 {
		t.Error("pgdown should move down a screen")
	}
	press(a, "pgup")
	if a.level().cursor != 0 {
		t.Error("pgup should return to the top")
	}
	press(a, "home")
	if a.level().cursor != 0 {
		t.Error("home should go to the first row")
	}
	press(a, "end")
	if a.level().cursor != len(a.level().visible())-1 {
		t.Error("end should go to the last row")
	}
}

func TestBracketKeysSwitchTabs(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "]")
	if tabs[a.tabIdx].Title != "Models" {
		t.Errorf("got %q", tabs[a.tabIdx].Title)
	}
	press(a, "[")
	if tabs[a.tabIdx].Title != "Overview" {
		t.Errorf("got %q", tabs[a.tabIdx].Title)
	}
}

func TestOverlayNavigationKeys(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	focus(t, a, "defaultMode")
	press(a, "enter")

	press(a, "end")
	if a.ov.cursor != len(a.ov.choices)-1 {
		t.Error("end should move to the last choice")
	}
	press(a, "home")
	if a.ov.cursor != 0 {
		t.Error("home should move to the first choice")
	}
	press(a, "down", "down", "up")
	if a.ov.cursor != 1 {
		t.Errorf("cursor = %d", a.ov.cursor)
	}
	press(a, "esc")
	if a.ov != nil {
		t.Error("esc should close the picker")
	}
}

func TestJSONEditorCanBeCancelled(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Hooks")
	focus(t, a, "hooks")
	press(a, "enter")
	a.ov.area.SetValue(`{"PreToolUse":[]}`)
	press(a, "esc")
	if a.ov != nil {
		t.Error("esc should close the editor")
	}
	if c.Doc.Has("hooks") {
		t.Error("cancelling must not write anything")
	}
}

func TestJSONEditorEmptyValueUnsets(t *testing.T) {
	a, c := newTestApp(t, `{"hooks":{"PreToolUse":[]}}`)
	gotoTab(t, a, "Hooks")
	focus(t, a, "hooks")
	press(a, "enter")
	a.ov.area.SetValue("")
	press(a, "ctrl+s")
	if c.Doc.Has("hooks") {
		t.Error("an empty JSON editor should unset the key")
	}
}

func TestJSONEditorTakesTyping(t *testing.T) {
	a, c := newTestApp(t, `{}`)
	gotoTab(t, a, "Hooks")
	focus(t, a, "hooks")
	press(a, "enter")
	a.ov.area.SetValue("")
	typeText(a, `{"PreToolUse":[]}`)
	press(a, "ctrl+s")
	if !c.Doc.Has("hooks") {
		t.Errorf("expected the typed JSON to be applied; status %q", a.status)
	}
}

func TestStatusClearsOnTheNextKey(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "s") // sets "Nothing to save."
	if a.status == "" {
		t.Fatal("expected a status message")
	}
	press(a, "down")
	if a.status != "" {
		t.Errorf("the message should clear on the next key, got %q", a.status)
	}
}

func TestDeprecatedKeysAreMarkedInTheDetailPane(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "f")
	typeText(a, "includeCoAuthoredBy")
	press(a, "enter")
	view := a.View()
	if !strings.Contains(view, "deprecated") {
		t.Errorf("expected the deprecation marker:\n%s", view)
	}
}

func TestManagedOnlyKeyIsExplained(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	press(a, "f")
	typeText(a, "allowManagedPermissionRulesOnly")
	press(a, "enter")
	if !strings.Contains(a.View(), "only in managed settings") {
		t.Error("the detail pane should say the key needs managed settings")
	}
}

func TestGlobalConfigKeyInTheWrongFileIsStillReachable(t *testing.T) {
	a, _ := newTestApp(t, `{"autoConnectIde":true}`)
	focus(t, a, "Global config settings")
	press(a, "enter")
	focus(t, a, "autoConnectIde")
	view := a.View()
	if !strings.Contains(view, "reads this key from ~/.claude.json") {
		t.Errorf("the detail pane should say the key is in the wrong file:\n%s", view)
	}
}

func TestDetailPaneWithoutARow(t *testing.T) {
	a, _ := newTestApp(t, `{}`)
	a.stacks[0] = []level{{title: "empty"}}
	view := a.View()
	if !strings.Contains(view, "test") {
		t.Errorf("the detail pane should fall back to describing the file:\n%s", view)
	}
}

func TestSectionDetailListsTheKeysOnOffer(t *testing.T) {
	a, _ := newTestApp(t, `{}`)
	focus(t, a, "Hooks and automation")
	view := a.View()
	if !strings.Contains(view, "These are the keys it offers") {
		t.Errorf("expected the key list:\n%s", view)
	}
}

func TestSectionDetailListsWhatIsSet(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	focus(t, a, "Permission settings")
	if !strings.Contains(a.View(), "permissions.allow") {
		t.Error("the detail pane should list the keys this file sets")
	}
}

func TestNarrowTerminalHidesTheDetailPane(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	if a.showDetail() {
		t.Error("50 columns is too narrow for two panes")
	}
	if strings.Contains(a.View(), "│") {
		t.Error("the divider should be gone with one pane")
	}
}

func TestVeryNarrowTerminalShowsOneTab(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.Update(tea.WindowSizeMsg{Width: 24, Height: 10})
	view := strings.Split(a.View(), "\n")[1]
	if !strings.Contains(view, "/9") {
		t.Errorf("expected a compact tab indicator, got %q", view)
	}
}

func TestLongListScrolls(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
	gotoTab(t, a, "Interface")
	focus(t, a, "Interface and terminal")
	press(a, "enter", "G")
	view := a.View()
	if !strings.Contains(view, "more") && a.level().offset == 0 {
		t.Error("a long list should scroll with the cursor")
	}
	if a.level().offset == 0 {
		t.Error("the window should have moved down")
	}
}

func TestUnknownNestedKeyIsListed(t *testing.T) {
	a, _ := newTestApp(t, `{"permissions":{"somethingNew":true}}`)
	gotoTab(t, a, "Permissions")
	found := false
	for _, r := range a.level().visible() {
		if r.field.Path == "permissions.somethingNew" {
			found = true
		}
	}
	if !found {
		t.Errorf("an unknown nested key should be shown: %v", labels(a.level()))
	}
}

func TestObjectWithFewChildrenStaysADrillIn(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Permissions")
	r := focus(t, a, "autoMode")
	if !r.drills() {
		t.Error("autoMode has one child, so it stays a group")
	}
	press(a, "enter")
	if len(a.stack()) != 2 {
		t.Fatal("expected to drill in")
	}
	if got := labels(a.level()); len(got) != 1 || got[0] != "classifyAllShell" {
		t.Errorf("got %v", got)
	}
}

func TestWindowResizeKeepsThingsWorking(t *testing.T) {
	a, c := newTestApp(t, sampleSettings)
	gotoTab(t, a, "Sandbox")
	focus(t, a, "enabled")
	a.Update(tea.WindowSizeMsg{Width: 70, Height: 15})
	press(a, " ")
	if v, ok := c.Doc.Bool("sandbox.enabled"); !ok || v {
		t.Error("editing should still work after a resize")
	}
}
