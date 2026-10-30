package jsondoc

import (
	"encoding/json"
	"strings"
	"testing"
)

const sample = `{
  "model": "sonnet",
  "permissions": {
    "allow": [
      "Bash(ls)"
    ],
    "defaultMode": "plan"
  },
  "count": 7,
  "ratio": 0.25,
  "flag": true,
  "nothing": null,
  "mixed": [
    1,
    "two",
    {
      "three": true
    }
  ]
}
`

func TestParseEncodeRoundTrip(t *testing.T) {
	o, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(o.Encode()); got != sample {
		t.Errorf("round trip changed the document:\n%s", got)
	}
}

func TestParseKeepsKeyOrder(t *testing.T) {
	o, err := Parse([]byte(`{"z":1,"a":2,"m":3}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"z", "a", "m"}
	got := o.Keys()
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\t"} {
		o, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if o.Len() != 0 {
			t.Fatalf("%q: expected an empty object", in)
		}
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	cases := []string{`[1,2]`, `{`, `{"a":}`, `"string"`, `{"a":1} {"b":2}`, `{"a":1`}
	for _, in := range cases {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%q: expected an error", in)
		}
	}
}

func TestGetSetDeleteNested(t *testing.T) {
	o := New()
	if err := o.Set("sandbox.network.allowedDomains", []any{"github.com"}); err != nil {
		t.Fatal(err)
	}
	if !o.Has("sandbox.network.allowedDomains") {
		t.Fatal("expected the key to be present")
	}
	if !o.Has("sandbox") || !o.Has("sandbox.network") {
		t.Fatal("expected intermediate objects to be created")
	}
	vals, ok := o.Strings("sandbox.network.allowedDomains")
	if !ok || len(vals) != 1 || vals[0] != "github.com" {
		t.Fatalf("got %v %v", vals, ok)
	}

	if !o.Delete("sandbox.network.allowedDomains") {
		t.Fatal("expected the delete to report success")
	}
	if o.Has("sandbox") {
		t.Error("deleting the last child should prune the empty parents")
	}
}

func TestDeleteKeepsSiblings(t *testing.T) {
	o, err := Parse([]byte(`{"sandbox":{"enabled":true,"network":{"httpProxyPort":8080}}}`))
	if err != nil {
		t.Fatal(err)
	}
	o.Delete("sandbox.network.httpProxyPort")
	if o.Has("sandbox.network") {
		t.Error("the emptied network object should be gone")
	}
	if !o.Has("sandbox.enabled") {
		t.Error("the sibling key should survive")
	}
}

func TestDeleteMissing(t *testing.T) {
	o, _ := Parse([]byte(`{"a":{"b":1}}`))
	if o.Delete("a.c") {
		t.Error("deleting a missing key should report false")
	}
	if o.Delete("x.y") {
		t.Error("deleting under a missing parent should report false")
	}
	if o.Delete("") {
		t.Error("deleting an empty path should report false")
	}
}

func TestSetThroughScalarFails(t *testing.T) {
	o, _ := Parse([]byte(`{"model":"sonnet"}`))
	if err := o.Set("model.name", "x"); err == nil {
		t.Fatal("expected setting a child of a string to fail")
	}
	if err := o.Set("", "x"); err == nil {
		t.Fatal("expected an empty path to fail")
	}
}

func TestSetReplacesInPlace(t *testing.T) {
	o, _ := Parse([]byte(`{"a":1,"b":2,"c":3}`))
	if err := o.Set("b", json.Number("9")); err != nil {
		t.Fatal(err)
	}
	if got := string(o.Encode()); !strings.Contains(got, "\"a\": 1,\n  \"b\": 9,\n  \"c\": 3") {
		t.Errorf("the key should keep its position:\n%s", got)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	o, _ := Parse([]byte(`{"permissions":{"allow":["a"]}}`))
	c := o.Clone()
	_ = c.Set("permissions.allow", []any{"a", "b"})
	vals, _ := o.Strings("permissions.allow")
	if len(vals) != 1 {
		t.Fatalf("the original changed: %v", vals)
	}
	if Equal(o, c) {
		t.Fatal("the clone should differ after the edit")
	}
}

func TestTypedAccessors(t *testing.T) {
	o, _ := Parse([]byte(`{"s":"x","b":true,"n":12,"arr":["a","b"],"bad":[1],"obj":{}}`))

	if v, ok := o.Str("s"); !ok || v != "x" {
		t.Errorf("Str: %v %v", v, ok)
	}
	if _, ok := o.Str("b"); ok {
		t.Error("Str should reject a Boolean")
	}
	if v, ok := o.Bool("b"); !ok || !v {
		t.Errorf("Bool: %v %v", v, ok)
	}
	if _, ok := o.Bool("missing"); ok {
		t.Error("Bool should report a missing key")
	}
	if v, ok := o.Num("n"); !ok || v.String() != "12" {
		t.Errorf("Num: %v %v", v, ok)
	}
	if _, ok := o.Num("s"); ok {
		t.Error("Num should reject a string")
	}
	if v, ok := o.Strings("arr"); !ok || len(v) != 2 {
		t.Errorf("Strings: %v %v", v, ok)
	}
	if _, ok := o.Strings("bad"); ok {
		t.Error("Strings should reject a non-string element")
	}
	if _, ok := o.Strings("obj"); ok {
		t.Error("Strings should reject an object")
	}
	if _, ok := o.Strings("missing"); ok {
		t.Error("Strings should report a missing key")
	}
}

func TestSetStringsEmptyDeletes(t *testing.T) {
	o, _ := Parse([]byte(`{"permissions":{"allow":["a"]}}`))
	if err := o.SetStrings("permissions.allow", nil); err != nil {
		t.Fatal(err)
	}
	if o.Has("permissions.allow") {
		t.Error("an empty list should remove the key")
	}
}

func TestSetRaw(t *testing.T) {
	o := New()
	if err := o.SetRaw("hooks", `{"PreToolUse":[{"matcher":"Bash"}]}`); err != nil {
		t.Fatal(err)
	}
	if !o.Has("hooks.PreToolUse") {
		t.Error("expected the nested key to be readable")
	}
	if err := o.SetRaw("x", `{not json}`); err == nil {
		t.Error("expected invalid JSON to fail")
	}
	if err := o.SetRaw("x", `1 2`); err == nil {
		t.Error("expected trailing content to fail")
	}
}

func TestEncodeEscapesWithoutHTMLMangling(t *testing.T) {
	o := New()
	_ = o.Set("pattern", `a<b>&"c"`)
	got := string(o.Encode())
	if !strings.Contains(got, `"a<b>&\"c\""`) {
		t.Errorf("unexpected escaping: %s", got)
	}
}

func TestEncodeEmptyContainers(t *testing.T) {
	o, _ := Parse([]byte(`{"a":{},"b":[]}`))
	if got := string(o.Encode()); got != "{\n  \"a\": {},\n  \"b\": []\n}\n" {
		t.Errorf("got %q", got)
	}
}

func TestSummary(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{true, "true"},
		{false, "false"},
		{"x", "x"},
		{json.Number("3"), "3"},
		{[]any{1, 2}, "2 items"},
		{[]any{1}, "1 item"},
	}
	for _, c := range cases {
		if got := Summary(c.in); got != c.want {
			t.Errorf("Summary(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	o, _ := Parse([]byte(`{"a":1,"b":2}`))
	if got := Summary(o); got != "2 keys" {
		t.Errorf("object summary = %q", got)
	}
}

func TestRender(t *testing.T) {
	if got := Render("x"); got != `"x"` {
		t.Errorf("got %q", got)
	}
	o, _ := Parse([]byte(`{"a":[1]}`))
	v, _ := o.Get("a")
	if got := Render(v); got != "[\n  1\n]" {
		t.Errorf("got %q", got)
	}
}

func TestDuplicateKeysKeepFirstPosition(t *testing.T) {
	o, err := Parse([]byte(`{"a":1,"b":2,"a":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.Len() != 2 {
		t.Fatalf("expected 2 keys, got %d", o.Len())
	}
	if n, _ := o.Num("a"); n.String() != "3" {
		t.Errorf("the last value should win, got %v", n)
	}
	if o.Keys()[0] != "a" {
		t.Errorf("the first position should be kept, got %v", o.Keys())
	}
}

func TestStringMatchesEncode(t *testing.T) {
	o, _ := Parse([]byte(`{"a":1}`))
	if o.String() != string(o.Encode()) {
		t.Error("String should render the same text as Encode")
	}
}

func TestEncodeGoNativeValues(t *testing.T) {
	o := New()
	_ = o.Set("f", 1.5)
	_ = o.Set("i", 7)
	_ = o.Set("other", struct {
		A int `json:"a"`
	}{A: 1})
	got := string(o.Encode())
	for _, want := range []string{`"f": 1.5`, `"i": 7`, `"other": {"a":1}`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}

func TestEncodeUnsupportedValueFallsBackToNull(t *testing.T) {
	o := New()
	_ = o.Set("bad", func() {})
	if got := string(o.Encode()); !strings.Contains(got, `"bad": null`) {
		t.Errorf("got %s", got)
	}
}

func TestSetStringsWritesTheList(t *testing.T) {
	o := New()
	if err := o.SetStrings("permissions.deny", []string{"Read(./.env)", "Bash(curl *)"}); err != nil {
		t.Fatal(err)
	}
	vals, ok := o.Strings("permissions.deny")
	if !ok || len(vals) != 2 || vals[1] != "Bash(curl *)" {
		t.Fatalf("got %v %v", vals, ok)
	}
	if err := o.SetStrings("permissions.deny.nope", []string{"x"}); err == nil {
		t.Error("expected writing through a list to fail")
	}
}

func TestGetThroughScalar(t *testing.T) {
	o, _ := Parse([]byte(`{"model":"sonnet"}`))
	if _, ok := o.Get("model.name"); ok {
		t.Error("a string has no children")
	}
	if _, ok := o.Get(""); ok {
		t.Error("an empty path resolves to nothing")
	}
}

func TestEqual(t *testing.T) {
	a, _ := Parse([]byte(`{"a":1,"b":2}`))
	b, _ := Parse([]byte(`{"a":1,"b":2}`))
	if !Equal(a, b) {
		t.Error("identical documents should compare equal")
	}
	reordered, _ := Parse([]byte(`{"b":2,"a":1}`))
	if Equal(a, reordered) {
		t.Error("key order is part of the document")
	}
}
