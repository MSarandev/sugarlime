// Package sandbox checks the path and domain entries that go in the
// sandbox.filesystem and sandbox.network lists.
package sandbox

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// PathPrefix describes how the sandbox resolves a filesystem entry.
type PathPrefix struct {
	Label string
	Doc   string
}

// Prefixes documents the three ways a sandbox path can start.
var Prefixes = []PathPrefix{
	{Label: "/path", Doc: "absolute, from the filesystem root"},
	{Label: "~/path", Doc: "relative to your home directory"},
	{Label: "./path", Doc: "relative to the project root, or to ~/.claude in user settings"},
}

// ValidatePath rejects entries the sandbox cannot resolve.
func ValidatePath(entry string) error {
	p := strings.TrimSpace(entry)
	if p == "" {
		return fmt.Errorf("a path cannot be empty")
	}
	if p != entry {
		return fmt.Errorf("a path cannot start or end with a space")
	}
	if strings.ContainsRune(p, 0) {
		return fmt.Errorf("a path cannot contain a null byte")
	}
	return nil
}

// WarnPath returns advice about a resolvable but surprising path entry.
func WarnPath(entry string) string {
	p := strings.TrimSpace(entry)
	switch {
	case strings.HasSuffix(p, "/"):
		return "the trailing slash is dropped, so this is the same as the path without it"
	case strings.HasPrefix(p, "//"):
		return "// is an absolute path here; a single / is absolute too"
	case strings.Contains(p, "{") || strings.Contains(p, "}"):
		return "braces are ordinary characters, so this covers one directory with braces in its name"
	case wildcardBeyondTrailingGlobstar(p):
		return "on Linux and WSL2 only a trailing /** works in write lists; add each path instead"
	}
	return ""
}

func wildcardBeyondTrailingGlobstar(p string) bool {
	trimmed := strings.TrimSuffix(p, "/**")
	return strings.ContainsAny(trimmed, "*?")
}

// ValidateDomain checks a host, wildcard or IP literal with an optional :port.
func ValidateDomain(entry string) error {
	d := strings.TrimSpace(entry)
	if d == "" {
		return fmt.Errorf("a domain cannot be empty")
	}
	if d != entry {
		return fmt.Errorf("a domain cannot start or end with a space")
	}
	if strings.Contains(d, "://") {
		return fmt.Errorf("write the host only, without a scheme such as https://")
	}
	if strings.Contains(d, "/") {
		return fmt.Errorf("write the host only, without a path")
	}

	host := d
	if h, port, ok := splitPort(d); ok {
		host = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("%q is not a port between 1 and 65535", port)
		}
	}
	if host == "" {
		return fmt.Errorf("a domain needs a host before the port")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" {
			return fmt.Errorf("%q has an empty label", d)
		}
		for _, r := range label {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			case r == '-' || r == '_' || r == '*':
			default:
				return fmt.Errorf("%q is not valid in a domain", string(r))
			}
		}
	}
	return nil
}

// splitPort separates a trailing :port, leaving bracketed IPv6 literals alone.
func splitPort(d string) (string, string, bool) {
	if strings.HasPrefix(d, "[") {
		if i := strings.LastIndex(d, "]:"); i > 0 {
			return d[:i+1], d[i+2:], true
		}
		return d, "", false
	}
	i := strings.LastIndex(d, ":")
	if i < 0 || strings.Count(d, ":") > 1 {
		return d, "", false
	}
	return d[:i], d[i+1:], true
}

// WarnDomain returns advice about a valid but easily mistaken domain entry.
func WarnDomain(entry string) string {
	d := strings.TrimSpace(entry)
	switch {
	case d == "*":
		return "allows every domain, which turns network isolation off in practice"
	case strings.HasPrefix(d, "*."):
		return "covers subdomains only; add the bare domain too if you need it"
	case strings.HasPrefix(d, "www."):
		return "covers this host only; the bare domain is a separate entry"
	}
	return ""
}

// ListKind tells the UI which validator a sandbox list needs.
type ListKind int

const (
	ListOther ListKind = iota
	ListPaths
	ListDomains
)

// KindFor maps a settings path to the validator its entries need.
func KindFor(path string) ListKind {
	switch path {
	case "sandbox.filesystem.allowRead",
		"sandbox.filesystem.allowWrite",
		"sandbox.filesystem.denyRead",
		"sandbox.filesystem.denyWrite":
		return ListPaths
	case "sandbox.network.allowedDomains",
		"sandbox.network.deniedDomains":
		return ListDomains
	default:
		return ListOther
	}
}
