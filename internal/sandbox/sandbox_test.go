package sandbox

import "testing"

func TestValidatePath(t *testing.T) {
	for _, ok := range []string{"/tmp/build", "~/.kube", "./output", "app/[slug]", "//abs"} {
		if err := ValidatePath(ok); err != nil {
			t.Errorf("ValidatePath(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"", "   ", " /tmp", "/tmp ", "a\x00b"} {
		if err := ValidatePath(bad); err == nil {
			t.Errorf("ValidatePath(%q) should have failed", bad)
		}
	}
}

func TestWarnPath(t *testing.T) {
	cases := map[string]bool{
		"/tmp/build":    false,
		"~/.aws/":       true,
		"//tmp":         true,
		"~/d/{a,b}":     true,
		"/srv/**/*.pem": true,
		"~/build/**":    false,
	}
	for path, wantWarning := range cases {
		got := WarnPath(path)
		if (got != "") != wantWarning {
			t.Errorf("WarnPath(%q) = %q, wanted a warning: %v", path, got, wantWarning)
		}
	}
}

func TestValidateDomain(t *testing.T) {
	for _, ok := range []string{
		"github.com", "*.github.com", "10.0.0.1", "::1",
		"api.example.com:8443", "[::1]:443", "localhost",
	} {
		if err := ValidateDomain(ok); err != nil {
			t.Errorf("ValidateDomain(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "  ", "https://github.com", "github.com/path",
		"github.com:0", "github.com:70000", "github.com:http", "10.0.0.0/8",
		":443", "exam ple.com", "a..b",
	} {
		if err := ValidateDomain(bad); err == nil {
			t.Errorf("ValidateDomain(%q) should have failed", bad)
		}
	}
}

func TestWarnDomain(t *testing.T) {
	if WarnDomain("github.com") != "" {
		t.Error("a plain domain needs no warning")
	}
	for _, d := range []string{"*", "*.github.com", "www.github.com"} {
		if WarnDomain(d) == "" {
			t.Errorf("expected a warning for %q", d)
		}
	}
}

func TestKindFor(t *testing.T) {
	cases := map[string]ListKind{
		"sandbox.filesystem.allowRead":   ListPaths,
		"sandbox.filesystem.denyWrite":   ListPaths,
		"sandbox.network.allowedDomains": ListDomains,
		"sandbox.network.deniedDomains":  ListDomains,
		"permissions.allow":              ListOther,
		"sandbox.excludedCommands":       ListOther,
	}
	for path, want := range cases {
		if got := KindFor(path); got != want {
			t.Errorf("KindFor(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestPrefixesDocumented(t *testing.T) {
	if len(Prefixes) != 3 {
		t.Fatalf("expected the three documented prefixes, got %d", len(Prefixes))
	}
	for _, p := range Prefixes {
		if p.Label == "" || p.Doc == "" {
			t.Errorf("incomplete prefix entry: %+v", p)
		}
	}
}
