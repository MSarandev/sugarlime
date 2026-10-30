package schema

import (
	"strings"
	"testing"
)

func TestRegistryIsComplete(t *testing.T) {
	if len(All()) < 200 {
		t.Fatalf("the registry looks truncated: %d keys", len(All()))
	}
	cats := map[string]bool{}
	for _, c := range Categories() {
		cats[c] = true
	}
	for _, f := range All() {
		if f.Path == "" || f.Name == "" {
			t.Errorf("field with an empty path or name: %+v", f)
		}
		if f.Kind == "" {
			t.Errorf("%s has no kind", f.Path)
		}
		if !cats[f.Category] {
			t.Errorf("%s is in category %q, which is not in the ordered list", f.Path, f.Category)
		}
		if f.Summary == "" {
			t.Errorf("%s has no summary", f.Path)
		}
		if f.Kind == KindEnum && len(f.Options) == 0 {
			t.Errorf("%s is an enum with no accepted values", f.Path)
		}
	}
}

func TestLookupKnownKeys(t *testing.T) {
	for _, path := range []string{
		"model", "permissions.allow", "permissions.defaultMode",
		"sandbox.enabled", "sandbox.filesystem.denyRead", "sandbox.network.allowedDomains",
		"hooks", "env", "autoConnectIde",
	} {
		if _, ok := Lookup(path); !ok {
			t.Errorf("%s is missing from the registry", path)
		}
	}
	if _, ok := Lookup("notASetting"); ok {
		t.Error("expected an unknown key to be absent")
	}
}

func TestKinds(t *testing.T) {
	cases := map[string]Kind{
		"sandbox.enabled":               KindBool,
		"permissions.allow":             KindStringList,
		"permissions.defaultMode":       KindEnum,
		"permissions":                   KindObject,
		"sandbox.network.httpProxyPort": KindNumber,
		"hooks":                         KindRaw,
		"apiKeyHelper":                  KindString,
		"sandbox.filesystem.allowRead":  KindStringList,
		"sandbox.filesystem.disabled":   KindBool,
	}
	for path, want := range cases {
		f, ok := Lookup(path)
		if !ok {
			t.Fatalf("%s is missing", path)
		}
		if f.Kind != want {
			t.Errorf("%s: kind %q, want %q (type text: %q)", path, f.Kind, want, f.TypeText)
		}
	}
}

func TestChildrenAndRoots(t *testing.T) {
	children := Children("permissions")
	var names []string
	for _, c := range children {
		names = append(names, c.Name)
		if c.Parent() != "permissions" {
			t.Errorf("%s is not a direct child", c.Path)
		}
	}
	for _, want := range []string{"allow", "ask", "deny", "defaultMode"} {
		if !contains(names, want) {
			t.Errorf("permissions is missing the child %q; got %v", want, names)
		}
	}

	// Grandchildren must not appear.
	for _, c := range Children("sandbox") {
		if strings.Count(c.Path, ".") != 1 {
			t.Errorf("%s is not a direct child of sandbox", c.Path)
		}
	}

	roots := Roots("Permission settings")
	for _, r := range roots {
		if p := r.Parent(); p != "" {
			if _, nested := Lookup(p); nested {
				t.Errorf("%s is nested and should not be a root", r.Path)
			}
		}
	}
	var rootNames []string
	for _, r := range roots {
		rootNames = append(rootNames, r.Name)
	}
	if !contains(rootNames, "permissions") {
		t.Errorf("permissions should be a root of its category: %v", rootNames)
	}
}

func TestSearchRanksExactPathFirst(t *testing.T) {
	hits := Search("permissions.allow")
	if len(hits) == 0 || hits[0].Path != "permissions.allow" {
		t.Fatalf("got %v", paths(hits))
	}

	hits = Search("defaultmode")
	if len(hits) == 0 {
		t.Fatal("expected a case-insensitive match")
	}

	hits = Search("sandbox")
	if len(hits) < 5 {
		t.Errorf("expected several sandbox keys, got %d", len(hits))
	}

	if got := Search("   "); got != nil {
		t.Errorf("a blank query should match nothing, got %v", paths(got))
	}
	if got := Search("zzzznothing"); len(got) != 0 {
		t.Errorf("got %v", paths(got))
	}
}

func TestSearchFindsByDescription(t *testing.T) {
	hits := Search("extended thinking")
	if len(hits) == 0 {
		t.Fatal("expected a match in the documentation text")
	}
}

func TestFieldHelpers(t *testing.T) {
	f, _ := Lookup("sandbox.filesystem.denyRead")
	if f.Depth() != 2 {
		t.Errorf("depth = %d", f.Depth())
	}
	if f.Parent() != "sandbox.filesystem" {
		t.Errorf("parent = %q", f.Parent())
	}

	top, _ := Lookup("model")
	if top.Parent() != "" || top.Depth() != 0 {
		t.Errorf("a top-level key has no parent")
	}

	global, _ := Lookup("autoConnectIde")
	if !global.GlobalConfig() {
		t.Error("autoConnectIde belongs in ~/.claude.json")
	}
	if top.GlobalConfig() {
		t.Error("model does not belong in ~/.claude.json")
	}

	managed, _ := Lookup("allowManagedPermissionRulesOnly")
	if !managed.Managed() {
		t.Errorf("expected a managed-only key, scope is %q", managed.Scope)
	}

	mode, _ := Lookup("permissions.defaultMode")
	vals := mode.OptionValues()
	if !contains(vals, "acceptEdits") || !contains(vals, "plan") {
		t.Errorf("got %v", vals)
	}
}

func TestDeprecatedKeysAreMarked(t *testing.T) {
	f, ok := Lookup("includeCoAuthoredBy")
	if !ok {
		t.Fatal("includeCoAuthoredBy is missing")
	}
	if !f.Deprecated {
		t.Error("includeCoAuthoredBy is deprecated in the reference")
	}
	if f.Notice == "" {
		t.Error("expected the reference's notice to be carried over")
	}

	current, _ := Lookup("model")
	if current.Deprecated {
		t.Error("model is not deprecated")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func paths(fields []Field) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.Path
	}
	return out
}
