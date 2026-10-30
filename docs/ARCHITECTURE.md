# Architecture

sugarlime edits files it does not own. Claude Code reads `settings.json`, other
tools write to it, and a file usually holds more keys than any one editing
session touches. Two decisions follow from that, and the rest of the design
hangs off them.

## 1. The file is a tree, not a struct

A settings file is parsed into `internal/jsondoc.Object`: a JSON object that
remembers the order its keys were parsed in and keeps every value, including
shapes sugarlime has no opinion about.

The obvious alternative — Go structs with `json` tags for each settings category
— cannot be saved safely. Marshalling a struct drops every key it does not
declare and emits a value for every key the user never set, so a single edit
rewrites the whole file and silently deletes anything the structs missed: a key
from a newer Claude Code, a key another tool wrote, a typo the user wants to
find. That is the opposite of what a settings editor should do.

So `jsondoc` offers:

- `Get`, `Set`, `Delete`, `Has` over dotted paths such as
  `sandbox.network.allowedDomains`, creating intermediate objects on write and
  pruning emptied ones on delete
- typed readers (`Str`, `Bool`, `Num`, `Strings`) that report when a value is
  not the shape the caller expected, rather than coercing it
- an encoder that writes keys in their stored order with two-space indentation

The payoff is that re-encoding an untouched file reproduces it byte for byte, so
a save changes the lines you edited and nothing else. There is a test asserting
this against the developer's own `~/.claude/settings.json`.

Three consequences worth knowing:

- numbers are kept as `json.Number`, so `8080` does not come back as `8080.0`
- a duplicate key keeps its first position and its last value, matching how
  `encoding/json` resolves one
- key order is the document's, so a key added by sugarlime lands at the end of
  its object

## 2. The key list is generated, not written

`settings_values.md` is the Claude Code settings reference. `cmd/gen-schema`
parses it into `internal/schema/registry_gen.go`: one `schema.Field` per key,
carrying its dotted path, category, one-line summary, opening paragraph, scope,
type text, default, inferred kind, accepted values with the reference's own notes
about each, and any deprecation notice.

The generator infers a `Kind` from the reference's type line, which is what
decides the editor a key gets:

| Kind | From a type line like | Editor |
| :- | :- | :- |
| `KindBool` | `Boolean` | `space` cycles true / false / unset |
| `KindEnum` | `string, one of:` | picker listing the accepted values |
| `KindStringList` | `array of …` | list editor: add, edit, delete, reorder |
| `KindNumber` | `number, a local TCP port` | text field, parsed as a number |
| `KindObject` | a key that has children in the registry | opens its children |
| `KindRaw` | `object with …`, `array of objects` | JSON editor |
| `KindString` | anything else | text field |

A key with children is an object regardless of its prose, so `permissions` and
`sandbox.filesystem` become groups automatically.

Everything the UI shows is derived from this registry: the tabs come from the
reference's own categories, the lists from `schema.Roots` and `schema.Children`,
the detail pane from the documentation fields, and `f` searches it. Refreshing
the reference and running `make gen` is the whole update when Claude Code adds a
setting. CI regenerates and diffs, so the checked-in registry cannot drift.

The generator is deliberately strict about nothing. A key it cannot classify
becomes `KindRaw` and gets the JSON editor, which is always correct if less
convenient; a key present only in the reference's index table still gets an
entry. Nothing in the reference is dropped.

### Where the reference comes from

`settings_values.md` is Anthropic's Claude Code documentation, vendored so the
build needs no network and the registry matches a known version of the docs. It
is not sugarlime's own work; see `NOTICE`. Refresh it from
<https://code.claude.com/docs/en/settings> (the `.md` form of the all-settings
page).

## Packages

```
cmd/sugarlime/        flag parsing, discovery, launching the TUI
cmd/gen-schema/       settings_values.md → internal/schema/registry_gen.go
internal/schema/      every settings key and what it accepts (generated + helpers)
internal/jsondoc/     order-preserving JSON tree
internal/config/      file discovery, loading, durable writes, the saved-file store
internal/permissions/ permission rule parsing, validation, advice
internal/sandbox/     sandbox path and domain validation
internal/catalog/     model names offered in model-valued pickers
internal/ui/          the terminal interface
```

Dependencies point one way: `ui` → everything, `config` → `jsondoc` + `schema`,
and `jsondoc`, `schema`, `permissions`, `sandbox`, `catalog` depend on nothing of
ours. The validation packages are separate from the UI so the rules can be
tested, and reused, without a terminal.

## Durable writes

`config.Context.Save` does, in order:

1. render the document
2. re-parse the rendered bytes, and refuse to write if they do not parse
3. `MkdirAll` the parent directory
4. write a temporary file in that same directory
5. `Sync` the file, then `Chmod` it to the original file's permissions
6. `Rename` it over the target, which is atomic within a filesystem
7. `Sync` the directory so the rename itself is durable

A crash therefore leaves either the previous file or the new one, never a
truncated one. `Context` keeps the document as loaded alongside the live one, so
`Dirty` is an exact comparison and `Revert` is free. Managed settings load with
`ReadOnly`, and `Save` refuses them.

## Discovery

`internal/config/discover.go` answers "which settings files exist on this
machine". Beyond the four files Claude Code's precedence order describes, it
looks for the configuration directories people use to keep separate Claude
environments: `$CLAUDE_CONFIG_DIR` first, since that is the directory a session
in the current shell would use, then `~/.claude`, then any `~/.claude*` or
`~/.config/claude*` sibling that actually holds a settings file. Each directory
is named after itself, so `~/.claude-work` lists as `claude-work` rather than a
second `user`.

`ScanDir` covers what cannot be guessed: it walks a directory for
`.claude/settings.json` files, skipping the usual noise (`node_modules`, `.git`,
`vendor`, dot-directories that are not `.claude`), stopping at five levels and
200 results, and following no symlinks. The picker remembers what it finds
through the same store as a hand-added path.

## The UI

`internal/ui` is one Bubbletea model with two top-level screens: the file
picker it opens on (`files.go`) and the browser (everything else). Its state is
the screen, a tab index, a navigation stack per tab, an optional modal overlay,
and a status line.

The picker comes first on purpose. Editing the wrong settings file is a quiet,
annoying mistake, and with several Claude environments on a machine it is an
easy one, so sugarlime makes choosing the file a deliberate step rather than
something inferred and shown in a corner. `c` goes back to it, `esc` returns to
the file you were editing.

- **Rows** (`rows.go`) are what a level lists: a settings key, a group that
  drills in, a list entry, or an object pair. Rows hold schema fields, never
  values — values are read from the document at render time, so an edit anywhere
  shows up everywhere without invalidation logic.
- **Levels** are a stack per tab, which is why going back lands where you were
  and switching tabs keeps your place.
- **Category objects are flattened.** A category whose root object has three or
  more children lists those children directly, so the Permissions tab opens on
  `allow`, `ask`, `deny` rather than on one row to drill through. Children keep
  their short names unless two flattened objects in one level would collide, in
  which case every row falls back to its full path.
- **The logo** is `internal/ui/logo/logo.txt`, embedded with `go:embed`. It
  carries its own colour escapes, so it is printed as authored rather than
  restyled. It is shown beside the open file's details when the window has room
  for it, and replaced by a one-line header when it does not.
- **Overlays** (`overlay.go`) are a text field, a choice list, or a JSON
  textarea, each carrying a `submit` closure. A submit that returns an error
  keeps the overlay open with the message attached, which is how validation
  surfaces; returning `errKeepOverlay` lets one overlay replace another, as the
  value picker does when you choose "Type another value…".
- **Unknown keys are first-class.** A key in the file that the registry does not
  describe appears under "Keys not in the reference" with a kind guessed from
  its value, and nested unknown keys appear beside their known siblings.

Rendering is pure: `View` reads state and returns a string, so tests drive the
app with `tea.KeyMsg` values and assert on either the document or the rendered
frame. `go test ./internal/ui -run TestRenderFrames -v` prints one frame per
screen for looking at the layout by eye.

## Testing

- `TestEveryKeyRenders` and `TestEveryKeyOpensAnEditor` walk all 247 registry
  keys, so a kind with no editor, or a field that breaks the detail pane, fails
  the build
- `TestRendersAtSmallSizes` asserts no rendered line exceeds the terminal width,
  at sizes from 40×12 up
- `TestRealSettingsFileLoads` opens the machine's own settings file read-only,
  renders every row of every tab, and asserts the byte-identical round trip
- the generator is tested against a small hand-written reference covering each
  type-line shape, and against the real one
