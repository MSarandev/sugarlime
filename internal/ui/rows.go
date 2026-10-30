package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/schema"
)

type rowKind int

const (
	rowField    rowKind = iota // a settings key
	rowSection                 // drills into another level
	rowItem                    // one entry of a string array
	rowMapEntry                // one key of a flat object
	rowAction                  // runs something, e.g. "add a settings file"
)

// row is one line in the left pane.
type row struct {
	kind  rowKind
	field schema.Field

	title string
	note  string

	// section
	cats  []string
	paths []string

	// rowItem / rowMapEntry
	owner string // settings path of the list or object being edited
	index int
	key   string

	action string
}

// level is one screen of rows, with the cursor state that belongs to it.
type level struct {
	title    string
	crumb    string
	rows     []row
	cursor   int
	offset   int
	filter   string
	listPath string // set when the level edits a string array
	mapPath  string // set when the level edits a flat object
}

func (l *level) visible() []row {
	if l.filter == "" {
		return l.rows
	}
	q := strings.ToLower(l.filter)
	var out []row
	for _, r := range l.rows {
		if strings.Contains(strings.ToLower(r.label()), q) ||
			strings.Contains(strings.ToLower(r.field.Summary), q) ||
			strings.Contains(strings.ToLower(r.field.Path), q) {
			out = append(out, r)
		}
	}
	return out
}

func (l *level) current() (row, bool) {
	rows := l.visible()
	if l.cursor < 0 || l.cursor >= len(rows) {
		return row{}, false
	}
	return rows[l.cursor], true
}

func (r row) label() string { return r.title }

// drills reports whether opening the row moves to another level.
func (r row) drills() bool {
	switch r.kind {
	case rowSection:
		return true
	case rowField:
		return r.field.Kind == schema.KindObject || r.field.Kind == schema.KindStringList
	default:
		return false
	}
}

// fieldRows labels each key by its last path segment; the breadcrumb gives the rest.
func fieldRows(fields []schema.Field) []row {
	out := make([]row, 0, len(fields))
	for _, f := range fields {
		out = append(out, row{kind: rowField, field: f, title: f.Name})
	}
	return out
}

// unknownFields makes rows for keys the reference does not describe.
func unknownFields(doc *jsondoc.Object) []row {
	var out []row
	for _, k := range doc.Keys() {
		if _, ok := schema.Lookup(k); ok {
			continue
		}
		v, _ := doc.Get(k)
		out = append(out, row{
			kind: rowField,
			field: schema.Field{
				Path:     k,
				Name:     k,
				Category: "Not in the reference",
				Summary:  "Not described in the settings reference bundled with sugarlime.",
				Kind:     kindOfValue(v),
			},
			title: k,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].field.Path < out[j].field.Path })
	return out
}

// kindOfValue guesses an editor for a value with no schema entry.
func kindOfValue(v any) schema.Kind {
	switch t := v.(type) {
	case bool:
		return schema.KindBool
	case string:
		return schema.KindString
	case []any:
		for _, e := range t {
			if _, ok := e.(string); !ok {
				return schema.KindRaw
			}
		}
		return schema.KindStringList
	default:
		return schema.KindRaw
	}
}

// setCount reports how many of a category's keys the document sets.
func setCount(doc *jsondoc.Object, category string) (set, total int) {
	for _, f := range schema.All() {
		if f.Category != category {
			continue
		}
		if f.Kind == schema.KindObject {
			continue // counted through its children
		}
		total++
		if doc.Has(f.Path) {
			set++
		}
	}
	return set, total
}

func sectionRow(title string, cats []string, note string) row {
	return row{kind: rowSection, title: title, cats: cats, note: note}
}

func countNote(set, total int) string {
	if set == 0 {
		return fmt.Sprintf("none of %d set", total)
	}
	return fmt.Sprintf("%d of %d set", set, total)
}
