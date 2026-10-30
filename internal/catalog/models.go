// Package catalog lists the models a settings file can name, so the pickers
// can show something friendlier than a bare ID.
package catalog

import "strings"

// Model is one entry a model-valued setting can hold.
type Model struct {
	ID    string
	Label string
	Note  string
}

// aliases resolve to Claude Code's current version of a family.
var aliases = []Model{
	{ID: "default", Label: "Default", Note: "your account's default model"},
	{ID: "best", Label: "Best available", Note: "the most capable model your account can use"},
	{ID: "opusplan", Label: "Opus for plan mode", Note: "Opus while planning, Sonnet to execute"},
	{ID: "opus", Label: "Opus", Note: "current Opus version"},
	{ID: "sonnet", Label: "Sonnet", Note: "current Sonnet version"},
	{ID: "haiku", Label: "Haiku", Note: "current Haiku version"},
	{ID: "fable", Label: "Fable", Note: "current Fable version"},
}

// versions are full model IDs, newest family first.
var versions = []Model{
	{ID: "claude-opus-5-5", Label: "Opus 5.5"},
	{ID: "claude-sonnet-5-5", Label: "Sonnet 5.5"},
	{ID: "claude-haiku-5-5", Label: "Haiku 5.5"},
	{ID: "claude-fable-5-1", Label: "Fable 5.1"},
	{ID: "claude-opus-5", Label: "Opus 5"},
	{ID: "claude-sonnet-5", Label: "Sonnet 5"},
	{ID: "claude-haiku-5", Label: "Haiku 5"},
}

// Models lists aliases first, then full version IDs.
func Models() []Model {
	out := make([]Model, 0, len(aliases)+len(versions))
	out = append(out, aliases...)
	out = append(out, versions...)
	return out
}

// Label returns a friendly name for a model value, or the value itself.
func Label(id string) string {
	for _, m := range Models() {
		if m.ID == id {
			return m.Label
		}
	}
	return id
}

// modelFields are the settings keys whose values name a model.
var modelFields = map[string]bool{
	"model":                    true,
	"advisorModel":             true,
	"fallbackModel":            true,
	"availableModels":          true,
	"deniedModels":             true,
	"agent.model":              true,
	"worktree.model":           true,
	"teammateDefaultModel":     true,
	"modelPicker":              true,
	"permissions.advisorModel": true,
}

// IsModelField reports whether a key holds model names, so pickers can suggest them.
func IsModelField(path string) bool {
	if modelFields[path] {
		return true
	}
	return strings.HasSuffix(path, "Model") || strings.HasSuffix(path, "Models")
}
