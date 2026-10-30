package ui

import (
	"fmt"
	"testing"
)

// TestRenderFrames prints a frame of each main screen. Run with -v to look at
// the layout by eye: go test ./internal/ui -run TestRenderFrames -v
func TestRenderFrames(t *testing.T) {
	a, _ := newTestApp(t, sampleSettings)

	frames := []struct {
		name string
		do   func()
	}{
		{"file picker", func() { a.screen = screenFiles; a.refreshFiles() }},
		{"overview", func() { enterBrowse(a) }},
		{"permissions", func() { gotoTab(t, a, "Permissions"); focus(t, a, "defaultMode") }},
		{"permissions.allow", func() { focus(t, a, "allow"); press(a, "enter") }},
		{"sandbox", func() { gotoTab(t, a, "Sandbox") }},
		{"bool picker", func() { focus(t, a, "enabled"); press(a, "enter") }},
		{"help", func() { press(a, "esc"); a.showHelp = true }},
	}
	for _, f := range frames {
		f.do()
		fmt.Printf("\n=== %s ===\n%s\n", f.name, a.View())
	}
}
