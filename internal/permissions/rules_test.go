package permissions

import "testing"

func TestParseValid(t *testing.T) {
	cases := []struct {
		raw       string
		tool      string
		specifier string
		hasSpec   bool
	}{
		{"Bash", "Bash", "", false},
		{"Bash(npm run *)", "Bash", "npm run *", true},
		{"Read(./.env)", "Read", "./.env", true},
		{"WebFetch(domain:example.com)", "WebFetch", "domain:example.com", true},
		{"mcp__github__get_issue", "mcp__github__get_issue", "", false},
		{"mcp__github__get_*", "mcp__github__get_*", "", false},
		{"*", "*", "", false},
		{"  Bash(ls)  ", "Bash", "ls", true},
	}
	for _, c := range cases {
		r, err := Parse(c.raw)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.raw, err)
			continue
		}
		if r.Tool != c.tool || r.Specifier != c.specifier || r.HasSpec != c.hasSpec {
			t.Errorf("Parse(%q) = %+v", c.raw, r)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"Bash(ls",
		"Bash)",
		"Bash()",
		"(ls)",
		"mcp__",
		"mcp__*__tool",
		"two words",
	}
	for _, raw := range cases {
		if err := Validate(raw); err == nil {
			t.Errorf("Validate(%q) should have failed", raw)
		}
	}
}

func TestWarning(t *testing.T) {
	if w := Warning("Bash(npm run *)"); w != "" {
		t.Errorf("a plain rule needs no warning, got %q", w)
	}
	if w := Warning("Bsah(ls)"); w == "" {
		t.Error("expected a warning for a misspelled tool")
	}
	if w := Warning("*"); w == "" {
		t.Error("expected a warning for the catch-all rule")
	}
	if w := Warning("WebFetch(example.com)"); w == "" {
		t.Error("expected a hint about the domain: prefix")
	}
	if w := Warning("mcp__github__get_issue"); w != "" {
		t.Errorf("MCP rules need no warning, got %q", w)
	}
	if w := Warning("Bash(ls | wc)"); w == "" {
		t.Error("expected a warning about a pipe in a Bash pattern")
	}
	if w := Warning("not a rule("); w != "" {
		t.Errorf("an unparseable rule reports no warning, got %q", w)
	}
}

func TestDescribe(t *testing.T) {
	cases := map[string]string{
		"Bash":                         "Every Bash call.",
		"*":                            "Every tool call.",
		"Bash(git diff:*)":             `Shell commands starting with "git diff".`,
		"Bash(ls)":                     `The exact shell command "ls".`,
		"Read(./.env)":                 "Read on paths matching ./.env.",
		"WebFetch(domain:example.com)": "Fetches matching domain:example.com.",
		"Agent(explore)":               "Subagent calls matching explore.",
		"Skill(deploy)":                "Skill calls matching deploy.",
	}
	for raw, want := range cases {
		if got := Describe(raw); got != want {
			t.Errorf("Describe(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := Describe("Bash("); got != "" {
		t.Errorf("an invalid rule has no description, got %q", got)
	}
}

func TestComplete(t *testing.T) {
	if got := Complete("Ba"); len(got) != 2 || got[0] != "Bash" || got[1] != "BashOutput" {
		t.Errorf("got %v", got)
	}
	if got := Complete("WebS"); len(got) != 1 || got[0] != "WebSearch" {
		t.Errorf("got %v", got)
	}
	if got := Complete("re"); len(got) == 0 {
		t.Error("completion should ignore case")
	}
	if got := Complete("Bash(np"); got != nil {
		t.Errorf("no completion once a specifier is open, got %v", got)
	}
	if got := Complete(""); len(got) != len(KnownTools()) {
		t.Errorf("an empty prefix offers every tool, got %d", len(got))
	}
}

func TestKnownToolsSorted(t *testing.T) {
	tools := KnownTools()
	for i := 1; i < len(tools); i++ {
		if tools[i-1] > tools[i] {
			t.Fatalf("not sorted at %d: %q then %q", i, tools[i-1], tools[i])
		}
	}
}

func TestConflicts(t *testing.T) {
	allow := []string{"Bash(ls)", "Read(./x)", "Write(./y)"}
	ask := []string{"Read(./x)"}
	deny := []string{"Bash(ls)", "Write(./y)"}

	got := Conflicts(allow, ask, deny)
	if len(got) != 3 {
		t.Fatalf("expected three conflicts, got %v", got)
	}
	if n := len(Conflicts(allow, nil, nil)); n != 0 {
		t.Errorf("expected no conflicts, got %d", n)
	}
}
