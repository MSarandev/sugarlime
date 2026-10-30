package ui

import (
	"fmt"
	"strings"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/schema"
	"github.com/charmbracelet/lipgloss"
)

const (
	minDetailWidth = 34
	listMinWidth   = 30
)

func (a *App) View() string {
	if a.quitting {
		return ""
	}
	if a.showHelp {
		return a.helpView()
	}
	if a.screen == screenFiles {
		return a.fileView()
	}

	header := a.renderHeader()
	tabBar := a.renderTabs()
	footer := a.renderFooter()

	bodyHeight := a.bodyHeight()
	var body string
	if a.ov != nil {
		body = a.renderOverlay(bodyHeight)
	} else {
		body = a.renderBody(bodyHeight)
	}

	return strings.Join([]string{header, tabBar, body, footer}, "\n")
}

func (a *App) bodyHeight() int {
	// The tab bar and footer take a line each, and one line is left spare: a
	// view that fills the terminal exactly scrolls the top line away.
	return max(3, a.height-a.headerHeight()-3)
}

func (a *App) headerHeight() int {
	if a.showLogo() {
		return logoHeight() + 2 // a blank line above and below the logo
	}
	return 1
}

// showLogo reports whether the terminal has room for the logo block without
// crowding out the list.
func (a *App) showLogo() bool {
	return logoFits(a.width, a.height)
}

// listWidth splits the body between the list and the detail pane.
func (a *App) listWidth() int {
	if !a.showDetail() {
		return a.width
	}
	w := a.width * 2 / 5
	return clamp(w, listMinWidth, a.width-minDetailWidth-3)
}

func (a *App) showDetail() bool {
	return a.width >= listMinWidth+minDetailWidth+3
}

func (a *App) detailSize() (int, int) {
	if !a.showDetail() {
		return max(10, a.width), a.bodyHeight()
	}
	return max(10, a.width-a.listWidth()-3), a.bodyHeight()
}

func (a *App) listHeight() int { return a.bodyHeight() }

func (a *App) renderHeader() string {
	if !a.showLogo() {
		return a.compactHeader()
	}

	width := max(24, a.width-logoWidth-5)
	block := lipgloss.JoinHorizontal(lipgloss.Bottom,
		logoBlock(),
		lipgloss.NewStyle().PaddingLeft(3).Render(a.openFileBlock(width)),
	)
	// The leading blank line keeps the logo clear of the top of the window.
	return "\n" + lipgloss.NewStyle().MaxWidth(a.width).Render(block) + "\n"
}

// openFileBlock is the headline beside the logo: which file is being edited,
// its state, and how to change it.
func (a *App) openFileBlock(width int) string {
	c := a.Context()
	if c == nil {
		return styleDim.Render("editing") + "\n" +
			styleWarn.Render(" no settings file ") + "\n\n" +
			styleFaint.Render("press c to pick one")
	}

	state, style := a.contextState(c)

	return styleDim.Render("editing") + "\n" +
		styleFileName.Render(" "+strings.ToUpper(truncate(c.Name, width-4))+" ") +
		"  " + style.Render(state) + "\n" +
		styleDim.Render(truncate(shortenPath(c.Path, width), width)) + "\n" +
		styleFaint.Render("press ") + styleKey.Render("c") + styleFaint.Render(" to change file")
}

// compactHeader is the one-line header for a terminal too small for the logo.
func (a *App) compactHeader() string {
	c := a.Context()
	left := lipgloss.NewStyle().Bold(true).Foreground(colAccent).Render(" sugarlime ")
	if a.width < 28 {
		left = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Render(" sl ")
	}

	state, stateStyle := a.contextState(c)
	right := ""
	if state != "" {
		right = stateStyle.Render(state + " ")
	}

	room := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	mid := ""
	if c == nil {
		mid = styleWarn.Render(truncate("no settings file", room))
	} else {
		name := styleFileName.Render(" " + strings.ToUpper(truncate(c.Name, max(4, room/2))) + " ")
		path := shortenPath(c.Path, max(0, room-lipgloss.Width(name)-2))
		mid = name
		if lipgloss.Width(name)+lipgloss.Width(path)+2 <= room {
			mid += "  " + styleDim.Render(path)
		}
	}

	gap := max(0, a.width-lipgloss.Width(left)-lipgloss.Width(mid)-lipgloss.Width(right))
	return left + mid + strings.Repeat(" ", gap) + right
}

// contextState is the one-word condition shown at the right of the header.
func (a *App) contextState(c *config.Context) (string, lipgloss.Style) {
	switch {
	case c == nil:
		return "", styleDim
	case c.LoadErr != nil:
		return "invalid JSON", styleErr
	case c.Dirty():
		return iconDirty + " unsaved", styleWarn
	case c.ReadOnly:
		return "read-only", styleDim
	case !c.Exists:
		return "new file", styleDim
	default:
		return "saved", styleOK
	}
}

func (a *App) renderTabs() string {
	var parts []string
	useShort := false
	for pass := 0; pass < 2; pass++ {
		parts = parts[:0]
		width := 0
		for i, t := range tabs {
			label := t.Title
			if useShort {
				label = t.Short
			}
			st := styleTabIdle
			if i == a.tabIdx {
				st = styleTabActive
			}
			rendered := st.Render(label)
			width += lipgloss.Width(rendered)
			parts = append(parts, rendered)
		}
		if width <= a.width || useShort {
			break
		}
		useShort = true
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
	if lipgloss.Width(bar) > a.width {
		// Last resort on a very narrow terminal: show the active tab only.
		bar = styleTabActive.Render(fmt.Sprintf("%d/%d %s", a.tabIdx+1, len(tabs), tabs[a.tabIdx].Title))
	}
	return bar
}

func (a *App) renderBody(height int) string {
	list := a.renderList(a.listWidth(), height)
	if !a.showDetail() {
		return list
	}
	rule := lipgloss.NewStyle().Foreground(colBorder).
		Render(strings.TrimRight(strings.Repeat("│\n", height), "\n"))
	detail := a.renderDetail(height)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(a.listWidth()).Render(list),
		lipgloss.NewStyle().Padding(0, 1).Render(rule),
		detail,
	)
}

func (a *App) renderList(width, height int) string {
	l := a.level()
	rows := l.visible()

	var lines []string

	crumb := a.crumbs()
	if crumb != "" {
		lines = append(lines, styleTitle.Render(truncate(crumb, width)))
	}
	if l.filter != "" {
		lines = append(lines, styleDim.Render(truncate(fmt.Sprintf("filter: %s (%d)", l.filter, len(rows)), width)))
	}
	head := len(lines)

	if len(rows) == 0 {
		lines = append(lines, "", styleDim.Render(a.emptyMessage()))
		return pad(strings.Join(lines, "\n"), height)
	}

	// Keep the cursor inside the window.
	avail := max(1, height-head)
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+avail {
		l.offset = l.cursor - avail + 1
	}
	l.offset = clamp(l.offset, 0, max(0, len(rows)-avail))

	end := min(len(rows), l.offset+avail)
	for i := l.offset; i < end; i++ {
		lines = append(lines, a.renderRow(rows[i], i == l.cursor, width))
	}
	if len(rows) > avail {
		lines[len(lines)-1] = styleDim.Render(truncate(fmt.Sprintf("  … %d more", len(rows)-end+1), width))
	}
	return pad(strings.Join(lines, "\n"), height)
}

func (a *App) emptyMessage() string {
	l := a.level()
	switch {
	case l.filter != "":
		return "  Nothing matches. Press esc to clear the filter."
	case l.listPath != "":
		return "  This list is empty. Press a to add an entry."
	case l.mapPath != "":
		return "  No keys yet. Press a to add one."
	default:
		return "  Nothing to show here."
	}
}

func (a *App) renderRow(r row, selected bool, width int) string {
	marker := "  "
	if selected {
		marker = styleSelected.Render(iconCursor + " ")
	}

	name := r.label()
	value := a.rowValue(r)

	nameStyle := styleText
	switch {
	case r.kind == rowField && r.field.Deprecated:
		nameStyle = styleFaint
	case selected:
		nameStyle = styleSelected
	}

	suffix := ""
	if r.drills() {
		suffix = " " + iconDrill
	}

	// Lay the name on the left and the value on the right.
	nameW := lipgloss.Width(name) + lipgloss.Width(suffix)
	valueW := lipgloss.Width(value)
	space := width - 2 - nameW - valueW - 1
	if space < 1 {
		keep := max(6, width-2-valueW-3-lipgloss.Width(suffix))
		name = truncate(name, keep)
		nameW = lipgloss.Width(name) + lipgloss.Width(suffix)
		space = max(1, width-2-nameW-valueW-1)
	}

	return marker + nameStyle.Render(name) + styleFaint.Render(suffix) +
		strings.Repeat(" ", space) + value
}

// rowValue renders the right-hand column of a row.
func (a *App) rowValue(r row) string {
	switch r.kind {
	case rowSection:
		return styleDim.Render(r.note)
	case rowItem:
		if w := warnerFor(r.owner); w != nil {
			if note := w(r.title); note != "" {
				return styleBadge.Render("!")
			}
		}
		return ""
	case rowMapEntry:
		return styleValue.Render(truncate(r.note, 28))
	}

	f := r.field
	v, ok := a.doc().Get(f.Path)
	if !ok {
		if f.Kind == schema.KindObject {
			return styleUnset.Render("not set")
		}
		return styleUnset.Render("not set")
	}

	switch f.Kind {
	case schema.KindBool:
		if b, _ := v.(bool); b {
			return styleValue.Render(iconCheckOn + " true")
		}
		return styleWarn.Render(iconCheckOff + " false")
	case schema.KindStringList:
		if vals, ok := a.doc().Strings(f.Path); ok {
			return styleValue.Render(jsondoc.Summary(toAny(vals)))
		}
		return styleWarn.Render("not a string list")
	default:
		return styleValue.Render(truncate(oneLine(jsondoc.Summary(v)), 30))
	}
}

func toAny(vals []string) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}

func (a *App) crumbs() string {
	stack := a.stack()
	parts := make([]string, 0, len(stack))
	for i, l := range stack {
		if i == 0 {
			parts = append(parts, tabs[a.tabIdx].Title)
			continue
		}
		name := l.crumb
		if name == "" {
			name = l.title
		}
		parts = append(parts, name)
	}
	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, " / ")
}

func (a *App) renderFooter() string {
	if a.status != "" {
		st := styleDim
		switch a.statusKind {
		case statusGood:
			st = styleOK
		case statusBad:
			st = styleErr
		}
		return st.Render(truncate(" "+a.status, a.width))
	}
	return styleDim.Render(truncate(" "+a.footerKeys(), a.width))
}

func (a *App) footerKeys() string {
	if a.ov != nil {
		switch a.ov.kind {
		case ovChoice:
			return "↑↓ choose · enter select · esc cancel"
		case ovJSON:
			return "ctrl+s apply · esc cancel"
		default:
			return "enter apply · tab complete · esc cancel"
		}
	}

	l := a.level()
	var hints []string
	switch {
	case l.listPath != "":
		hints = []string{"a add", "enter edit", "d delete", "J/K reorder", "← back"}
	case l.mapPath != "":
		hints = []string{"a add", "enter edit", "d delete", "← back"}
	default:
		hints = []string{"↑↓ move", "enter open", "space toggle", "x unset", "/ filter", "f find"}
	}
	// These stay on the line at any width: saving, changing file, and leaving.
	return fitHints(hints, []string{"s save", "c change file", "? help", "q exit"}, a.width-1)
}

// fitHints drops the optional hints from the end until the line fits, keeping
// the ones a user must be able to find.
func fitHints(optional, always []string, width int) string {
	const sep = " · "
	for i := len(optional); i >= 0; i-- {
		line := strings.Join(append(append([]string{}, optional[:i]...), always...), sep)
		if lipgloss.Width(line) <= width || i == 0 {
			return line
		}
	}
	return strings.Join(always, sep)
}

// shortenPath keeps the identifying tail of a path, truncating if even that is too long.
func shortenPath(p string, width int) string {
	if width <= 0 || lipgloss.Width(p) <= width {
		return p
	}
	if parts := strings.Split(p, "/"); len(parts) >= 3 {
		if short := ".../" + strings.Join(parts[len(parts)-2:], "/"); lipgloss.Width(short) <= width {
			return short
		}
	}
	return truncate(p, width)
}

// lipglossWidth is the display width of a string, escapes excluded.
func lipglossWidth(s string) int { return lipgloss.Width(s) }

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if width == 1 {
		return "…"
	}
	return string(runes[:min(len(runes), width-1)]) + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// pad grows a block to a fixed number of lines so panes line up.
func pad(s string, height int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
