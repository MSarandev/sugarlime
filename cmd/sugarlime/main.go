// Command sugarlime is a terminal editor for Claude Code settings files.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// Build information, set with -ldflags in release builds. Reported by
// -version, so a bug report can name an exact build.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// buildInfo is what the Go toolchain stamps into a binary by itself, which is
// all there is to go on when it was installed with `go install` rather than
// built by the release pipeline.
type buildInfo struct {
	version  string
	revision string
	time     string
	modified bool
}

func currentBuildInfo() buildInfo {
	var bi buildInfo
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return bi
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		bi.version = v
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			bi.revision = s.Value
		case "vcs.time":
			bi.time = s.Value
		case "vcs.modified":
			bi.modified = s.Value == "true"
		}
	}
	return bi
}

// versionLine describes this build in one line.
func versionLine() string {
	return formatVersion(version, commit, date, currentBuildInfo())
}

func formatVersion(v, c, d string, bi buildInfo) string {
	if v == "dev" && bi.version != "" {
		// Module versions carry a v prefix; the release pipeline's do not.
		// Drop it so both report the same string.
		v = strings.TrimPrefix(bi.version, "v")
	}
	if c == "" && len(bi.revision) >= 7 {
		c = bi.revision[:7]
	}
	if d == "" {
		d = bi.time
	}

	var detail []string
	if c != "" {
		detail = append(detail, c)
	}
	if d != "" {
		detail = append(detail, d)
	}
	if bi.modified {
		detail = append(detail, "modified")
	}

	out := "sugarlime " + v
	if len(detail) > 0 {
		out += " (" + strings.Join(detail, ", ") + ")"
	}
	return out
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, launch))
}

// launcher runs the interactive app. Tests pass their own.
type launcher func(*ui.App) error

func launch(app *ui.App) error {
	_, err := tea.NewProgram(app, tea.WithAltScreen()).Run()
	return err
}

func run(args []string, stdout, stderr io.Writer, launch launcher) int {
	fs := flag.NewFlagSet("sugarlime", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "sugarlime — edit Claude Code settings files\n\nUsage: sugarlime [flags]\n\nFlags:")
		fs.PrintDefaults()
	}
	var (
		showVersion = fs.Bool("version", false, "print the version and exit")
		list        = fs.Bool("list", false, "list the settings files sugarlime found and exit")
		dir         = fs.String("dir", "", "directory to look for project settings in (default: the working directory)")
		extra       = fs.String("file", "", "also open this settings file")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, versionLine())
		return 0
	}

	ctxs := config.Discover(*dir)
	if *extra != "" {
		c, err := config.NewContext(*extra)
		if err != nil {
			fmt.Fprintln(stderr, "sugarlime:", err)
			return 1
		}
		ctxs = append(ctxs, c)
	}

	if *list {
		printContexts(stdout, ctxs)
		return 0
	}

	app := ui.New(ctxs).WithWorkdir(*dir)
	if err := launch(app); err != nil {
		fmt.Fprintln(stderr, "sugarlime:", err)
		return 1
	}
	if msg := app.QuitMessage(); msg != "" {
		fmt.Fprintln(stdout, msg)
	}
	return 0
}

func printContexts(w io.Writer, ctxs []config.Context) {
	if len(ctxs) == 0 {
		fmt.Fprintln(w, "No Claude settings files found.")
		return
	}
	for _, c := range ctxs {
		state := "ok"
		switch {
		case c.LoadErr != nil:
			state = "unreadable: " + c.LoadErr.Error()
		case !c.Exists:
			state = "not created yet"
		}
		fmt.Fprintf(w, "%-14s %-14s %s (%s)\n", c.Name, c.Kind, c.Path, state)
	}
}
