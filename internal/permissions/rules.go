// Package permissions parses and checks the rule strings that go in
// permissions.allow, permissions.ask and permissions.deny.
package permissions

import (
	"fmt"
	"sort"
	"strings"
)

// Rule is a parsed permission rule, as in Bash(npm run *) or Read(./.env).
type Rule struct {
	Raw       string
	Tool      string
	Specifier string
	HasSpec   bool
}

// Tools Claude Code ships with, used for completion and for warning about typos.
var tools = []string{
	"Agent", "Artifact", "ArtifactComments", "ArtifactData", "AskUserQuestion",
	"Bash", "BashOutput", "CronCreate", "CronDelete", "CronList", "Edit",
	"EndConversation", "EnterPlanMode", "ExitPlanMode", "Glob", "Grep", "KillShell",
	"ListAgents", "ListMcpResourcesTool", "LSP", "Monitor", "NotebookEdit",
	"PushNotification", "Read", "ReadMcpResourceTool", "ReportFindings",
	"ScheduleWakeup", "SendFeedback", "SendMessage", "Skill", "SlashCommand",
	"TaskStop", "TodoWrite", "ToolSearch", "WebFetch", "WebSearch", "Workflow", "Write",
}

// KnownTools lists the built-in tool names a rule can name.
func KnownTools() []string {
	out := append([]string(nil), tools...)
	sort.Strings(out)
	return out
}

// Parse splits a rule into its tool name and specifier.
func Parse(raw string) (Rule, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Rule{}, fmt.Errorf("a rule cannot be empty")
	}
	r := Rule{Raw: s}

	open := strings.Index(s, "(")
	if open < 0 {
		if strings.Contains(s, ")") {
			return Rule{}, fmt.Errorf("closing %q without an opening %q", ")", "(")
		}
		r.Tool = s
		return r, validateTool(r.Tool)
	}
	if !strings.HasSuffix(s, ")") {
		return Rule{}, fmt.Errorf("missing the closing %q", ")")
	}
	r.Tool = strings.TrimSpace(s[:open])
	r.Specifier = s[open+1 : len(s)-1]
	r.HasSpec = true
	if r.Tool == "" {
		return Rule{}, fmt.Errorf("a rule needs a tool name before %q", "(")
	}
	if strings.TrimSpace(r.Specifier) == "" {
		return Rule{}, fmt.Errorf("%s() has an empty specifier; drop the brackets to match every call", r.Tool)
	}
	return r, validateTool(r.Tool)
}

func validateTool(tool string) error {
	if strings.ContainsAny(tool, " \t") {
		return fmt.Errorf("a tool name cannot contain spaces")
	}
	if strings.HasPrefix(tool, "mcp__") {
		return validateMcpTool(tool)
	}
	return nil
}

// validateMcpTool checks mcp__<server>__<tool>; a wildcard fits only the tool part.
func validateMcpTool(tool string) error {
	rest := strings.TrimPrefix(tool, "mcp__")
	parts := strings.SplitN(rest, "__", 2)
	if parts[0] == "" {
		return fmt.Errorf("an MCP rule needs a server name after mcp__")
	}
	if strings.Contains(parts[0], "*") {
		return fmt.Errorf("a wildcard cannot appear in the MCP server name")
	}
	return nil
}

// Validate reports rules sugarlime refuses to save.
func Validate(raw string) error {
	_, err := Parse(raw)
	return err
}

// Warning advises about a valid rule that is probably not what was meant.
func Warning(raw string) string {
	r, err := Parse(raw)
	if err != nil {
		return ""
	}
	switch {
	case r.Tool == "*":
		return "matches every tool"
	case strings.HasPrefix(r.Tool, "mcp__"), strings.Contains(r.Tool, "*"):
		return ""
	case !known(r.Tool):
		return fmt.Sprintf("%q is not a built-in tool name; check the spelling, or keep it if a plugin provides it", r.Tool)
	case r.Tool == "Bash" && r.HasSpec && strings.Contains(r.Specifier, "|"):
		return "a Bash rule matches the command as Claude writes it, so a pipe in the pattern rarely matches"
	case r.Tool == "WebFetch" && r.HasSpec && !strings.HasPrefix(r.Specifier, "domain:"):
		return "a WebFetch rule usually reads WebFetch(domain:example.com)"
	}
	return ""
}

func known(tool string) bool {
	for _, t := range tools {
		if t == tool {
			return true
		}
	}
	return false
}

// Describe explains in one line what a rule matches, for the detail pane.
func Describe(raw string) string {
	r, err := Parse(raw)
	if err != nil {
		return ""
	}
	if !r.HasSpec {
		if r.Tool == "*" {
			return "Every tool call."
		}
		return fmt.Sprintf("Every %s call.", r.Tool)
	}
	switch r.Tool {
	case "Bash":
		if strings.HasSuffix(r.Specifier, "*") {
			prefix := strings.TrimSuffix(r.Specifier, "*")
			prefix = strings.TrimRight(prefix, ": ")
			return fmt.Sprintf("Shell commands starting with %q.", prefix)
		}
		return fmt.Sprintf("The exact shell command %q.", r.Specifier)
	case "Read", "Edit", "Write":
		return fmt.Sprintf("%s on paths matching %s.", r.Tool, r.Specifier)
	case "WebFetch":
		return fmt.Sprintf("Fetches matching %s.", r.Specifier)
	case "Agent":
		return fmt.Sprintf("Subagent calls matching %s.", r.Specifier)
	default:
		return fmt.Sprintf("%s calls matching %s.", r.Tool, r.Specifier)
	}
}

// Complete suggests rule prefixes for what the user has typed so far.
func Complete(prefix string) []string {
	p := strings.TrimSpace(prefix)
	if strings.Contains(p, "(") {
		return nil
	}
	lower := strings.ToLower(p)
	var out []string
	for _, t := range KnownTools() {
		if lower == "" || strings.HasPrefix(strings.ToLower(t), lower) {
			out = append(out, t)
		}
	}
	return out
}

// Conflicts reports rules in more than one list, where the stricter one wins.
func Conflicts(allow, ask, deny []string) []string {
	in := func(list []string, rule string) bool {
		for _, r := range list {
			if r == rule {
				return true
			}
		}
		return false
	}
	var out []string
	for _, r := range allow {
		switch {
		case in(deny, r):
			out = append(out, fmt.Sprintf("%s is in allow and deny; deny wins", r))
		case in(ask, r):
			out = append(out, fmt.Sprintf("%s is in allow and ask; ask wins", r))
		}
	}
	for _, r := range ask {
		if in(deny, r) {
			out = append(out, fmt.Sprintf("%s is in ask and deny; deny wins", r))
		}
	}
	sort.Strings(out)
	return out
}
