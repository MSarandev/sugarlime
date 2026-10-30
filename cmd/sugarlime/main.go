// Command sugarlime is a terminal editor for Claude Code settings files.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

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

// versionLine describes this build in one line.
func versionLine() string {
	out := "sugarlime " + version
	switch {
	case commit != "" && date != "":
		out += " (" + commit + ", " + date + ")"
	case commit != "":
		out += " (" + commit + ")"
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
