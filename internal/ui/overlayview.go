package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderOverlay draws the modal centred in the body area.
func (a *App) renderOverlay(height int) string {
	ov := a.ov
	width := clamp(a.width-8, 32, 78)

	var b strings.Builder
	b.WriteString(styleTitle.Render(truncate(ov.title, width)) + "\n")
	if ov.hint != "" {
		b.WriteString(styleDim.Render(wrap(ov.hint, width)) + "\n")
	}
	b.WriteString("\n")

	switch ov.kind {
	case ovChoice:
		for i, c := range ov.choices {
			marker := "  "
			labelStyle := styleText
			if i == ov.cursor {
				marker = styleSelected.Render(iconCursor + " ")
				labelStyle = styleSelected
			}
			if i < 9 {
				marker = styleFaint.Render(fmt.Sprintf("%d ", i+1)) + marker[2:]
				if i == ov.cursor {
					marker = styleFaint.Render(fmt.Sprintf("%d", i+1)) + styleSelected.Render(iconCursor+" ")
				}
			}
			if c.danger {
				labelStyle = styleErr
			}
			label := c.label
			if label == "" {
				label = "not set"
			}
			line := marker + labelStyle.Render(label)
			if c.desc != "" {
				line += styleDim.Render("  " + c.desc)
			}
			b.WriteString(truncate(oneLine(line), width) + "\n")
		}

	case ovJSON:
		ov.area.SetWidth(width)
		ov.area.SetHeight(clamp(height-8, 4, 18))
		b.WriteString(ov.area.View() + "\n")

	default:
		ov.input.Width = width - 4
		b.WriteString(ov.input.View() + "\n")
		if c := ov.completion(); c != "" && c != ov.input.Value() {
			b.WriteString(styleFaint.Render(truncate("tab → "+c, width)) + "\n")
		}
		if len(ov.results) > 0 {
			b.WriteString("\n")
			for i, f := range ov.results {
				marker := "  "
				st := styleText
				if i == ov.cursor {
					marker = styleSelected.Render(iconCursor + " ")
					st = styleSelected
				}
				line := marker + st.Render(f.Path) + styleDim.Render("  "+f.Summary)
				b.WriteString(truncate(oneLine(line), width) + "\n")
			}
		}
	}

	if ov.errMsg != "" {
		b.WriteString("\n" + styleErr.Render(wrap(ov.errMsg, width)) + "\n")
	}

	box := styleOverlay.Width(width).Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(a.width, height, lipgloss.Center, lipgloss.Center, box)
}

// QuitMessage is printed after the program exits, so the shell shows what happened.
func (a *App) QuitMessage() string {
	if a.Dirty() {
		return fmt.Sprintf("Left %s unsaved.", a.dirtyList())
	}
	return ""
}
