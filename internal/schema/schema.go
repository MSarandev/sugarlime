// Package schema describes every key Claude Code reads from a settings file.
// The registry in registry_gen.go is generated from settings_values.md by
// cmd/gen-schema; everything here is hand-written behaviour on top of it.
package schema

import (
	"sort"
	"strings"
)

// Kind decides which editor the UI opens for a field.
type Kind string

const (
	KindString     Kind = "string"
	KindBool       Kind = "bool"
	KindEnum       Kind = "enum"
	KindNumber     Kind = "number"
	KindStringList Kind = "stringlist"
	KindObject     Kind = "object"
	KindRaw        Kind = "raw"
)

// Option is one accepted value of an enum field.
type Option struct {
	Value string
	Desc  string
}

// Field is one settings key.
type Field struct {
	Path     string // dotted path, e.g. "permissions.allow"
	Name     string // last path segment
	Category string
	Summary  string // one line, from the settings index
	Doc      string // opening paragraph of the key's entry
	Scope    string
	TypeText string
	Default  string
	Notice   string // a removal or caveat the reference calls out
	Kind     Kind
	Options  []Option

	// Deprecated marks a key the reference says was removed or superseded.
	Deprecated bool
}

// GlobalConfig reports whether the key belongs in ~/.claude.json rather than a settings file.
func (f Field) GlobalConfig() bool {
	return f.Scope == "Global config"
}

// Managed reports whether Claude Code only honours the key from managed settings.
func (f Field) Managed() bool {
	return f.Scope == "Managed"
}

// Depth is how many levels deep the key nests.
func (f Field) Depth() int {
	return strings.Count(f.Path, ".")
}

// Parent returns the dotted path of the enclosing key, or "" at the top level.
func (f Field) Parent() string {
	i := strings.LastIndex(f.Path, ".")
	if i < 0 {
		return ""
	}
	return f.Path[:i]
}

// OptionValues lists an enum field's accepted values.
func (f Field) OptionValues() []string {
	out := make([]string, len(f.Options))
	for i, o := range f.Options {
		out[i] = o.Value
	}
	return out
}

var byPath = func() map[string]Field {
	m := make(map[string]Field, len(fields))
	for _, f := range fields {
		m[f.Path] = f
	}
	return m
}()

// All returns every field, in registry order.
func All() []Field { return fields }

// Lookup finds a field by dotted path.
func Lookup(path string) (Field, bool) {
	f, ok := byPath[path]
	return f, ok
}

// Categories returns the category names in documentation order.
func Categories() []string { return categoryOrder }

// Roots returns a category's fields that do not nest under another key.
func Roots(category string) []Field {
	var out []Field
	for _, f := range fields {
		if f.Category != category {
			continue
		}
		if p := f.Parent(); p != "" {
			if _, nested := byPath[p]; nested {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// Children returns the direct children of an object field.
func Children(path string) []Field {
	prefix := path + "."
	var out []Field
	for _, f := range fields {
		if !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		if strings.Contains(strings.TrimPrefix(f.Path, prefix), ".") {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Search ranks fields matching a free-text query against path, summary and doc.
func Search(query string) []Field {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	type scored struct {
		f     Field
		score int
	}
	var hits []scored
	for _, f := range fields {
		path := strings.ToLower(f.Path)
		name := strings.ToLower(f.Name)
		switch {
		case path == q:
			hits = append(hits, scored{f, 0})
		case strings.HasPrefix(name, q):
			hits = append(hits, scored{f, 1})
		case strings.Contains(path, q):
			hits = append(hits, scored{f, 2})
		case strings.Contains(strings.ToLower(f.Summary), q):
			hits = append(hits, scored{f, 3})
		case strings.Contains(strings.ToLower(f.Doc), q):
			hits = append(hits, scored{f, 4})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score < hits[j].score
		}
		return hits[i].f.Path < hits[j].f.Path
	})
	out := make([]Field, len(hits))
	for i, h := range hits {
		out[i] = h.f
	}
	return out
}
