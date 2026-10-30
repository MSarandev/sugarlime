package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MSarandev/sugarlime/internal/catalog"
	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/permissions"
	"github.com/MSarandev/sugarlime/internal/sandbox"
	"github.com/MSarandev/sugarlime/internal/schema"
	tea "github.com/charmbracelet/bubbletea"
)

// openCurrent drills into the row under the cursor, or opens its editor.
func (a *App) openCurrent() tea.Cmd {
	r, ok := a.level().current()
	if !ok {
		return nil
	}
	switch r.kind {
	case rowSection:
		a.push(a.sectionLevel(r))
	case rowField:
		return a.openField(r.field)
	case rowItem:
		return a.editListItem(r)
	case rowMapEntry:
		return a.editMapEntry(r)
	}
	return nil
}

func (a *App) sectionLevel(r row) level {
	if len(r.paths) == 1 && r.paths[0] == "__unknown__" {
		return level{title: "Keys not in the reference", crumb: r.title, rows: unknownFields(a.doc())}
	}
	if len(r.cats) == 1 {
		return level{title: r.cats[0], crumb: r.title, rows: a.categoryRows(r.cats[0])}
	}
	return level{title: r.title, crumb: r.title, rows: fieldRows(nil)}
}

// openField routes a key to the editor its kind needs.
func (a *App) openField(f schema.Field) tea.Cmd {
	if c := a.Context(); c != nil && !c.Editable() {
		a.setStatus(fmt.Sprintf("%s is read-only; this key cannot be changed here.", c.Name), statusBad)
		return nil
	}
	doc := a.doc()
	cur, isSet := doc.Get(f.Path)

	switch f.Kind {
	case schema.KindObject:
		a.push(level{title: f.Path, crumb: f.Name, rows: a.objectRows(f)})
		return nil

	case schema.KindStringList:
		vals, ok := doc.Strings(f.Path)
		if isSet && !ok {
			return a.openRawEditor(f)
		}
		a.push(level{
			title:    f.Path,
			crumb:    f.Name,
			rows:     listRows(f.Path, vals),
			listPath: f.Path,
		})
		return nil

	case schema.KindBool:
		return a.openBoolPicker(f)

	case schema.KindEnum:
		return a.openEnumPicker(f)

	case schema.KindNumber:
		return a.openNumberEditor(f)

	case schema.KindString:
		if catalog.IsModelField(f.Path) {
			return a.openEnumPicker(f)
		}
		return a.openStringEditor(f)

	default:
		if obj, ok := cur.(*jsondoc.Object); ok && flatObject(obj) {
			a.push(level{
				title:   f.Path,
				crumb:   f.Name,
				rows:    mapRows(doc, f.Path),
				mapPath: f.Path,
			})
			return nil
		}
		if !isSet && mapShaped(f) {
			a.push(level{
				title:   f.Path,
				crumb:   f.Name,
				rows:    mapRows(doc, f.Path),
				mapPath: f.Path,
			})
			return nil
		}
		return a.openRawEditor(f)
	}
}

// objectRows lists an object's children, including ones the reference omits.
func (a *App) objectRows(f schema.Field) []row {
	rows := fieldRows(schema.Children(f.Path))
	known := map[string]bool{}
	for _, r := range rows {
		known[r.field.Path] = true
	}
	if obj, ok := a.doc().Get(f.Path); ok {
		if o, isObj := obj.(*jsondoc.Object); isObj {
			for _, k := range o.Keys() {
				path := f.Path + "." + k
				if known[path] {
					continue
				}
				v, _ := o.Get(k)
				rows = append(rows, row{
					kind: rowField,
					field: schema.Field{
						Path:     path,
						Name:     k,
						Category: f.Category,
						Summary:  "Not described in the settings reference bundled with sugarlime.",
						Kind:     kindOfValue(v),
					},
					title: k,
				})
			}
		}
	}
	return rows
}

// flatObject reports whether an object can be edited as key/value pairs.
func flatObject(o *jsondoc.Object) bool {
	if o.Len() == 0 {
		return false
	}
	for _, k := range o.Keys() {
		v, _ := o.Get(k)
		switch v.(type) {
		case string, bool, json.Number, nil:
		default:
			return false
		}
	}
	return true
}

// mapShaped lets an unset mapping key still open the pair editor.
func mapShaped(f schema.Field) bool {
	t := strings.ToLower(f.TypeText)
	if !strings.HasPrefix(t, "object mapping") {
		return false
	}
	return strings.Contains(t, "to a boolean") || strings.Contains(t, "to string") ||
		strings.Contains(t, "string values") || strings.Contains(t, "provider model id") ||
		strings.Contains(t, `to "<esc>"`)
}

func listRows(path string, vals []string) []row {
	out := make([]row, 0, len(vals))
	for i, v := range vals {
		out = append(out, row{kind: rowItem, owner: path, index: i, title: v})
	}
	return out
}

func mapRows(doc *jsondoc.Object, path string) []row {
	var out []row
	v, ok := doc.Get(path)
	if !ok {
		return out
	}
	o, isObj := v.(*jsondoc.Object)
	if !isObj {
		return out
	}
	for _, k := range o.Keys() {
		val, _ := o.Get(k)
		out = append(out, row{
			kind:  rowMapEntry,
			owner: path,
			key:   k,
			title: k,
			note:  jsondoc.Summary(val),
		})
	}
	return out
}

// toggleCurrent flips a Boolean without opening an editor.
func (a *App) toggleCurrent() tea.Cmd {
	r, ok := a.level().current()
	if !ok || r.kind != rowField {
		return a.openCurrent()
	}
	if !a.editable() {
		return nil
	}
	f := r.field
	if f.Kind != schema.KindBool {
		return a.openCurrent()
	}
	cur, isSet := a.doc().Bool(f.Path)
	switch {
	case !isSet:
		a.set(f.Path, true)
	case cur:
		a.set(f.Path, false)
	default:
		a.unset(f.Path)
	}
	return nil
}

// unsetCurrent removes a key, so Claude Code falls back to its default.
func (a *App) unsetCurrent() tea.Cmd {
	r, ok := a.level().current()
	if !ok {
		return nil
	}
	if !a.editable() {
		return nil
	}
	switch r.kind {
	case rowItem, rowMapEntry:
		return a.deleteEntry()
	case rowField:
		if !a.doc().Has(r.field.Path) {
			a.setStatus(fmt.Sprintf("%s is not set in this file.", r.field.Path), statusInfo)
			return nil
		}
		path := r.field.Path
		a.ov = confirmOverlay("Unset key",
			fmt.Sprintf("Remove %q from %s? Claude Code falls back to: %s", path, a.Context().Name, defaultText(r.field)),
			[]choice{
				{label: "Remove the key", value: "yes", danger: true},
				{label: "Keep it", value: "no"},
			},
			func(app *App, value string) (string, error) {
				if value != "yes" {
					return "", nil
				}
				app.unset(path)
				return fmt.Sprintf("Unset %s", path), nil
			})
	}
	return nil
}

func defaultText(f schema.Field) string {
	if f.Default == "" {
		return "its built-in default"
	}
	return f.Default
}

// addEntry adds an array entry, an object pair, or a settings file.
func (a *App) addEntry() tea.Cmd {
	l := a.level()
	switch {
	case l.listPath != "":
		return a.addListItem(l.listPath)
	case l.mapPath != "":
		return a.addMapEntry(l.mapPath)
	case a.tabIdx == 0:
		return a.openAddContext()
	}
	r, ok := l.current()
	if ok && r.kind == rowField && r.field.Kind == schema.KindStringList {
		if cmd := a.openField(r.field); cmd != nil {
			return cmd
		}
		return a.addListItem(r.field.Path)
	}
	a.setStatus("Nothing to add here. Open a list first, or press c to manage settings files.", statusInfo)
	return nil
}

func (a *App) deleteEntry() tea.Cmd {
	l := a.level()
	r, ok := l.current()
	if !ok || !a.editable() {
		return nil
	}
	switch r.kind {
	case rowItem:
		vals, _ := a.doc().Strings(r.owner)
		if r.index >= len(vals) {
			return nil
		}
		removed := vals[r.index]
		vals = append(vals[:r.index:r.index], vals[r.index+1:]...)
		if err := a.doc().SetStrings(r.owner, vals); err != nil {
			a.setStatus(err.Error(), statusBad)
			return nil
		}
		a.rebuild()
		a.setStatus(fmt.Sprintf("Removed %s", removed), statusGood)
	case rowMapEntry:
		a.unset(r.owner + "." + r.key)
		a.setStatus(fmt.Sprintf("Removed %s", r.key), statusGood)
	default:
		return a.unsetCurrent()
	}
	return nil
}

// moveEntry reorders an array entry, which matters for keys like prependPlugins.
func (a *App) moveEntry(delta int) tea.Cmd {
	l := a.level()
	if l.listPath == "" || l.filter != "" {
		return nil
	}
	r, ok := l.current()
	if !ok || r.kind != rowItem || !a.editable() {
		return nil
	}
	vals, _ := a.doc().Strings(l.listPath)
	to := r.index + delta
	if to < 0 || to >= len(vals) {
		return nil
	}
	vals[r.index], vals[to] = vals[to], vals[r.index]
	if err := a.doc().SetStrings(l.listPath, vals); err != nil {
		a.setStatus(err.Error(), statusBad)
		return nil
	}
	a.rebuild()
	a.level().cursor = to
	return nil
}

func (a *App) editable() bool {
	c := a.Context()
	if c == nil {
		a.setStatus("No settings file selected. Press c to pick one.", statusBad)
		return false
	}
	if !c.Editable() {
		a.setStatus(fmt.Sprintf("%s is read-only.", c.Name), statusBad)
		return false
	}
	return true
}

// set writes a value and refreshes the rows that show it.
func (a *App) set(path string, v any) {
	if err := a.doc().Set(path, v); err != nil {
		a.setStatus(err.Error(), statusBad)
		return
	}
	a.rebuild()
}

func (a *App) unset(path string) {
	a.doc().Delete(path)
	a.rebuild()
}

// validatorFor returns the per-entry check a list needs, if any.
func validatorFor(path string) func(string) error {
	switch path {
	case "permissions.allow", "permissions.ask", "permissions.deny":
		return permissions.Validate
	}
	switch sandbox.KindFor(path) {
	case sandbox.ListPaths:
		return sandbox.ValidatePath
	case sandbox.ListDomains:
		return sandbox.ValidateDomain
	}
	return nil
}

// warnerFor returns the soft advice a list entry can carry.
func warnerFor(path string) func(string) string {
	switch path {
	case "permissions.allow", "permissions.ask", "permissions.deny":
		return permissions.Warning
	}
	switch sandbox.KindFor(path) {
	case sandbox.ListPaths:
		return sandbox.WarnPath
	case sandbox.ListDomains:
		return sandbox.WarnDomain
	}
	return nil
}

func (a *App) addListItem(path string) tea.Cmd {
	if !a.editable() {
		return nil
	}
	f, _ := schema.Lookup(path)
	a.ov = textOverlay(fmt.Sprintf("Add to %s", path), entryHint(path, f), "", validatorFor(path),
		func(app *App, value string) (string, error) {
			vals, _ := app.doc().Strings(path)
			for _, v := range vals {
				if v == value {
					return "", fmt.Errorf("%q is already in the list", value)
				}
			}
			vals = append(vals, value)
			if err := app.doc().SetStrings(path, vals); err != nil {
				return "", err
			}
			app.rebuild()
			msg := fmt.Sprintf("Added %s", value)
			if w := warnerFor(path); w != nil {
				if note := w(value); note != "" {
					msg += " — " + note
				}
			}
			return msg, nil
		})
	a.ov.completions = completionsFor(path)
	return nil
}

func (a *App) editListItem(r row) tea.Cmd {
	if !a.editable() {
		return nil
	}
	f, _ := schema.Lookup(r.owner)
	path, index := r.owner, r.index
	a.ov = textOverlay(fmt.Sprintf("Edit entry %d of %s", index+1, path), entryHint(path, f), r.title, validatorFor(path),
		func(app *App, value string) (string, error) {
			vals, _ := app.doc().Strings(path)
			if index >= len(vals) {
				return "", fmt.Errorf("the entry is gone")
			}
			vals[index] = value
			if err := app.doc().SetStrings(path, vals); err != nil {
				return "", err
			}
			app.rebuild()
			return "Entry updated.", nil
		})
	a.ov.completions = completionsFor(path)
	return nil
}

func completionsFor(path string) []string {
	switch path {
	case "permissions.allow", "permissions.ask", "permissions.deny":
		return permissions.KnownTools()
	}
	if catalog.IsModelField(path) {
		var out []string
		for _, m := range catalog.Models() {
			out = append(out, m.ID)
		}
		return out
	}
	return nil
}

func entryHint(path string, f schema.Field) string {
	switch path {
	case "permissions.allow", "permissions.ask", "permissions.deny":
		return "Tool or Tool(specifier), such as Bash(npm run *) or Read(./.env)"
	}
	switch sandbox.KindFor(path) {
	case sandbox.ListPaths:
		return "/absolute, ~/home-relative, or ./project-relative"
	case sandbox.ListDomains:
		return "host, wildcard or IP, with an optional :port"
	}
	if f.TypeText != "" {
		return f.TypeText
	}
	return "one entry of the list"
}

func (a *App) addMapEntry(path string) tea.Cmd {
	if !a.editable() {
		return nil
	}
	a.ov = textOverlay(fmt.Sprintf("Add a key to %s", path), "name=value", "", func(s string) error {
		if !strings.Contains(s, "=") {
			return fmt.Errorf("write it as name=value")
		}
		if strings.TrimSpace(strings.SplitN(s, "=", 2)[0]) == "" {
			return fmt.Errorf("the name cannot be empty")
		}
		return nil
	}, func(app *App, value string) (string, error) {
		parts := strings.SplitN(value, "=", 2)
		key := strings.TrimSpace(parts[0])
		app.set(path+"."+key, scalarFrom(strings.TrimSpace(parts[1])))
		return fmt.Sprintf("Added %s", key), nil
	})
	return nil
}

func (a *App) editMapEntry(r row) tea.Cmd {
	if !a.editable() {
		return nil
	}
	full := r.owner + "." + r.key
	cur, _ := a.doc().Get(full)
	a.ov = textOverlay(fmt.Sprintf("Edit %s", full), "the value for this key", jsondoc.Summary(cur), nil,
		func(app *App, value string) (string, error) {
			app.set(full, scalarFrom(value))
			return fmt.Sprintf("Set %s", full), nil
		})
	return nil
}

// scalarFrom keeps a Boolean or number typed instead of quoting it.
func scalarFrom(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return json.Number(s)
	}
	return s
}

func (a *App) openBoolPicker(f schema.Field) tea.Cmd {
	cur, isSet := a.doc().Bool(f.Path)
	choices := []choice{
		{label: "true", value: "true", desc: optionDesc(f, "true")},
		{label: "false", value: "false", desc: optionDesc(f, "false")},
		{label: "not set", value: "", desc: "Claude Code uses " + defaultText(f)},
	}
	sel := 2
	if isSet {
		if cur {
			sel = 0
		} else {
			sel = 1
		}
	}
	path := f.Path
	a.ov = choiceOverlay(f.Path, f.Summary, choices, sel, func(app *App, value string) (string, error) {
		switch value {
		case "true":
			app.set(path, true)
		case "false":
			app.set(path, false)
		default:
			app.unset(path)
			return fmt.Sprintf("Unset %s", path), nil
		}
		return fmt.Sprintf("%s = %s", path, value), nil
	})
	return nil
}

// optionDesc finds the reference's note for one accepted value.
func optionDesc(f schema.Field, value string) string {
	for _, o := range f.Options {
		if o.Value == value {
			return o.Desc
		}
	}
	return ""
}

func (a *App) openEnumPicker(f schema.Field) tea.Cmd {
	cur, isSet := a.doc().Str(f.Path)

	var choices []choice
	seen := map[string]bool{}
	addChoice := func(value, desc string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		choices = append(choices, choice{label: value, value: value, desc: desc})
	}
	for _, o := range f.Options {
		addChoice(o.Value, o.Desc)
	}
	if catalog.IsModelField(f.Path) {
		for _, m := range catalog.Models() {
			desc := m.Note
			if desc == "" {
				desc = m.Label
			}
			addChoice(m.ID, desc)
		}
	}
	if isSet {
		addChoice(cur, "the value in this file")
	}
	choices = append(choices,
		choice{label: "Type another value…", value: valueCustom, desc: "anything the key accepts"},
		choice{label: "not set", value: "", desc: "Claude Code uses " + defaultText(f)},
	)

	sel := len(choices) - 1
	for i, c := range choices {
		if isSet && c.value == cur {
			sel = i
			break
		}
	}

	path, summary := f.Path, f.Summary
	a.ov = choiceOverlay(path, summary, choices, sel, func(app *App, value string) (string, error) {
		if value == valueCustom {
			app.ov = textOverlay(path, f.TypeText, cur, nil, func(app *App, v string) (string, error) {
				app.set(path, v)
				return fmt.Sprintf("%s = %s", path, v), nil
			})
			return "", errKeepOverlay
		}
		if value == "" {
			app.unset(path)
			return fmt.Sprintf("Unset %s", path), nil
		}
		app.set(path, value)
		return fmt.Sprintf("%s = %s", path, value), nil
	})
	return nil
}

func (a *App) openStringEditor(f schema.Field) tea.Cmd {
	cur, _ := a.doc().Str(f.Path)
	path := f.Path
	a.ov = textOverlay(path, f.TypeText, cur, nil, func(app *App, value string) (string, error) {
		if value == "" {
			app.unset(path)
			return fmt.Sprintf("Unset %s", path), nil
		}
		app.set(path, value)
		return fmt.Sprintf("%s = %s", path, value), nil
	})
	a.ov.emptyUnsets = true
	return nil
}

func (a *App) openNumberEditor(f schema.Field) tea.Cmd {
	cur := ""
	if n, ok := a.doc().Num(f.Path); ok {
		cur = n.String()
	}
	path := f.Path
	a.ov = textOverlay(path, f.TypeText, cur, func(s string) error {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("%q is not a number", s)
		}
		return nil
	}, func(app *App, value string) (string, error) {
		if strings.TrimSpace(value) == "" {
			app.unset(path)
			return fmt.Sprintf("Unset %s", path), nil
		}
		app.set(path, json.Number(strings.TrimSpace(value)))
		return fmt.Sprintf("%s = %s", path, value), nil
	})
	a.ov.emptyUnsets = true
	return nil
}

func (a *App) openRawEditor(f schema.Field) tea.Cmd {
	cur := ""
	if v, ok := a.doc().Get(f.Path); ok {
		cur = jsondoc.Render(v)
	} else if tmpl := rawTemplate(f); tmpl != "" {
		cur = tmpl
	}
	path := f.Path
	a.ov = jsonOverlay(path, f.TypeText, cur, func(app *App, value string) (string, error) {
		if strings.TrimSpace(value) == "" {
			app.unset(path)
			return fmt.Sprintf("Unset %s", path), nil
		}
		if err := app.doc().SetRaw(path, value); err != nil {
			return "", err
		}
		app.rebuild()
		return fmt.Sprintf("Set %s", path), nil
	})
	return nil
}

// rawTemplate seeds the JSON editor for keys whose shape is easy to get wrong.
func rawTemplate(f schema.Field) string {
	switch f.Path {
	case "hooks":
		return "{\n  \"PreToolUse\": [\n    {\n      \"matcher\": \"Bash\",\n      \"hooks\": [\n        { \"type\": \"command\", \"command\": \"\" }\n      ]\n    }\n  ]\n}"
	case "statusLine", "subagentStatusLine":
		return "{\n  \"type\": \"command\",\n  \"command\": \"\"\n}"
	case "env", "modelOverrides", "vimInsertModeRemaps", "enabledPlugins":
		return "{\n  \n}"
	}
	if strings.HasPrefix(strings.ToLower(f.TypeText), "array") {
		return "[\n  \n]"
	}
	if strings.HasPrefix(strings.ToLower(f.TypeText), "object") {
		return "{\n  \n}"
	}
	return ""
}

func (a *App) openFilter() {
	l := a.level()
	a.ov = textOverlay("Filter "+l.title, "narrows the list below", l.filter, nil,
		func(app *App, value string) (string, error) {
			lv := app.level()
			lv.filter = value
			lv.cursor = 0
			if value != "" && len(lv.visible()) == 0 {
				return fmt.Sprintf("Nothing matches %q.", value), nil
			}
			return "", nil
		})
	a.ov.live = func(app *App, value string) {
		lv := app.level()
		lv.filter = value
		lv.cursor = 0
	}
}

// openSearch jumps to any key in the reference, from any tab.
func (a *App) openSearch() {
	a.ov = textOverlay("Find a setting", "type part of a key name or its description", "", nil,
		func(app *App, value string) (string, error) {
			hits := schema.Search(value)
			if len(hits) == 0 {
				return "", fmt.Errorf("no key matches %q", value)
			}
			app.jumpTo(hits[0])
			return "", nil
		})
	a.ov.live = func(app *App, value string) {
		hits := schema.Search(value)
		if len(hits) > 8 {
			hits = hits[:8]
		}
		app.ov.results = hits
	}
	a.ov.onResult = func(app *App, f schema.Field) (string, error) {
		app.jumpTo(f)
		return "", nil
	}
}

// jumpTo opens the tab and level that holds a key, and puts the cursor on it.
func (a *App) jumpTo(f schema.Field) {
	a.tabIdx = tabOfField(f)
	root := a.rootLevel(a.tabIdx)
	a.stacks[a.tabIdx] = []level{root}

	// Open the section for the category when the tab groups several.
	if len(tabs[a.tabIdx].Cats) > 1 {
		for i, r := range root.rows {
			if r.kind == rowSection && len(r.cats) == 1 && r.cats[0] == f.Category {
				a.level().cursor = i
				a.push(a.sectionLevel(r))
				break
			}
		}
	}
	// Drill through the parent objects of a nested key.
	if parent := f.Parent(); parent != "" {
		if pf, ok := schema.Lookup(parent); ok {
			if a.focusRow(pf.Path) {
				a.push(level{title: pf.Path, crumb: pf.Name, rows: a.objectRows(pf)})
			}
		}
	}
	if !a.focusRow(f.Path) {
		a.setStatus(fmt.Sprintf("%s is not shown for this file; it belongs in %s.", f.Path, f.Scope), statusInfo)
		return
	}
	a.setStatus(f.Path, statusInfo)
}

// focusRow moves the cursor to the row for a settings path at the current level.
func (a *App) focusRow(path string) bool {
	l := a.level()
	l.filter = ""
	for i, r := range l.visible() {
		if r.kind == rowField && r.field.Path == path {
			l.cursor = i
			return true
		}
	}
	return false
}

func (a *App) openAddContext() tea.Cmd {
	a.ov = addContextOverlay()
	return nil
}

func addContextOverlay() *overlay {
	ov := textOverlay("Add a settings file", "path to a settings.json, ~ allowed", "", func(s string) error {
		_, err := config.ExpandPath(s)
		return err
	}, func(app *App, value string) (string, error) {
		c, err := config.NewContext(value)
		if err != nil {
			return "", err
		}
		for i := range app.ctxs {
			if app.ctxs[i].Path == c.Path {
				app.files.cursor = i
				return fmt.Sprintf("Already in the list: %s", c.Path), nil
			}
		}
		if err := config.Remember(c.Path); err != nil {
			return "", err
		}
		app.ctxs = append(app.ctxs, c)
		app.refreshFiles()
		app.files.cursor = len(app.ctxs) - 1
		if c.LoadErr != nil {
			return "", fmt.Errorf("%s: %w", c.Path, c.LoadErr)
		}
		if !c.Exists {
			return fmt.Sprintf("Added %s; it will be created when you save.", c.Path), nil
		}
		return fmt.Sprintf("Added %s", c.Path), nil
	})
	return ov
}
