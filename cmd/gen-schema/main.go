// Command gen-schema turns the Claude Code settings reference
// (settings_values.md) into internal/schema/registry_gen.go. Run it with
// `make gen` after refreshing the reference.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type entry struct {
	Path     string
	Name     string
	Category string
	Summary  string
	Doc      string
	Scope    string
	TypeText string
	Default  string
	Notice   string
	Kind     string
	Options  []option
	Deprec   bool
}

type option struct {
	Value string
	Desc  string
}

func main() {
	in := flag.String("in", "settings_values.md", "settings reference markdown")
	out := flag.String("out", "internal/schema/registry_gen.go", "generated Go file")
	flag.Parse()

	src, err := os.ReadFile(*in)
	if err != nil {
		fail(err)
	}
	entries, categories, err := parse(string(src))
	if err != nil {
		fail(err)
	}
	code, err := render(entries, categories, *in)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, code, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s: %d keys across %d categories\n", *out, len(entries), len(categories))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-schema:", err)
	os.Exit(1)
}

var (
	indexRow   = regexp.MustCompile(`^\|\s*\[` + "`" + `([^` + "`" + `]+)` + "`" + `\]\([^)]*\)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*$`)
	headingH2  = regexp.MustCompile(`^## (.+)$`)
	headingH3  = regexp.MustCompile("^### `([^`]+)`$")
	bulletType = regexp.MustCompile(`^\* \*\*Type\*\*:\s*(.*)$`)
	bulletDef  = regexp.MustCompile(`^\* \*\*Default\*\*:\s*(.*)$`)
	bulletAny  = regexp.MustCompile(`^\* \*\*[A-Za-z ]+\*\*:`)
	nestedOpt  = regexp.MustCompile("^\\s+\\* `(?:\"([^\"]*)\"|([^`]+))`(?::\\s*(.*))?$")
	mdLink     = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	quoted     = regexp.MustCompile("`\"([^\"]*)\"`")
)

func parse(src string) ([]entry, []string, error) {
	type meta struct{ summary, category, scope string }
	index := map[string]meta{}
	var categories []string
	seenCategory := map[string]bool{}

	byPath := map[string]*entry{}
	var order []*entry

	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		section     string // current H2
		inIndex     bool
		cur         *entry
		inOpts      bool
		docDone     bool
		docLines    []string
		noticeLines []string
	)

	flush := func() {
		if cur == nil {
			return
		}
		cur.Doc = clean(strings.Join(docLines, " "))
		cur.Notice = clean(strings.Join(noticeLines, " "))
		cur.Deprec = isDeprecated(cur)
		order = append(order, cur)
		byPath[cur.Path] = cur
		cur, docLines, noticeLines, inOpts, docDone = nil, nil, nil, false, false
	}

	for sc.Scan() {
		line := sc.Text()

		if m := headingH2.FindStringSubmatch(line); m != nil {
			flush()
			section = strings.TrimSpace(m[1])
			inIndex = section == "Settings index"
			if section != "Settings index" && section != "See also" && !seenCategory[section] {
				seenCategory[section] = true
				categories = append(categories, section)
			}
			continue
		}

		if inIndex {
			if m := indexRow.FindStringSubmatch(line); m != nil {
				index[m[1]] = meta{
					summary:  clean(m[2]),
					category: clean(m[3]),
					scope:    clean(m[4]),
				}
			}
			continue
		}

		if m := headingH3.FindStringSubmatch(line); m != nil {
			flush()
			key := m[1]
			cur = &entry{Path: key, Name: lastSegment(key), Category: section}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "#### ") {
			inOpts = false
			docDone = true
			continue
		}

		if strings.HasPrefix(line, ">") {
			t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">"))
			if t != "" && t != "[!WARNING]" && t != "[!NOTE]" && t != "[!IMPORTANT]" {
				noticeLines = append(noticeLines, t)
			}
			continue
		}

		switch {
		case bulletType.MatchString(line):
			cur.TypeText = clean(bulletType.FindStringSubmatch(line)[1])
			inOpts = true
			docDone = true
		case bulletDef.MatchString(line):
			cur.Default = clean(bulletDef.FindStringSubmatch(line)[1])
			inOpts = false
			docDone = true
		case bulletAny.MatchString(line):
			inOpts = false
			docDone = true
		case inOpts && nestedOpt.MatchString(line):
			m := nestedOpt.FindStringSubmatch(line)
			val := m[1]
			if val == "" {
				val = strings.TrimSpace(m[2])
			}
			cur.Options = append(cur.Options, option{Value: val, Desc: clean(m[3])})
		case strings.HasPrefix(line, "```"):
			inOpts = false
			docDone = true
		case strings.TrimSpace(line) == "":
			if len(docLines) > 0 {
				docDone = true
			}
		default:
			if !docDone && !strings.HasPrefix(line, "*") {
				docLines = append(docLines, strings.TrimSpace(line))
			}
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}

	// Keys that appear only in the index still deserve an entry.
	for key, m := range index {
		if _, ok := byPath[key]; ok {
			continue
		}
		e := &entry{Path: key, Name: lastSegment(key), Category: m.category}
		order = append(order, e)
		byPath[key] = e
	}

	for _, e := range order {
		if m, ok := index[e.Path]; ok {
			e.Summary = m.summary
			e.Scope = m.scope
			if e.Category == "" {
				e.Category = m.category
			}
		}
		if e.Summary == "" {
			e.Summary = firstSentence(e.Doc)
		}
	}

	// Kind inference needs to know which keys have children.
	hasChild := map[string]bool{}
	for _, e := range order {
		if p := parentOf(e.Path); p != "" {
			hasChild[p] = true
		}
	}
	for _, e := range order {
		e.Kind = inferKind(e, hasChild[e.Path])
		if e.Kind != "KindEnum" {
			e.Options = nil
		} else if len(e.Options) == 0 {
			e.Options = inlineOptions(e.TypeText)
		}
		if e.Kind == "KindEnum" && len(e.Options) == 0 {
			e.Kind = "KindString"
		}
	}

	// Reference order within a category reads better than alphabetical.
	sort.SliceStable(order, func(i, j int) bool {
		return categoryIndex(categories, order[i].Category) < categoryIndex(categories, order[j].Category)
	})

	out := make([]entry, 0, len(order))
	for _, e := range order {
		out = append(out, *e)
	}
	return out, categories, nil
}

func categoryIndex(categories []string, name string) int {
	for i, c := range categories {
		if c == name {
			return i
		}
	}
	return len(categories)
}

// isDeprecated spots keys the reference marks as removed or superseded.
func isDeprecated(e *entry) bool {
	for _, s := range []string{e.Notice, e.Summary, e.Doc} {
		l := strings.ToLower(s)
		if strings.Contains(l, "removed in v") || strings.HasPrefix(l, "deprecated") || strings.Contains(l, "; deprecated") {
			return true
		}
	}
	return false
}

func inferKind(e *entry, hasChild bool) string {
	t := strings.ToLower(e.TypeText)
	switch {
	case hasChild:
		return "KindObject"
	case strings.HasPrefix(t, "boolean"):
		return "KindBool"
	case strings.HasPrefix(t, "array of object"), strings.HasPrefix(t, "array of table"):
		return "KindRaw"
	case strings.HasPrefix(t, "array"):
		return "KindStringList"
	case strings.HasPrefix(t, "object"), strings.HasPrefix(t, "map"):
		return "KindRaw"
	case strings.HasPrefix(t, "number"), strings.HasPrefix(t, "integer"), strings.HasPrefix(t, "float"):
		return "KindNumber"
	case strings.Contains(t, "one of"), strings.HasPrefix(t, "the string "):
		return "KindEnum"
	case len(e.Options) >= 2 && allQuotedValues(e.Options):
		return "KindEnum"
	case t == "":
		return "KindRaw"
	default:
		return "KindString"
	}
}

func allQuotedValues(opts []option) bool {
	for _, o := range opts {
		if o.Value == "" || strings.ContainsAny(o.Value, " ") {
			return false
		}
	}
	return true
}

// inlineOptions reads the values out of `string, one of "a", "b", or "c"`.
func inlineOptions(typeText string) []option {
	var opts []option
	seen := map[string]bool{}
	for _, m := range quoted.FindAllStringSubmatch(reinsertTicks(typeText), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		opts = append(opts, option{Value: m[1]})
	}
	return opts
}

// clean strips markdown links and code ticks so the text reads well in a terminal.
func clean(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	return strings.TrimSpace(s)
}

// reinsertTicks restores the ticks clean() stripped, for enum extraction.
func reinsertTicks(s string) string {
	return regexp.MustCompile(`"([^"]*)"`).ReplaceAllString(s, "`\"$1\"`")
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}
	return path
}

func parentOf(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[:i]
	}
	return ""
}

func render(entries []entry, categories []string, source string) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by cmd/gen-schema from %s. DO NOT EDIT.\n\n", source)
	buf.WriteString("package schema\n\n")

	buf.WriteString("var categoryOrder = []string{\n")
	for _, c := range categories {
		fmt.Fprintf(&buf, "\t%q,\n", c)
	}
	buf.WriteString("}\n\n")

	buf.WriteString("var fields = []Field{\n")
	for _, e := range entries {
		buf.WriteString("\t{\n")
		fmt.Fprintf(&buf, "\t\tPath: %q,\n", e.Path)
		fmt.Fprintf(&buf, "\t\tName: %q,\n", e.Name)
		fmt.Fprintf(&buf, "\t\tCategory: %q,\n", e.Category)
		fmt.Fprintf(&buf, "\t\tSummary: %q,\n", e.Summary)
		fmt.Fprintf(&buf, "\t\tDoc: %q,\n", e.Doc)
		fmt.Fprintf(&buf, "\t\tScope: %q,\n", e.Scope)
		fmt.Fprintf(&buf, "\t\tTypeText: %q,\n", e.TypeText)
		fmt.Fprintf(&buf, "\t\tDefault: %q,\n", e.Default)
		if e.Notice != "" {
			fmt.Fprintf(&buf, "\t\tNotice: %q,\n", e.Notice)
		}
		if e.Deprec {
			buf.WriteString("\t\tDeprecated: true,\n")
		}
		fmt.Fprintf(&buf, "\t\tKind: %s,\n", e.Kind)
		if len(e.Options) > 0 {
			buf.WriteString("\t\tOptions: []Option{\n")
			for _, o := range e.Options {
				fmt.Fprintf(&buf, "\t\t\t{Value: %q, Desc: %q},\n", o.Value, o.Desc)
			}
			buf.WriteString("\t\t},\n")
		}
		buf.WriteString("\t},\n")
	}
	buf.WriteString("}\n")

	return format.Source(buf.Bytes())
}

// readReference loads the bundled reference, relative to this package.
func readReference() (string, error) {
	data, err := os.ReadFile(filepath.Join("..", "..", "settings_values.md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
