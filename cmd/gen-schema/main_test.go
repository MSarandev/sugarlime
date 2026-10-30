package main

import (
	"strings"
	"testing"
)

const doc = "" +
	"## Settings index\n\n" +
	"| Key | Description | Topic | Scope |\n" +
	"| :- | :- | :- | :- |\n" +
	"| [`model`](#model) | Set the model new sessions use | Model and responses | Any file |\n" +
	"| [`permissions.allow`](#permissions-allow) | Approve listed tool uses | Permission settings | Any file |\n" +
	"| [`permissions`](#permissions) | Rules and the starting mode | Permission settings | Any file |\n" +
	"| [`sandbox.enabled`](#sandbox-enabled) | Turn on Bash sandboxing | Sandbox settings | Any file |\n" +
	"| [`indexOnly`](#index-only) | A key with no entry of its own | Sandbox settings | Managed |\n" +
	"| [`oldKey`](#oldkey) | Deprecated; use newKey | Sandbox settings | Any file |\n" +
	"| [`sandbox.network.httpProxyPort`](#p) | Pick the proxy port | Sandbox settings | Any file |\n" +
	"| [`env`](#env) | Set environment variables | Memory and context | Any file |\n\n" +
	"## Model and responses\n\n" +
	"Intro prose for the group.\n\n" +
	"### `model`\n\n" +
	"Set the model every new session uses, so you don't have to [pick one](https://x) each time.\n\n" +
	"* **Scope**: [`Any file`](#scopes)\n" +
	"* **Type**: string, a model alias or full model ID\n" +
	"* **Default**: unset\n\n" +
	"```json settings.json\n{\n  \"model\": \"sonnet\"\n}\n```\n\n" +
	"## Permission settings\n\n" +
	"### `permissions`\n\n" +
	"Control which tools Claude can use.\n\n" +
	"* **Type**: object with `allow`, `ask`, and `deny`\n" +
	"* **Default**: unset\n\n" +
	"### `permissions.allow`\n\n" +
	"List the tool uses Claude Code approves without asking.\n\n" +
	"* **Type**: array of permission rule strings\n" +
	"* **Default**: unset\n\n" +
	"#### Permission rule syntax\n\n" +
	"Rules follow the format `Tool` or `Tool(specifier)`.\n\n" +
	"## Sandbox settings\n\n" +
	"### `sandbox.enabled`\n\n" +
	"Turn on sandboxing for Bash commands.\n\n" +
	"* **Type**: Boolean\n" +
	"  * `true`: commands run sandboxed\n" +
	"  * `false`: commands run as they do now\n" +
	"* **Default**: `false`\n\n" +
	"### `sandbox.network.httpProxyPort`\n\n" +
	"Pick the port the proxy listens on.\n\n" +
	"* **Type**: number, a local TCP port\n" +
	"* **Default**: a free port\n\n" +
	"### `oldKey`\n\n" +
	"> [!WARNING]\n" +
	">\n" +
	"> Removed in v2.1.277, together with the tool it sized.\n\n" +
	"Through v2.1.276 this did something.\n\n" +
	"### `mode`\n\n" +
	"A key with inline accepted values.\n\n" +
	"* **Type**: string, one of `\"notify\"`, `\"quiet\"`, or `\"off\"`\n" +
	"* **Default**: `\"notify\"`\n\n" +
	"## Memory and context\n\n" +
	"### `env`\n\n" +
	"Set environment variables for every session.\n\n" +
	"* **Type**: object mapping variable names to string values\n" +
	"* **Default**: unset\n\n" +
	"## See also\n\nLinks.\n"

func parseDoc(t *testing.T) (map[string]entry, []string) {
	t.Helper()
	entries, categories, err := parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]entry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}
	return byPath, categories
}

func TestParseCategories(t *testing.T) {
	_, categories := parseDoc(t)
	want := []string{"Model and responses", "Permission settings", "Sandbox settings", "Memory and context"}
	if len(categories) != len(want) {
		t.Fatalf("got %v", categories)
	}
	for i := range want {
		if categories[i] != want[i] {
			t.Fatalf("got %v, want %v", categories, want)
		}
	}
}

func TestParseFieldDetails(t *testing.T) {
	fields, _ := parseDoc(t)

	model, ok := fields["model"]
	if !ok {
		t.Fatal("model is missing")
	}
	if model.Category != "Model and responses" {
		t.Errorf("category = %q", model.Category)
	}
	if model.Summary != "Set the model new sessions use" {
		t.Errorf("summary = %q", model.Summary)
	}
	if strings.Contains(model.Doc, "https://") || strings.Contains(model.Doc, "[") {
		t.Errorf("markdown links should be flattened: %q", model.Doc)
	}
	if !strings.Contains(model.Doc, "pick one each time") {
		t.Errorf("doc = %q", model.Doc)
	}
	if model.Kind != "KindString" {
		t.Errorf("kind = %q", model.Kind)
	}
	if model.Scope != "Any file" {
		t.Errorf("scope = %q", model.Scope)
	}
	if model.Default != "unset" {
		t.Errorf("default = %q", model.Default)
	}
}

func TestParseKinds(t *testing.T) {
	fields, _ := parseDoc(t)
	cases := map[string]string{
		"model":                         "KindString",
		"permissions":                   "KindObject",
		"permissions.allow":             "KindStringList",
		"sandbox.enabled":               "KindBool",
		"sandbox.network.httpProxyPort": "KindNumber",
		"env":                           "KindRaw",
		"mode":                          "KindEnum",
	}
	for path, want := range cases {
		if got := fields[path].Kind; got != want {
			t.Errorf("%s: kind %q, want %q", path, got, want)
		}
	}
}

func TestParseEnumOptions(t *testing.T) {
	fields, _ := parseDoc(t)
	mode := fields["mode"]
	if len(mode.Options) != 3 {
		t.Fatalf("got %+v", mode.Options)
	}
	if mode.Options[0].Value != "notify" || mode.Options[2].Value != "off" {
		t.Errorf("got %+v", mode.Options)
	}

	// A Boolean's nested bullets are notes, not accepted values to pick from.
	if len(fields["sandbox.enabled"].Options) != 0 {
		t.Errorf("got %+v", fields["sandbox.enabled"].Options)
	}
}

func TestParseDeprecation(t *testing.T) {
	fields, _ := parseDoc(t)
	old := fields["oldKey"]
	if !old.Deprec {
		t.Error("expected the removal notice to mark the key deprecated")
	}
	if !strings.Contains(old.Notice, "Removed in v2.1.277") {
		t.Errorf("notice = %q", old.Notice)
	}
	if strings.Contains(old.Notice, "[!WARNING]") {
		t.Errorf("the callout marker should be stripped: %q", old.Notice)
	}
	if fields["model"].Deprec {
		t.Error("model is not deprecated")
	}
}

func TestParseKeysThatOnlyAppearInTheIndex(t *testing.T) {
	fields, _ := parseDoc(t)
	only, ok := fields["indexOnly"]
	if !ok {
		t.Fatal("a key listed only in the index should still be registered")
	}
	if only.Category != "Sandbox settings" || only.Scope != "Managed" {
		t.Errorf("got %+v", only)
	}
	if only.Kind != "KindRaw" {
		t.Errorf("a key with no type line falls back to the JSON editor, got %q", only.Kind)
	}
}

func TestParseSkipsSubsections(t *testing.T) {
	fields, _ := parseDoc(t)
	if _, ok := fields["Permission rule syntax"]; ok {
		t.Error("a #### subsection is not a settings key")
	}
	allow := fields["permissions.allow"]
	if strings.Contains(allow.Doc, "Rules follow the format") {
		t.Errorf("subsection prose leaked into the doc: %q", allow.Doc)
	}
}

func TestRenderProducesCompilableGo(t *testing.T) {
	entries, categories, err := parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	code, err := render(entries, categories, "settings_values.md")
	if err != nil {
		t.Fatalf("the generated code does not parse: %v", err)
	}
	body := string(code)
	for _, want := range []string{
		"// Code generated by cmd/gen-schema from settings_values.md. DO NOT EDIT.",
		"package schema",
		"var categoryOrder = []string{",
		"var fields = []Field{",
		`Path:     "model"`,
		"Kind:     KindBool",
		"Deprecated: true",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("generated code is missing %q", want)
		}
	}
}

func TestHelpers(t *testing.T) {
	if got := lastSegment("sandbox.network.httpProxyPort"); got != "httpProxyPort" {
		t.Errorf("got %q", got)
	}
	if got := lastSegment("model"); got != "model" {
		t.Errorf("got %q", got)
	}
	if got := parentOf("a.b.c"); got != "a.b" {
		t.Errorf("got %q", got)
	}
	if got := parentOf("a"); got != "" {
		t.Errorf("got %q", got)
	}
	if got := clean("see [the docs](https://x) for `details` and **more**"); got != "see the docs for details and more" {
		t.Errorf("got %q", got)
	}
	if got := firstSentence("One. Two."); got != "One." {
		t.Errorf("got %q", got)
	}
	if got := firstSentence("No full stop"); got != "No full stop" {
		t.Errorf("got %q", got)
	}
	if got := categoryIndex([]string{"a", "b"}, "b"); got != 1 {
		t.Errorf("got %d", got)
	}
	if got := categoryIndex([]string{"a"}, "zz"); got != 1 {
		t.Errorf("an unknown category sorts last, got %d", got)
	}
}

func TestInlineOptions(t *testing.T) {
	got := inlineOptions(`string, one of "60s", "5m", or "never"`)
	if len(got) != 3 || got[0].Value != "60s" || got[2].Value != "never" {
		t.Errorf("got %+v", got)
	}
	if got := inlineOptions("Boolean"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestParseTheRealReference(t *testing.T) {
	src, err := readReference()
	if err != nil {
		t.Skipf("the bundled reference is not available: %v", err)
	}
	entries, categories, err := parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 200 {
		t.Errorf("parsed only %d keys", len(entries))
	}
	if len(categories) < 15 {
		t.Errorf("parsed only %d categories", len(categories))
	}
	if _, err := render(entries, categories, "settings_values.md"); err != nil {
		t.Fatal(err)
	}
}
