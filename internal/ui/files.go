package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MSarandev/sugarlime/internal/config"
)

type fileAction int

const (
	fileOpen fileAction = iota
	fileAdd
	fileScan
	fileRescan
)

// fileRow is one line on the file picker: a settings file, or something to do.
type fileRow struct {
	action fileAction
	ctx    int // index into App.ctxs, for fileOpen
	label  string
	hint   string
}

// filePicker is the screen sugarlime opens on: which settings file to edit.
type filePicker struct {
	rows   []fileRow
	cursor int
	note   string
}

// refreshFiles rebuilds the picker's rows from the open settings files.
func (a *App) refreshFiles() {
	var rows []fileRow
	for i := range a.ctxs {
		rows = append(rows, fileRow{action: fileOpen, ctx: i, label: a.ctxs[i].Name})
	}
	rows = append(rows,
		fileRow{action: fileAdd, label: "Add a settings file by path", hint: "a"},
		fileRow{action: fileScan, label: "Scan a directory for settings files", hint: "s"},
		fileRow{action: fileRescan, label: "Look again in the usual places", hint: "r"},
	)
	a.files.rows = rows
	a.files.cursor = clamp(a.files.cursor, 0, len(rows)-1)
}

func (a *App) fileKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		if msg.String() == "esc" && len(a.ctxs) > 0 && a.entered {
			a.screen = screenBrowse
			return nil
		}
		a.quitting = true
		return tea.Quit
	case "?":
		a.showHelp = true
	case "up", "k":
		a.files.cursor = clamp(a.files.cursor-1, 0, len(a.files.rows)-1)
	case "down", "j":
		a.files.cursor = clamp(a.files.cursor+1, 0, len(a.files.rows)-1)
	case "home", "g":
		a.files.cursor = 0
	case "end", "G":
		a.files.cursor = len(a.files.rows) - 1
	case "enter", "right", "l", " ":
		return a.openFileRow()
	case "a", "+":
		return a.openAddContext()
	case "s":
		return a.openScan()
	case "r", "R":
		return a.rescan()
	case "d", "x":
		return a.forgetFileRow()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if n := int(msg.String()[0] - '1'); n < len(a.ctxs) {
			a.files.cursor = n
			return a.openFileRow()
		}
	}
	return nil
}

func (a *App) openFileRow() tea.Cmd {
	if a.files.cursor >= len(a.files.rows) {
		return nil
	}
	r := a.files.rows[a.files.cursor]
	switch r.action {
	case fileAdd:
		return a.openAddContext()
	case fileScan:
		return a.openScan()
	case fileRescan:
		return a.rescan()
	}

	a.ctxIdx = clamp(r.ctx, 0, len(a.ctxs)-1)
	c := a.Context()
	if c == nil {
		return nil
	}
	if c.LoadErr != nil {
		a.setStatus(c.LoadErr.Error(), statusBad)
		return nil
	}
	a.stacks = map[int][]level{}
	a.tabIdx = 0
	a.screen = screenBrowse
	a.entered = true
	if c.ReadOnly {
		a.setStatus(fmt.Sprintf("%s is read-only; it is shown but cannot be changed.", c.Name), statusInfo)
	}
	return nil
}

// forgetFileRow drops a hand-added file from sugarlime's list, leaving the file
// itself alone.
func (a *App) forgetFileRow() tea.Cmd {
	if a.files.cursor >= len(a.files.rows) {
		return nil
	}
	r := a.files.rows[a.files.cursor]
	if r.action != fileOpen {
		return nil
	}
	c := &a.ctxs[r.ctx]
	if c.Kind != config.KindCustom {
		a.setStatus("sugarlime finds this file on its own, so it cannot be removed from the list.", statusInfo)
		return nil
	}
	path, name, idx := c.Path, c.Name, r.ctx
	a.ov = confirmOverlay("Remove from the list",
		fmt.Sprintf("Stop showing %s? The file itself stays on disk.", path),
		[]choice{
			{label: "Remove it from the list", value: "yes", danger: true},
			{label: "Keep it", value: "no"},
		},
		func(app *App, value string) (string, error) {
			if value != "yes" {
				return "", nil
			}
			if err := config.Forget(path); err != nil {
				return "", err
			}
			app.ctxs = append(app.ctxs[:idx], app.ctxs[idx+1:]...)
			app.ctxIdx = clamp(app.ctxIdx, 0, max(0, len(app.ctxs)-1))
			app.stacks = map[int][]level{}
			app.refreshFiles()
			return fmt.Sprintf("Removed %s from the list", name), nil
		})
	return nil
}

// rescan looks again in the usual places, picking up a file created since
// sugarlime started.
func (a *App) rescan() tea.Cmd {
	before := len(a.ctxs)
	found := config.Discover(a.workdir)

	known := map[string]bool{}
	for i := range a.ctxs {
		known[a.ctxs[i].Path] = true
	}
	for _, c := range found {
		if !known[c.Path] {
			a.ctxs = append(a.ctxs, c)
		}
	}
	a.refreshFiles()

	switch n := len(a.ctxs) - before; {
	case n > 0:
		a.setStatus(fmt.Sprintf("Found %s.", plural(n, "new settings file")), statusGood)
	default:
		a.setStatus("Nothing new in the usual places.", statusInfo)
	}
	return nil
}

func (a *App) openScan() tea.Cmd {
	a.ov = textOverlay("Scan for settings files",
		"a directory to look under, such as ~/code; .claude/settings.json files below it are added",
		"~/", func(s string) error {
			_, err := config.ExpandPath(s)
			return err
		},
		func(app *App, value string) (string, error) {
			found, err := config.ScanDir(value)
			if err != nil {
				return "", err
			}
			known := map[string]bool{}
			for i := range app.ctxs {
				known[app.ctxs[i].Path] = true
			}
			added := 0
			for _, p := range found {
				if known[p] {
					continue
				}
				c, err := config.NewContext(p)
				if err != nil {
					continue
				}
				if err := config.Remember(p); err != nil {
					return "", err
				}
				app.ctxs = append(app.ctxs, c)
				added++
			}
			app.refreshFiles()
			if added == 0 {
				return fmt.Sprintf("Found nothing new under %s.", value), nil
			}
			return fmt.Sprintf("Added %s.", plural(added, "settings file")), nil
		})
	return nil
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// fileView is the picker screen: the logo, then the files, then what to do.
func (a *App) fileView() string {
	var b strings.Builder

	if logoFitsPicker(a.width, a.height) {
		b.WriteString(logoBlock() + "\n\n")
	} else {
		b.WriteString(styleTitle.Render("sugarlime") + "\n\n")
	}

	if len(a.ctxs) == 0 {
		b.WriteString(styleWarn.Render(wrap("No Claude settings files found. Add one by path, or scan a directory for them.", a.width)) + "\n\n")
	} else {
		b.WriteString(styleDim.Render("Which settings file do you want to edit?") + "\n\n")
	}

	nameWidth := 8
	for i := range a.ctxs {
		nameWidth = max(nameWidth, lipglossWidth(a.ctxs[i].Name))
	}
	nameWidth = min(nameWidth, max(8, a.width/3))
	pathWidth := max(12, a.width-nameWidth-26)

	for i, r := range a.files.rows {
		marker := "  "
		labelStyle := styleText
		if i == a.files.cursor {
			marker = styleSelected.Render(iconCursor + " ")
			labelStyle = styleSelected
		}

		if r.action != fileOpen {
			if i > 0 && a.files.rows[i-1].action == fileOpen {
				b.WriteString("\n")
			}
			label := truncate(r.label, max(8, a.width-10))
			pad := strings.Repeat(" ", max(1, 42-lipglossWidth(label)))
			line := marker + styleKey.Render("+ ") + labelStyle.Render(label)
			if r.hint != "" {
				line += pad + styleFaint.Render(r.hint)
			}
			b.WriteString(truncate(line, a.width) + "\n")
			continue
		}

		c := &a.ctxs[r.ctx]
		name := r.label + strings.Repeat(" ", max(0, nameWidth-lipglossWidth(r.label)))
		path := shortenPath(c.Path, pathWidth)
		path += strings.Repeat(" ", max(0, pathWidth-lipglossWidth(path)))

		state, style := a.contextState(c)
		if c.LoadErr == nil && c.Exists {
			state = plural(c.Doc.Len(), "key")
			style = styleDim
			if c.ReadOnly {
				state = "read-only · " + state
			}
		}

		line := marker + labelStyle.Render(name) + "  " +
			styleDim.Render(path) + "  " + style.Render(state)
		b.WriteString(truncate(line, a.width) + "\n")
	}

	b.WriteString("\n")
	if a.status != "" {
		st := styleDim
		switch a.statusKind {
		case statusGood:
			st = styleOK
		case statusBad:
			st = styleErr
		}
		b.WriteString(st.Render(wrap(a.status, a.width)) + "\n\n")
	} else {
		b.WriteString(styleFaint.Render(wrap(discoveryNote(), a.width)) + "\n\n")
	}

	keys := "↑↓ move · enter open · a add · s scan · r rescan · ? help · q exit"
	if a.entered {
		keys = "↑↓ move · enter open · a add · s scan · r rescan · esc back · q exit"
	}
	b.WriteString(styleDim.Render(truncate(keys, a.width)))

	body := b.String()
	if a.ov != nil {
		return a.renderOverlay(a.height - 1)
	}
	return body
}

func discoveryNote() string {
	return "sugarlime looks in ~/.claude, $CLAUDE_CONFIG_DIR, any ~/.claude* environment, " +
		"this project's .claude, managed settings and ~/.claude.json."
}
