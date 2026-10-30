package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MSarandev/sugarlime/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const sampleSettings = `{
  "model": "claude-sonnet-5-5",
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(git diff:*)", "Read(~/.zshrc)"],
    "deny": ["Read(./.env)"]
  },
  "sandbox": {
    "enabled": true,
    "network": {
      "allowedDomains": ["github.com"]
    }
  },
  "env": {
    "EDITOR": "vim"
  },
  "someKeyWeDoNotKnow": 42
}`

// newTestApp builds an app over a temp settings file holding the given JSON.
func newTestApp(t *testing.T, body string) (*App, *config.Context) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := config.Context{Name: "test", Path: path, Kind: config.KindUser}
	_ = c.Load() // a deliberately broken file is a case the tests cover
	a := New([]config.Context{c})
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	enterBrowse(a)
	return a, a.Context()
}

// enterBrowse opens the first settings file, as pressing enter on the picker
// does, so a test can start from the browser.
func enterBrowse(a *App) {
	a.files.cursor = 0
	a.openFileRow()
	a.status = ""
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// press sends a sequence of keys to the app.
func press(a *App, keys ...string) {
	for _, k := range keys {
		a.Update(key(k))
	}
}

// typeText sends each rune of s as its own key press.
func typeText(a *App, s string) {
	for _, r := range s {
		a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// gotoTab selects a tab by its title.
func gotoTab(t *testing.T, a *App, title string) {
	t.Helper()
	for i, tb := range tabs {
		if tb.Title == title {
			a.tabIdx = i
			return
		}
	}
	t.Fatalf("no tab named %q", title)
}

// focus moves the cursor to the row whose label matches name.
func focus(t *testing.T, a *App, name string) row {
	t.Helper()
	l := a.level()
	for i, r := range l.visible() {
		if r.label() == name {
			l.cursor = i
			return r
		}
	}
	t.Fatalf("no row named %q in %q; rows: %v", name, l.title, labels(l))
	return row{}
}

func labels(l *level) []string {
	var out []string
	for _, r := range l.visible() {
		out = append(out, r.label())
	}
	return out
}

// lineWidth measures a rendered line, ignoring ANSI styling.
func lineWidth(line string) int {
	return lipgloss.Width(line)
}

func sizeMsg(w, h int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: w, Height: h}
}
