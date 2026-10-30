package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/schema"
)

func TestScalarFrom(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"true", true},
		{"false", false},
		{"null", nil},
		{"12", json.Number("12")},
		{"1.5", json.Number("1.5")},
		{"vim", "vim"},
		{"", ""},
	}
	for _, c := range cases {
		if got := scalarFrom(c.in); got != c.want {
			t.Errorf("scalarFrom(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestKindOfValue(t *testing.T) {
	doc, err := jsondoc.Parse([]byte(`{"b":true,"s":"x","list":["a"],"mixed":[1],"n":2,"obj":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]schema.Kind{
		"b":     schema.KindBool,
		"s":     schema.KindString,
		"list":  schema.KindStringList,
		"mixed": schema.KindRaw,
		"n":     schema.KindRaw,
		"obj":   schema.KindRaw,
	}
	for key, want := range cases {
		v, _ := doc.Get(key)
		if got := kindOfValue(v); got != want {
			t.Errorf("%s: got %q, want %q", key, got, want)
		}
	}
}

func TestFlatObject(t *testing.T) {
	flat, _ := jsondoc.Parse([]byte(`{"a":{"x":"1","y":true,"z":null}}`))
	v, _ := flat.Get("a")
	if !flatObject(v.(*jsondoc.Object)) {
		t.Error("an object of scalars is flat")
	}

	nested, _ := jsondoc.Parse([]byte(`{"a":{"x":{"deep":1}}}`))
	v, _ = nested.Get("a")
	if flatObject(v.(*jsondoc.Object)) {
		t.Error("an object holding an object is not flat")
	}

	if flatObject(jsondoc.New()) {
		t.Error("an empty object has no pairs to edit")
	}
}

func TestMapShaped(t *testing.T) {
	env, _ := schema.Lookup("env")
	if !mapShaped(env) {
		t.Errorf("env maps names to strings: %q", env.TypeText)
	}
	hooks, _ := schema.Lookup("hooks")
	if mapShaped(hooks) {
		t.Error("hooks is not a simple mapping")
	}
}

func TestValidatorAndWarnerSelection(t *testing.T) {
	for _, path := range []string{"permissions.allow", "permissions.ask", "permissions.deny"} {
		if validatorFor(path) == nil || warnerFor(path) == nil {
			t.Errorf("%s needs rule checking", path)
		}
	}
	for _, path := range []string{"sandbox.filesystem.denyRead", "sandbox.network.allowedDomains"} {
		if validatorFor(path) == nil || warnerFor(path) == nil {
			t.Errorf("%s needs sandbox checking", path)
		}
	}
	if validatorFor("availableModels") != nil || warnerFor("availableModels") != nil {
		t.Error("an ordinary list needs no per-entry check")
	}

	check := validatorFor("permissions.allow")
	if err := check("Bash("); err == nil {
		t.Error("expected the rule validator")
	}
	check = validatorFor("sandbox.network.allowedDomains")
	if err := check("https://x.com"); err == nil {
		t.Error("expected the domain validator")
	}
}

func TestCompletionsFor(t *testing.T) {
	if got := completionsFor("permissions.allow"); len(got) == 0 {
		t.Error("rule editing offers tool names")
	}
	if got := completionsFor("availableModels"); len(got) == 0 {
		t.Error("model lists offer the catalog")
	}
	if got := completionsFor("sandbox.excludedCommands"); got != nil {
		t.Errorf("got %v", got)
	}
}

func TestEntryHint(t *testing.T) {
	allow, _ := schema.Lookup("permissions.allow")
	if !strings.Contains(entryHint("permissions.allow", allow), "Tool(specifier)") {
		t.Error("rule entries explain the syntax")
	}
	paths, _ := schema.Lookup("sandbox.filesystem.denyRead")
	if !strings.Contains(entryHint("sandbox.filesystem.denyRead", paths), "project-relative") {
		t.Error("path entries explain the prefixes")
	}
	domains, _ := schema.Lookup("sandbox.network.allowedDomains")
	if !strings.Contains(entryHint("sandbox.network.allowedDomains", domains), "port") {
		t.Error("domain entries mention the port suffix")
	}
	models, _ := schema.Lookup("availableModels")
	if got := entryHint("availableModels", models); got != models.TypeText {
		t.Errorf("got %q", got)
	}
	if got := entryHint("whatever", schema.Field{}); got != "one entry of the list" {
		t.Errorf("got %q", got)
	}
}

func TestRawTemplate(t *testing.T) {
	hooks, _ := schema.Lookup("hooks")
	if !strings.Contains(rawTemplate(hooks), "PreToolUse") {
		t.Error("the hooks editor should start from a usable shape")
	}
	status, _ := schema.Lookup("statusLine")
	if !strings.Contains(rawTemplate(status), `"type": "command"`) {
		t.Error("the status line editor should start from a usable shape")
	}
	arr, _ := schema.Lookup("sshConfigs")
	if got := rawTemplate(arr); !strings.HasPrefix(got, "[") {
		t.Errorf("an array key should start from brackets, got %q", got)
	}
	obj, _ := schema.Lookup("modelPricing")
	if got := rawTemplate(obj); !strings.HasPrefix(got, "{") {
		t.Errorf("an object key should start from braces, got %q", got)
	}
	if got := rawTemplate(schema.Field{Path: "x"}); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestEditHintCoversEveryKind(t *testing.T) {
	kinds := []schema.Kind{
		schema.KindBool, schema.KindEnum, schema.KindStringList,
		schema.KindObject, schema.KindRaw, schema.KindNumber, schema.KindString,
	}
	seen := map[string]bool{}
	for _, k := range kinds {
		hint := editHint(schema.Field{Kind: k})
		if hint == "" {
			t.Errorf("%q has no hint", k)
		}
		seen[hint] = true
	}
	if len(seen) != len(kinds) {
		t.Errorf("expected a distinct hint per kind, got %d", len(seen))
	}
}

func TestOptionDescAndDefaultText(t *testing.T) {
	mode, _ := schema.Lookup("permissions.defaultMode")
	if optionDesc(mode, "plan") == "" {
		t.Error("expected the reference's note for plan")
	}
	if optionDesc(mode, "nope") != "" {
		t.Error("an unknown value has no note")
	}
	if got := defaultText(schema.Field{}); got != "its built-in default" {
		t.Errorf("got %q", got)
	}
	if got := defaultText(schema.Field{Default: "false"}); got != "false" {
		t.Errorf("got %q", got)
	}
}

func TestKindDocCoversEveryKind(t *testing.T) {
	for _, k := range []config.Kind{
		config.KindUser, config.KindProject, config.KindLocal,
		config.KindManaged, config.KindGlobal, config.KindCustom,
	} {
		if kindDoc(k) == "" {
			t.Errorf("%v has no description", k)
		}
	}
}

func TestShortenPath(t *testing.T) {
	long := "/Users/someone/projects/thing/.claude/settings.json"
	if got := shortenPath(long, 80); got != long {
		t.Errorf("a path that fits is left alone, got %q", got)
	}
	got := shortenPath(long, 30)
	if !strings.HasPrefix(got, ".../") || !strings.HasSuffix(got, "settings.json") {
		t.Errorf("got %q", got)
	}
	if got := shortenPath(long, 5); got != "/Use…" {
		t.Errorf("too little room to shorten, got %q", got)
	}
	if got := shortenPath(long, 0); got != long {
		t.Errorf("got %q", got)
	}
	if got := shortenPath("settings.json", 8); got != "setting…" {
		t.Errorf("got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "…"},
		{"hello", 0, ""},
		{"hello", -3, ""},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.width); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}

func TestPad(t *testing.T) {
	if got := pad("a\nb", 4); got != "a\nb\n\n" {
		t.Errorf("got %q", got)
	}
	if got := pad("a\nb\nc", 2); got != "a\nb" {
		t.Errorf("got %q", got)
	}
}

func TestOneLine(t *testing.T) {
	if got := oneLine("a\n  b   c\n"); got != "a b c" {
		t.Errorf("got %q", got)
	}
}

func TestHangIndentsContinuationLines(t *testing.T) {
	got := hang(strings.Repeat("word ", 20), 30, 4)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping, got %q", got)
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, "    ") {
			t.Errorf("line %d is not indented: %q", i, l)
		}
	}
}

func TestWrapTooNarrow(t *testing.T) {
	if got := wrap("abc", 1); got != "abc" {
		t.Errorf("got %q", got)
	}
}

func TestCountNote(t *testing.T) {
	if got := countNote(0, 5); got != "none of 5 set" {
		t.Errorf("got %q", got)
	}
	if got := countNote(2, 5); got != "2 of 5 set" {
		t.Errorf("got %q", got)
	}
}

func TestClamp(t *testing.T) {
	if clamp(5, 0, 3) != 3 || clamp(-1, 0, 3) != 0 || clamp(2, 0, 3) != 2 {
		t.Error("clamp is wrong")
	}
}

func TestTabOfCategory(t *testing.T) {
	if tabs[tabOfCategory("Permission settings")].Title != "Permissions" {
		t.Error("permission keys belong on the Permissions tab")
	}
	if tabs[tabOfCategory("Git and attribution")].Title != "Interface" {
		t.Error("attribution keys are grouped under Interface")
	}
	if tabs[tabOfCategory("Tools")].Title != "More" {
		t.Error("the remaining categories live under More")
	}
	if tabOfCategory("No Such Category") != 0 {
		t.Error("an unknown category falls back to the first tab")
	}
}

func TestEveryCategoryHasATab(t *testing.T) {
	for _, c := range schema.Categories() {
		found := false
		for _, tb := range tabs {
			for _, tc := range tb.Cats {
				if tc == c {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("category %q is not shown on any tab", c)
		}
	}
}

func TestHasDuplicateLabels(t *testing.T) {
	if hasDuplicateLabels([]row{{title: "a"}, {title: "b"}}) {
		t.Error("no duplicates here")
	}
	if !hasDuplicateLabels([]row{{title: "a"}, {title: "a"}}) {
		t.Error("expected the duplicate to be spotted")
	}
}

func TestRowDrills(t *testing.T) {
	cases := []struct {
		r    row
		want bool
	}{
		{row{kind: rowSection}, true},
		{row{kind: rowField, field: schema.Field{Kind: schema.KindObject}}, true},
		{row{kind: rowField, field: schema.Field{Kind: schema.KindStringList}}, true},
		{row{kind: rowField, field: schema.Field{Kind: schema.KindBool}}, false},
		{row{kind: rowItem}, false},
	}
	for _, c := range cases {
		if got := c.r.drills(); got != c.want {
			t.Errorf("%+v: got %v", c.r, got)
		}
	}
}
