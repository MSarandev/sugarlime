package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSarandev/sugarlime/internal/config"
	"github.com/MSarandev/sugarlime/internal/ui"
)

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUGARLIME_CONFIG_DIR", t.TempDir())
}

func TestRunVersion(t *testing.T) {
	isolate(t)
	var out, errOut bytes.Buffer
	code := run([]string{"-version"}, &out, &errOut, func(*ui.App) error {
		t.Fatal("the app should not start")
		return nil
	})
	if code != 0 {
		t.Errorf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "sugarlime "+version) {
		t.Errorf("got %q", out.String())
	}
}

func TestVersionLine(t *testing.T) {
	saved := []string{version, commit, date}
	t.Cleanup(func() { version, commit, date = saved[0], saved[1], saved[2] })

	version, commit, date = "1.2.3", "", ""
	if got := versionLine(); got != "sugarlime 1.2.3" {
		t.Errorf("got %q", got)
	}
	commit = "abc1234"
	if got := versionLine(); got != "sugarlime 1.2.3 (abc1234)" {
		t.Errorf("got %q", got)
	}
	date = "2026-10-30"
	if got := versionLine(); got != "sugarlime 1.2.3 (abc1234, 2026-10-30)" {
		t.Errorf("got %q", got)
	}
}

func TestRunList(t *testing.T) {
	isolate(t)
	var out, errOut bytes.Buffer
	code := run([]string{"-list", "-dir", t.TempDir()}, &out, &errOut, func(*ui.App) error {
		t.Fatal("the app should not start")
		return nil
	})
	if code != 0 {
		t.Errorf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "settings.json") {
		t.Errorf("got %q", out.String())
	}
}

func TestRunOpensExtraFile(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "team.json")
	if err := os.WriteFile(path, []byte(`{"model":"opus"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	var seen *ui.App
	code := run([]string{"-file", path}, &out, &errOut, func(a *ui.App) error {
		seen = a
		return nil
	})
	if code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, errOut.String())
	}
	if seen == nil {
		t.Fatal("the app did not start")
	}
	found := false
	for _, c := range seen.Contexts() {
		if c.Path == path {
			found = true
		}
	}
	if !found {
		t.Error("the file given with -file should be open")
	}
}

func TestRunRejectsABadFilePath(t *testing.T) {
	isolate(t)
	var out, errOut bytes.Buffer
	code := run([]string{"-file", "   "}, &out, &errOut, func(*ui.App) error { return nil })
	if code != 1 {
		t.Errorf("exit code %d", code)
	}
	if !strings.Contains(errOut.String(), "sugarlime:") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunReportsALaunchFailure(t *testing.T) {
	isolate(t)
	var out, errOut bytes.Buffer
	code := run(nil, &out, &errOut, func(*ui.App) error { return errors.New("no terminal") })
	if code != 1 {
		t.Errorf("exit code %d", code)
	}
	if !strings.Contains(errOut.String(), "no terminal") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunRejectsUnknownFlags(t *testing.T) {
	isolate(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"-nope"}, &out, &errOut, func(*ui.App) error { return nil }); code != 2 {
		t.Errorf("exit code %d", code)
	}
}

func TestRunPrintsTheQuitMessage(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"model":"opus"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := run([]string{"-file", path}, &out, &errOut, func(a *ui.App) error {
		// Leave an unsaved edit behind, as quitting without saving would.
		for _, c := range a.Contexts() {
			if c.Path == path {
				return c.Doc.Set("model", "sonnet")
			}
		}
		return nil
	})
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "unsaved") {
		t.Errorf("got %q", out.String())
	}
}

func TestPrintContexts(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(good, []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}

	var ctxs []config.Context
	for _, p := range []string{good, broken, filepath.Join(dir, "missing.json")} {
		c := config.Context{Name: filepath.Base(p), Path: p, Kind: config.KindUser}
		_ = c.Load()
		ctxs = append(ctxs, c)
	}

	var out bytes.Buffer
	printContexts(&out, ctxs)
	for _, want := range []string{"settings.json", "(ok)", "unreadable:", "not created yet"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestPrintContextsWithNone(t *testing.T) {
	var out bytes.Buffer
	printContexts(&out, nil)
	if !strings.Contains(out.String(), "No Claude settings files found.") {
		t.Errorf("got %q", out.String())
	}
}

func TestFormatVersion(t *testing.T) {
	cases := []struct {
		name    string
		v, c, d string
		bi      buildInfo
		want    string
	}{
		{
			name: "release build, stamped with ldflags",
			v:    "1.0.0", c: "abc1234", d: "2026-10-30",
			want: "sugarlime 1.0.0 (abc1234, 2026-10-30)",
		},
		{
			name: "go install of a tagged version",
			v:    "dev",
			bi:   buildInfo{version: "v1.0.0"},
			want: "sugarlime 1.0.0",
		},
		{
			name: "go install off an untagged branch",
			v:    "dev",
			bi:   buildInfo{version: "v0.0.0-20261030164757-0228ddde93c2"},
			want: "sugarlime 0.0.0-20261030164757-0228ddde93c2",
		},
		{
			name: "go build in a clean checkout",
			v:    "dev",
			bi:   buildInfo{revision: "0228ddde93c285ca", time: "2026-10-30T16:47:57Z"},
			want: "sugarlime dev (0228ddd, 2026-10-30T16:47:57Z)",
		},
		{
			name: "go build with uncommitted changes",
			v:    "dev",
			bi:   buildInfo{revision: "0228ddde93c285ca", modified: true},
			want: "sugarlime dev (0228ddd, modified)",
		},
		{
			name: "nothing known at all",
			v:    "dev",
			want: "sugarlime dev",
		},
		{
			name: "ldflags win over the stamped module version",
			v:    "1.2.3", c: "deadbee",
			bi:   buildInfo{version: "v0.0.0-pseudo", revision: "ffffffffff"},
			want: "sugarlime 1.2.3 (deadbee)",
		},
		{
			name: "a revision too short to shorten is left out",
			v:    "dev",
			bi:   buildInfo{revision: "abc"},
			want: "sugarlime dev",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatVersion(c.v, c.c, c.d, c.bi); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestCurrentBuildInfoDoesNotPanic(t *testing.T) {
	// Under `go test` the binary carries build info, so this exercises the
	// real reader rather than a fake.
	bi := currentBuildInfo()
	if bi.revision != "" && len(bi.revision) < 7 {
		t.Errorf("unexpected revision %q", bi.revision)
	}
}
