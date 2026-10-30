package ui

import (
	"fmt"
	"strings"

	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/permissions"
	"github.com/MSarandev/sugarlime/internal/sandbox"
	"github.com/MSarandev/sugarlime/internal/schema"
	"github.com/charmbracelet/lipgloss"
)

// renderDetail describes whatever the cursor is on.
func (a *App) renderDetail(height int) string {
	w, _ := a.detailSize()
	body := a.detailBody(w)
	a.detail.Width = w
	a.detail.Height = height
	a.detail.SetContent(body)
	return lipgloss.NewStyle().Width(w).Render(pad(a.detail.View(), height))
}

func (a *App) detailBody(width int) string {
	r, ok := a.level().current()
	if !ok {
		return a.contextOverview(width)
	}

	switch r.kind {
	case rowSection:
		return a.sectionDetail(r, width)
	case rowItem:
		return a.listItemDetail(r, width)
	case rowMapEntry:
		return a.mapEntryDetail(r, width)
	default:
		return a.fieldDetail(r.field, width)
	}
}

func (a *App) sectionDetail(r row, width int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(r.title) + "\n\n")
	if r.note != "" {
		b.WriteString(styleDim.Render(r.note) + "\n\n")
	}
	if len(r.paths) == 1 && r.paths[0] == "__unknown__" {
		b.WriteString(wrap("Keys this file sets that the bundled settings reference does not describe. They may come from a newer Claude Code, a typo, or a tool of your own. sugarlime leaves them alone unless you change them.", width) + "\n\n")
		for _, u := range unknownFields(a.doc()) {
			b.WriteString(styleKey.Render("  "+u.field.Path) + "\n")
		}
		return b.String()
	}
	if len(r.cats) == 1 {
		set := 0
		for _, f := range schema.All() {
			if f.Category == r.cats[0] && a.doc().Has(f.Path) {
				set++
				if set <= 12 {
					b.WriteString(styleKey.Render("  "+f.Path) + "\n")
				}
			}
		}
		if set == 0 {
			b.WriteString(wrap("Nothing here is set in this file. These are the keys it offers:", width) + "\n\n")
			shown := 0
			for _, f := range schema.All() {
				if f.Category != r.cats[0] || f.Kind == schema.KindObject {
					continue
				}
				if shown == 10 {
					b.WriteString(styleDim.Render("  …\n"))
					break
				}
				b.WriteString(styleKey.Render("  "+f.Name) + styleDim.Render("  "+truncate(f.Summary, max(10, width-len(f.Name)-6))) + "\n")
				shown++
			}
		} else if set > 12 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  … and %d more\n", set-12)))
		}
	}
	return b.String()
}

func (a *App) listItemDetail(r row, width int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("Entry %d", r.index+1)) + "\n\n")
	b.WriteString(wrap(r.title, width) + "\n\n")

	if strings.HasPrefix(r.owner, "permissions.") {
		if desc := permissions.Describe(r.title); desc != "" {
			b.WriteString(wrap(desc, width) + "\n")
		}
	}
	if v := validatorFor(r.owner); v != nil {
		if err := v(r.title); err != nil {
			b.WriteString("\n" + styleErr.Render(wrap("Invalid: "+err.Error(), width)) + "\n")
		}
	}
	if w := warnerFor(r.owner); w != nil {
		if note := w(r.title); note != "" {
			b.WriteString("\n" + styleWarn.Render(wrap("Note: "+note, width)) + "\n")
		}
	}
	if sandbox.KindFor(r.owner) == sandbox.ListPaths {
		b.WriteString("\n" + styleDim.Render("Path prefixes") + "\n")
		for _, p := range sandbox.Prefixes {
			b.WriteString(styleKey.Render("  "+p.Label) + styleDim.Render("  "+p.Doc) + "\n")
		}
	}
	return b.String()
}

func (a *App) mapEntryDetail(r row, width int) string {
	full := r.owner + "." + r.key
	v, _ := a.doc().Get(full)
	var b strings.Builder
	b.WriteString(styleTitle.Render(r.key) + "\n\n")
	b.WriteString(styleDim.Render("in "+r.owner) + "\n\n")
	b.WriteString(styleValue.Render(wrap(jsondoc.Render(v), width)) + "\n")
	return b.String()
}

func (a *App) fieldDetail(f schema.Field, width int) string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(f.Path) + "\n")
	if f.Deprecated {
		b.WriteString(styleWarn.Render("deprecated") + "\n")
	}
	b.WriteString("\n")

	if f.Doc != "" {
		b.WriteString(wrap(f.Doc, width) + "\n\n")
	} else if f.Summary != "" {
		b.WriteString(wrap(f.Summary, width) + "\n\n")
	}
	if f.Notice != "" {
		b.WriteString(styleWarn.Render(wrap(f.Notice, width)) + "\n\n")
	}

	b.WriteString(a.valueBlock(f, width))

	if f.TypeText != "" {
		b.WriteString(labelled("Accepts", f.TypeText, width))
	}
	for _, o := range f.Options {
		head := styleKey.Render("  " + o.Value)
		if o.Desc == "" {
			b.WriteString(head + "\n")
			continue
		}
		b.WriteString(head + styleDim.Render(":") + "\n")
		b.WriteString(styleDim.Render(hang(o.Desc, width, 4)) + "\n")
	}
	if f.Default != "" {
		b.WriteString(labelled("Default", f.Default, width))
	}
	if f.Scope != "" {
		b.WriteString(labelled("Goes in", f.Scope, width))
	}
	if c := a.Context(); c != nil && !c.Accepts(f) {
		where := "a settings file"
		if f.GlobalConfig() {
			where = "~/.claude.json"
		}
		b.WriteString("\n" + styleWarn.Render(wrap(fmt.Sprintf("Claude Code reads this key from %s, not from %s.", where, c.Path), width)) + "\n")
	}
	if f.Managed() {
		b.WriteString("\n" + styleDim.Render(wrap("Claude Code honours this key only in managed settings.", width)) + "\n")
	}

	b.WriteString("\n" + styleFaint.Render(wrap(editHint(f), width)) + "\n")
	return b.String()
}

// valueBlock shows what the active file sets, and the other files that set it too.
func (a *App) valueBlock(f schema.Field, width int) string {
	var b strings.Builder
	v, ok := a.doc().Get(f.Path)
	b.WriteString(label("In this file"))
	if !ok {
		b.WriteString(styleUnset.Render("not set") + "\n")
	} else {
		rendered := jsondoc.Render(v)
		if strings.Contains(rendered, "\n") {
			b.WriteString("\n")
			for _, line := range strings.Split(rendered, "\n") {
				b.WriteString(styleValue.Render("  "+truncate(line, width-2)) + "\n")
			}
		} else {
			b.WriteString(styleValue.Render(truncate(rendered, width-14)) + "\n")
		}
	}

	var others []string
	for i := range a.ctxs {
		if i == a.ctxIdx || a.ctxs[i].Doc == nil {
			continue
		}
		if other, ok := a.ctxs[i].Doc.Get(f.Path); ok {
			others = append(others, fmt.Sprintf("%s = %s", a.ctxs[i].Name, oneLine(jsondoc.Render(other))))
		}
	}
	if len(others) > 0 {
		b.WriteString(label("Also set in"))
		b.WriteString("\n")
		for _, o := range others {
			b.WriteString(styleDim.Render("  "+truncate(o, width-2)) + "\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

func editHint(f schema.Field) string {
	switch f.Kind {
	case schema.KindBool:
		return "space cycles true, false and unset. enter opens the picker."
	case schema.KindEnum:
		return "enter opens the list of accepted values."
	case schema.KindStringList:
		return "enter opens the list, where a adds and d deletes an entry."
	case schema.KindObject:
		return "enter opens the keys that nest under it."
	case schema.KindRaw:
		return "enter opens a JSON editor; ctrl+s applies it."
	case schema.KindNumber:
		return "enter edits the number. An empty value unsets the key."
	default:
		return "enter edits the value. An empty value unsets the key."
	}
}

func label(s string) string {
	return styleDim.Render(s + ": ")
}

// labelled writes "Label: text", wrapping under the label, not mid-word.
func labelled(name, text string, width int) string {
	head := name + ": "
	if lipgloss.Width(head+text) <= width {
		return styleDim.Render(head) + styleText.Render(text) + "\n"
	}
	return styleDim.Render(head) + "\n" + styleText.Render(hang(text, width, 2)) + "\n"
}

// contextOverview is the detail pane's fallback when no row is selected.
func (a *App) contextOverview(width int) string {
	c := a.Context()
	if c == nil {
		return wrap("No settings file is open. Press c to add one, or a on the Overview tab.", width)
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render(c.Name) + "\n\n")
	b.WriteString(styleDim.Render(c.Path) + "\n\n")
	if c.LoadErr != nil {
		b.WriteString(styleErr.Render(wrap(c.LoadErr.Error(), width)) + "\n\n")
	}
	b.WriteString(wrap(kindDoc(c.Kind), width) + "\n")
	return b.String()
}

// hang wraps text and indents every line by indent columns.
func hang(s string, width, indent int) string {
	pad := strings.Repeat(" ", indent)
	wrapped := wrap(s, max(8, width-indent))
	lines := strings.Split(wrapped, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

func wrap(s string, width int) string {
	if width <= 2 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}
