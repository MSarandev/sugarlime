// Package config discovers Claude settings files, loads them and writes them
// back durably.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/MSarandev/sugarlime/internal/jsondoc"
	"github.com/MSarandev/sugarlime/internal/schema"
)

// Kind is the role a settings file plays in Claude Code's precedence order.
type Kind int

const (
	KindUser Kind = iota
	KindProject
	KindLocal
	KindManaged
	KindGlobal // ~/.claude.json
	KindCustom
)

func (k Kind) String() string {
	switch k {
	case KindUser:
		return "user"
	case KindProject:
		return "project"
	case KindLocal:
		return "local"
	case KindManaged:
		return "managed"
	case KindGlobal:
		return "global config"
	case KindCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// runtimeGOOS is a variable so tests can check the other platforms' paths.
var runtimeGOOS = runtime.GOOS

// Context is one settings file and the edits made to it in this session.
type Context struct {
	Name     string
	Path     string
	Kind     Kind
	ReadOnly bool

	// Doc holds the live, possibly edited document. It is never nil after Load.
	Doc *jsondoc.Object

	Exists  bool
	LoadErr error

	original *jsondoc.Object
	mode     os.FileMode
}

// Dirty reports whether the context has unsaved edits.
func (c *Context) Dirty() bool {
	if c.Doc == nil || c.original == nil {
		return false
	}
	return !jsondoc.Equal(c.Doc, c.original)
}

// Editable reports whether sugarlime will write to this context.
func (c *Context) Editable() bool { return !c.ReadOnly && c.LoadErr == nil }

// Accepts reports whether a settings key belongs in this file.
func (c *Context) Accepts(f schema.Field) bool {
	if c.Kind == KindGlobal {
		return f.GlobalConfig()
	}
	return !f.GlobalConfig()
}

// Load reads the file into the context. A missing file yields an empty document.
func (c *Context) Load() error {
	c.LoadErr = nil
	c.mode = 0o644

	info, err := os.Stat(c.Path)
	switch {
	case err == nil:
		c.Exists = true
		c.mode = info.Mode().Perm()
	case os.IsNotExist(err):
		c.Exists = false
		c.Doc = jsondoc.New()
		c.original = jsondoc.New()
		return nil
	default:
		c.LoadErr = err
		c.Doc = jsondoc.New()
		c.original = jsondoc.New()
		return err
	}

	data, err := os.ReadFile(c.Path)
	if err != nil {
		c.LoadErr = err
		c.Doc = jsondoc.New()
		c.original = jsondoc.New()
		return err
	}
	doc, err := jsondoc.Parse(data)
	if err != nil {
		c.LoadErr = fmt.Errorf("%s is not valid JSON: %w", c.Path, err)
		c.Doc = jsondoc.New()
		c.original = jsondoc.New()
		return c.LoadErr
	}
	c.Doc = doc
	c.original = doc.Clone()
	return nil
}

// Revert throws away unsaved edits.
func (c *Context) Revert() {
	if c.original != nil {
		c.Doc = c.original.Clone()
	}
}

// Save writes the document atomically, so a crash leaves either file intact.
func (c *Context) Save() error {
	if c.ReadOnly {
		return fmt.Errorf("%s is read-only", c.Path)
	}
	data := c.Doc.Encode()
	if _, err := jsondoc.Parse(data); err != nil {
		return fmt.Errorf("refusing to write invalid JSON: %w", err)
	}

	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".sugarlime-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // no-op once the rename succeeds
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, c.mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, c.Path); err != nil {
		return err
	}
	syncDir(dir)

	c.Exists = true
	c.original = c.Doc.Clone()
	return nil
}

// syncDir flushes the directory entry so the rename survives a crash.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}

// NewContext builds a context for a path the user typed.
func NewContext(path string) (Context, error) {
	expanded, err := ExpandPath(path)
	if err != nil {
		return Context{}, err
	}
	kind := KindCustom
	if filepath.Base(expanded) == ".claude.json" {
		kind = KindGlobal
	}
	c := Context{Name: contextName(expanded), Path: expanded, Kind: kind}
	_ = c.Load()
	return c, nil
}

// ExpandPath resolves ~ and makes the path absolute.
func ExpandPath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	return filepath.Abs(p)
}
