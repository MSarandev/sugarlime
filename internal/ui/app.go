// Package ui is sugarlime's terminal interface: a tabbed browser over every
// Claude settings key, with an editor for each kind of value.
package ui

import (
	"fmt"
	"strings"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/schema"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// screen is which of the two top-level views is showing.
type screen int

const (
	screenFiles screen = iota // the picker sugarlime opens on
	screenBrowse
)

type statusKind int

const (
	statusInfo statusKind = iota
	statusGood
	statusBad
)

// App is the root bubbletea model.
type App struct {
	ctxs    []config.Context
	ctxIdx  int
	workdir string

	screen  screen
	files   filePicker
	entered bool // true once a file has been opened, so esc can go back

	tabIdx int
	stacks map[int][]level

	ov *overlay

	status     string
	statusKind statusKind

	showHelp bool
	quitting bool

	detail viewport.Model

	width  int
	height int
}

// New builds the app over the discovered settings files. It opens on the file
// picker, so which file is about to be edited is always a deliberate choice.
func New(ctxs []config.Context) *App {
	a := &App{
		ctxs:   ctxs,
		stacks: map[int][]level{},
		detail: viewport.New(40, 10),
		width:  80,
		height: 24,
	}
	a.refreshFiles()
	return a
}

// WithWorkdir records the directory project settings were looked for in, so a
// rescan looks in the same place.
func (a *App) WithWorkdir(dir string) *App {
	a.workdir = dir
	return a
}

func (a *App) Init() tea.Cmd { return nil }

// Context returns the settings file being edited, or nil when there is none.
func (a *App) Context() *config.Context {
	if a.ctxIdx < 0 || a.ctxIdx >= len(a.ctxs) {
		return nil
	}
	return &a.ctxs[a.ctxIdx]
}

func (a *App) doc() *jsondoc.Object {
	c := a.Context()
	if c == nil || c.Doc == nil {
		return jsondoc.New()
	}
	return c.Doc
}

func (a *App) setStatus(msg string, kind statusKind) {
	a.status, a.statusKind = msg, kind
}

// Dirty reports whether any loaded file has unsaved edits.
func (a *App) Dirty() bool {
	for i := range a.ctxs {
		if a.ctxs[i].Dirty() {
			return true
		}
	}
	return false
}

// stack returns the active tab's navigation stack, building its root level lazily.
func (a *App) stack() []level {
	s, ok := a.stacks[a.tabIdx]
	if !ok || len(s) == 0 {
		s = []level{a.rootLevel(a.tabIdx)}
		a.stacks[a.tabIdx] = s
	}
	return s
}

func (a *App) level() *level {
	s := a.stack()
	return &a.stacks[a.tabIdx][len(s)-1]
}

func (a *App) push(l level) {
	a.stacks[a.tabIdx] = append(a.stack(), l)
}

func (a *App) pop() bool {
	s := a.stack()
	if len(s) <= 1 {
		return false
	}
	a.stacks[a.tabIdx] = s[:len(s)-1]
	return true
}

// rebuild refreshes cached rows after an edit, keeping the cursor in place.
func (a *App) rebuild() {
	for idx := range a.stacks {
		stack := a.stacks[idx]
		for i := range stack {
			l := &stack[i]
			switch {
			case l.listPath != "":
				vals, _ := a.doc().Strings(l.listPath)
				l.rows = listRows(l.listPath, vals)
			case l.mapPath != "":
				l.rows = mapRows(a.doc(), l.mapPath)
			case i == 0:
				l.rows = a.rootLevel(idx).rows
			}
			if l.cursor >= len(l.visible()) {
				l.cursor = max(0, len(l.visible())-1)
			}
		}
	}
}

// rootLevel builds the first screen of a tab.
func (a *App) rootLevel(tabIdx int) level {
	t := tabs[tabIdx]
	if tabIdx == 0 {
		return a.overviewLevel()
	}
	if len(t.Cats) == 1 {
		return level{title: t.Title, rows: a.categoryRows(t.Cats[0])}
	}
	var rows []row
	for _, c := range t.Cats {
		if !a.categoryApplies(c) {
			continue
		}
		set, total := setCount(a.doc(), c)
		rows = append(rows, sectionRow(c, []string{c}, countNote(set, total)))
	}
	return level{title: t.Title, rows: rows}
}

// categoryRows lists a category's keys, flattening an object such as
// permissions into its own keys so the list opens on what people came to change.
func (a *App) categoryRows(category string) []row {
	var out []row
	flattened := 0
	for _, f := range schema.Roots(category) {
		if f.Kind == schema.KindObject && len(schema.Children(f.Path)) >= 3 {
			flattened++
			for _, r := range a.objectRows(f) {
				out = append(out, a.fieldRow(r.field, r.field.Name))
			}
			continue
		}
		out = append(out, a.fieldRow(f, f.Name))
	}
	if flattened > 1 || hasDuplicateLabels(out) {
		for i := range out {
			out[i].title = out[i].field.Path
		}
	}
	return a.trimRows(out)
}

// hasDuplicateLabels spots the collision two flattened objects would cause.
func hasDuplicateLabels(rows []row) bool {
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.title] {
			return true
		}
		seen[r.title] = true
	}
	return false
}

// fieldRow builds a row for one key, labelled as given.
func (a *App) fieldRow(f schema.Field, label string) row {
	return row{kind: rowField, field: f, title: label}
}

// trimRows drops keys the open file cannot hold, unless it already sets them.
func (a *App) trimRows(rows []row) []row {
	c := a.Context()
	if c == nil {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		if r.kind == rowField && !c.Accepts(r.field) && !a.doc().Has(r.field.Path) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// categoryApplies keeps a category visible when the file sets one of its keys,
// even one Claude Code would ignore there.
func (a *App) categoryApplies(category string) bool {
	c := a.Context()
	if c == nil {
		return true
	}
	global := category == "Global config settings"
	if global == (c.Kind == config.KindGlobal) {
		return true
	}
	set, _ := setCount(a.doc(), category)
	return set > 0
}

func (a *App) overviewLevel() level {
	doc := a.doc()

	var rows []row
	if n := len(unknownFields(doc)); n > 0 {
		rows = append(rows, row{
			kind:  rowSection,
			title: "Keys not in the reference",
			paths: []string{"__unknown__"},
			note:  fmt.Sprintf("%d in this file", n),
		})
	}
	for _, c := range schema.Categories() {
		if !a.categoryApplies(c) {
			continue
		}
		set, total := setCount(doc, c)
		rows = append(rows, sectionRow(c, []string{c}, countNote(set, total)))
	}
	return level{title: "Overview", rows: rows}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.layout()
		return a, nil
	case tea.KeyMsg:
		return a, a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	a.status = "" // a handler below may set a new one

	if a.ov != nil {
		return a.overlayKey(msg)
	}
	if a.showHelp {
		switch msg.String() {
		case "?", "esc", "q", "enter", "ctrl+c":
			a.showHelp = false
		}
		return nil
	}
	if a.screen == screenFiles {
		return a.fileKey(msg)
	}
	return a.browseKey(msg)
}

func (a *App) browseKey(msg tea.KeyMsg) tea.Cmd {
	l := a.level()

	switch msg.String() {
	case "ctrl+c":
		return a.quit()
	case "q":
		return a.quit()
	case "?":
		a.showHelp = true
	case "tab", "]":
		a.tabIdx = (a.tabIdx + 1) % len(tabs)
	case "shift+tab", "[":
		a.tabIdx = (a.tabIdx - 1 + len(tabs)) % len(tabs)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if n := int(msg.String()[0] - '1'); n < len(tabs) {
			a.tabIdx = n
		}
	case "up", "k":
		a.moveCursor(-1)
	case "down", "j":
		a.moveCursor(1)
	case "pgup":
		a.moveCursor(-a.listHeight())
	case "pgdown":
		a.moveCursor(a.listHeight())
	case "home", "g":
		l.cursor = 0
	case "end", "G":
		l.cursor = max(0, len(l.visible())-1)
	case "left", "h", "esc":
		if l.filter != "" {
			l.filter = ""
			l.cursor = 0
			return nil
		}
		if !a.pop() && msg.String() == "esc" {
			a.toPicker() // esc at the top of a tab goes back to the file list
		}
	case "right", "l", "enter":
		return a.openCurrent()
	case " ":
		return a.toggleCurrent()
	case "x", "delete":
		return a.unsetCurrent()
	case "a", "+":
		return a.addEntry()
	case "d":
		return a.deleteEntry()
	case "J":
		return a.moveEntry(1)
	case "K":
		return a.moveEntry(-1)
	case "/":
		a.openFilter()
	case "f":
		a.openSearch()
	case "c":
		a.toPicker()
	case "s", "ctrl+s":
		return a.save()
	case "r":
		return a.revert()
	case "R":
		return a.reload()
	}
	return nil
}

// toPicker shows the file list again, with the open file under the cursor.
func (a *App) toPicker() {
	a.screen = screenFiles
	a.refreshFiles()
	a.files.cursor = clamp(a.ctxIdx, 0, max(0, len(a.files.rows)-1))
}

func (a *App) moveCursor(delta int) {
	l := a.level()
	n := len(l.visible())
	if n == 0 {
		l.cursor = 0
		return
	}
	l.cursor = clamp(l.cursor+delta, 0, n-1)
}

func (a *App) quit() tea.Cmd {
	if !a.Dirty() {
		a.quitting = true
		return tea.Quit
	}
	a.ov = confirmOverlay(
		"Unsaved changes",
		fmt.Sprintf("%s has unsaved edits.", a.dirtyList()),
		[]choice{
			{label: "Save and quit", value: "save"},
			{label: "Quit without saving", value: "discard", danger: true},
			{label: "Keep editing", value: "cancel"},
		},
		func(app *App, value string) (string, error) {
			switch value {
			case "save":
				for i := range app.ctxs {
					if app.ctxs[i].Dirty() {
						if err := app.ctxs[i].Save(); err != nil {
							return "", err
						}
					}
				}
				app.quitting = true
				return "", nil
			case "discard":
				app.quitting = true
				return "", nil
			}
			return "", nil
		})
	return nil
}

func (a *App) dirtyList() string {
	var names []string
	for i := range a.ctxs {
		if a.ctxs[i].Dirty() {
			names = append(names, a.ctxs[i].Name)
		}
	}
	return strings.Join(names, ", ")
}

func (a *App) save() tea.Cmd {
	c := a.Context()
	if c == nil {
		a.setStatus("No settings file selected.", statusBad)
		return nil
	}
	if !c.Editable() {
		a.setStatus(fmt.Sprintf("%s is read-only.", c.Name), statusBad)
		return nil
	}
	if !c.Dirty() {
		a.setStatus("Nothing to save.", statusInfo)
		return nil
	}
	if err := c.Save(); err != nil {
		a.setStatus(err.Error(), statusBad)
		return nil
	}
	a.setStatus(fmt.Sprintf("Saved %s", c.Path), statusGood)
	return nil
}

func (a *App) revert() tea.Cmd {
	c := a.Context()
	if c == nil || !c.Dirty() {
		a.setStatus("No unsaved edits in this file.", statusInfo)
		return nil
	}
	a.ov = confirmOverlay("Discard edits",
		fmt.Sprintf("Throw away the unsaved edits to %s?", c.Name),
		[]choice{
			{label: "Discard edits", value: "yes", danger: true},
			{label: "Keep editing", value: "no"},
		},
		func(app *App, value string) (string, error) {
			if value != "yes" {
				return "", nil
			}
			app.Context().Revert()
			app.rebuild()
			return "Edits discarded.", nil
		})
	return nil
}

func (a *App) reload() tea.Cmd {
	c := a.Context()
	if c == nil {
		return nil
	}
	if err := c.Load(); err != nil {
		a.setStatus(err.Error(), statusBad)
		return nil
	}
	a.rebuild()
	a.setStatus(fmt.Sprintf("Reloaded %s", c.Path), statusGood)
	return nil
}

func (a *App) layout() {
	w, h := a.detailSize()
	a.detail.Width = w
	a.detail.Height = h
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Contexts returns the settings files the app has open.
func (a *App) Contexts() []*config.Context {
	out := make([]*config.Context, len(a.ctxs))
	for i := range a.ctxs {
		out[i] = &a.ctxs[i]
	}
	return out
}
