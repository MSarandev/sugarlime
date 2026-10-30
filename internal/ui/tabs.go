package ui

import "github.com/MSarandev/sugarlime/internal/schema"

// tabDef groups the reference's categories into the tabs across the top.
type tabDef struct {
	Title string
	Short string
	Cats  []string
}

var tabs = []tabDef{
	{Title: "Overview", Short: "Over"},
	{Title: "Models", Short: "Mdl", Cats: []string{"Model and responses"}},
	{Title: "Permissions", Short: "Perm", Cats: []string{"Permission settings"}},
	{Title: "Sandbox", Short: "Sbx", Cats: []string{"Sandbox settings"}},
	{Title: "MCP", Short: "MCP", Cats: []string{"MCP"}},
	{Title: "Hooks", Short: "Hook", Cats: []string{"Hooks and automation"}},
	{Title: "Plugins", Short: "Plug", Cats: []string{"Plugins and skills"}},
	{Title: "Interface", Short: "UI", Cats: []string{"Interface and terminal", "Git and attribution"}},
	{Title: "More", Short: "More", Cats: []string{
		"Memory and context",
		"Agents, sessions, and worktrees",
		"Remote, desktop, and notifications",
		"Authentication and providers",
		"Tools",
		"Privacy and telemetry",
		"Updates and versioning",
		"Enterprise and managed settings",
		"Global config settings",
	}},
}

// tabOfCategory finds the tab a category is shown under.
func tabOfCategory(category string) int {
	for i, t := range tabs {
		for _, c := range t.Cats {
			if c == category {
				return i
			}
		}
	}
	return 0
}

// tabOfField finds the tab a settings key is shown under.
func tabOfField(f schema.Field) int { return tabOfCategory(f.Category) }
