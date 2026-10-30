package ui

import (
	"fmt"
	"strings"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/charmbracelet/lipgloss"
)

type keyHelp struct {
	keys string
	what string
}

var helpSections = []struct {
	title string
	keys  []keyHelp
}{
	{"Moving around", []keyHelp{
		{"↑ ↓ / k j", "move the cursor"},
		{"enter / →", "open a group, or edit the key"},
		{"esc / ←", "go back one level, or clear the filter"},
		{"tab / shift+tab", "next or previous tab"},
		{"1 … 9", "jump straight to a tab"},
		{"g / G", "first or last row"},
		{"/", "filter the list you are on"},
		{"f", "find any key in the settings reference"},
	}},
	{"Changing values", []keyHelp{
		{"space", "cycle a Boolean: true, false, not set"},
		{"enter", "pick a value, edit text, or open the JSON editor"},
		{"x", "unset the key, so Claude Code uses its default"},
		{"a", "add an entry to a list, or a key to an object"},
		{"d", "delete the entry under the cursor"},
		{"J / K", "move a list entry down or up"},
	}},
	{"Files", []keyHelp{
		{"c", "go back to the file picker, to switch, add or scan"},
		{"s / ctrl+s", "save the open file"},
		{"r", "discard unsaved edits"},
		{"R", "re-read the file from disk"},
	}},
	{"On the file picker", []keyHelp{
		{"enter, 1 … 9", "open that settings file"},
		{"a", "add a settings file by path"},
		{"s", "scan a directory for settings files"},
		{"r", "look again in the usual places"},
		{"d", "remove a file you added from the list"},
		{"esc", "back to the file you were editing"},
	}},
	{"Leaving", []keyHelp{
		{"q", "quit, asking first if anything is unsaved"},
		{"?", "close this help"},
	}},
}

func (a *App) helpView() string {
	blocks := make([]string, 0, len(helpSections))
	for _, sec := range helpSections {
		var b strings.Builder
		b.WriteString(styleTitle.Render(sec.title) + "\n")
		for _, k := range sec.keys {
			b.WriteString(styleKey.Render(fmt.Sprintf("  %-16s", k.keys)) + styleText.Render(k.what) + "\n")
		}
		blocks = append(blocks, b.String())
	}

	title := styleTitle.Render("sugarlime — Claude settings editor")
	if logoFitsPicker(a.width, a.height) {
		title = logoBlock()
	}

	content := title + "\n\n" + a.helpColumns(blocks) + "\n" + a.helpFooter() +
		"\n" + styleFaint.Render("press ? or esc to go back")

	lines := strings.Split(lipgloss.NewStyle().MaxWidth(a.width).Render(content), "\n")
	if len(lines) > a.height {
		lines = lines[:max(1, a.height-1)]
		lines = append(lines, styleFaint.Render("… widen the window to see the rest"))
	}
	return strings.Join(lines, "\n")
}

// helpColumns lays the sections side by side when the window is wide enough,
// so the whole list fits on one screen.
func (a *App) helpColumns(blocks []string) string {
	colWidth := 0
	for _, b := range blocks {
		colWidth = max(colWidth, lipgloss.Width(b))
	}
	if a.width < colWidth*2+4 {
		return strings.Join(blocks, "\n")
	}

	half := (len(blocks) + 1) / 2
	left := strings.Join(blocks[:half], "\n")
	right := strings.Join(blocks[half:], "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colWidth+3).Render(left),
		right,
	)
}

func (a *App) helpFooter() string {
	out := styleDim.Render("Where sugarlime looks for settings files:") + "\n"
	for _, row := range [][2]string{
		{"~/.claude", "your settings for every project"},
		{"$CLAUDE_CONFIG_DIR", "the Claude directory a session is using"},
		{"~/.claude*", "separate environments, such as ~/.claude-work"},
		{"./.claude", "this project, found by walking up from here"},
	} {
		out += styleKey.Render(fmt.Sprintf("  %-20s ", row[0])) + styleDim.Render(row[1]) + "\n"
	}
	out += "\n" + styleDim.Render("Claude Code reads these files, in the order they win:") + "\n"
	for _, k := range []config.Kind{config.KindManaged, config.KindLocal, config.KindProject, config.KindUser} {
		out += styleKey.Render(fmt.Sprintf("  %-20s ", k.String())) + styleDim.Render(kindDoc(k)) + "\n"
	}
	return out
}

func kindDoc(k config.Kind) string {
	switch k {
	case config.KindUser:
		return "~/.claude/settings.json, your settings for every project"
	case config.KindProject:
		return ".claude/settings.json, shared with the repository"
	case config.KindLocal:
		return ".claude/settings.local.json, yours and not committed"
	case config.KindManaged:
		return "deployed by your organization; read-only here"
	case config.KindGlobal:
		return "~/.claude.json, the few keys Claude Code keeps outside settings files"
	default:
		return "a file you added by hand"
	}
}
