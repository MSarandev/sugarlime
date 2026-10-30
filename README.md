# sugarlime

A terminal UI for Claude Code's `settings.json`.

Claude Code reads a lot of settings — 247 keys at the time of writing, spread
across user, project and local files, many of them nested, most of them
undiscoverable unless you keep the reference open in a browser tab. sugarlime
finds those files, shows every key with its documentation next to its value, and
writes your changes back without touching anything else in the file.

```
 ___ _   _  ___   _   ___ _    ___ __  __ ___
/ __| | | |/ __| /_\ | _ \ |  |_ _|  \/  | __|
\__ \ |_| | (_ |/ _ \|   / |__ | || |\/| | _|
|___/\___/ \___/_/ \_\_|_\____|___|_|  |_|___|
                               by Exovoid Ltd.

Which settings file do you want to edit?

› user              ~/.claude/settings.json                       12 keys
  claude-work       ~/.claude-work/settings.json                   4 keys
  claude-personal   ~/.claude-personal/settings.json               7 keys
  project           ./.claude/settings.json                 not created yet
  global config     ~/.claude.json                                 3 keys

  + Add a settings file by path               a
  + Scan a directory for settings files       s
  + Look again in the usual places            r

↑↓ move · enter open · a add · s scan · r rescan · ? help · q exit
```

Pick a file, and you land in the editor:

```
 ___ _   _  ___   _   ___ _    ___ __  __ ___
/ __| | | |/ __| /_\ | _ \ |  |_ _|  \/  | __|    editing  user
\__ \ |_| | (_ |/ _ \|   / |__ | || |\/| | _|     ~/.claude/settings.json
|___/\___/ \___/_/ \_\_|_\____|___|_|  |_|___|    ● unsaved   c switches file
                               by Exovoid Ltd.

 Overview  Models  Permissions  Sandbox  MCP  Hooks  Plugins  Interface  More
  allow ▸                              2 items  │ permissions.defaultMode
  ask ▸                                not set  │
  deny ▸                                1 item  │ Set the permission mode new
  additionalDirectories ▸              not set  │ sessions start in. When you
› defaultMode                      acceptEdits  │ leave it unset, sessions start
  disableBypassPermissionsMode         not set  │ in the built-in default for
  useAutoModeDuringPlan                not set  │ your surface.
                                               │
                                               │ In this file: "acceptEdits"
                                               │
                                               │ Accepts: string, one of:
                                               │   default:
                                               │     runs only reads without
                                               │     asking
 ↑↓ move · enter open · space toggle · x unset · / filter · f find · q exit
```


## Why

Editing `settings.json` by hand means knowing the key exists, remembering
whether it nests, and getting its accepted values right. `/config` covers the
common ones. sugarlime covers all of them, offline, and tells you what each one
does while you change it:

- **Every environment in one place.** It opens on a picker listing each Claude
  setup it found — `~/.claude`, whatever `$CLAUDE_CONFIG_DIR` points at, any
  `~/.claude-work` style sibling, this project's `.claude` — so switching
  between work and personal is one keystroke, not a cd and a path to remember.
- **Everything is reachable.** Tabs follow the reference's own grouping, `f`
  finds any key by name or description and jumps straight to it, and keys the
  reference does not describe are listed rather than hidden.
- **The documentation is in the pane.** What the key does, what it accepts, what
  each accepted value means, what Claude Code does without it, and which file it
  is supposed to live in.
- **Your file survives.** Saving rewrites the keys you edited and leaves the
  rest of the file — including its key order and anything sugarlime has never
  heard of — exactly as it was.
- **It knows the rules.** Permission rules are parsed and checked, sandbox paths
  and domains are validated, and both get advice about the mistakes that are
  easy to make and hard to notice.

## Install

### Download a binary

Grab the archive for your platform from the
[latest release](https://github.com/MSarandev/sugarlime/releases/latest),
unpack it, and put `sugarlime` on your `PATH`:

```sh
tar xzf sugarlime_*_darwin_arm64.tar.gz
sudo mv sugarlime /usr/local/bin/
```

Builds are published for macOS and Linux on both arm64 and x86-64, with
`checksums.txt` to verify them:

```sh
sha256sum --check checksums.txt --ignore-missing
```

On Linux there are `.deb`, `.rpm` and `.apk` packages too.

The binaries are not code-signed. `go install` and a `curl`-fetched
archive both run as-is; an archive downloaded through a **browser** on macOS is
quarantined by Gatekeeper, so clear the flag once:

```sh
xattr -d com.apple.quarantine ./sugarlime
```

### With Go

Needs Go 1.27 or later.

```sh
go install github.com/MSarandev/sugarlime/cmd/sugarlime@latest
```

### From source

```sh
make install          # into $(go env GOPATH)/bin
make build            # ./sugarlime in the working directory
```

It is one static binary with no runtime dependencies — the settings registry and
the logo are compiled in — so it also works to copy it between machines of the
same platform.

A Homebrew tap is not set up yet; see
[docs/RELEASING.md](docs/RELEASING.md#adding-a-homebrew-tap) if you want one.

## Use it

```sh
sugarlime                  # open the settings files it finds
sugarlime -list            # print them and exit
sugarlime -file path.json  # also open another file
sugarlime -dir ~/code/api  # look for project settings there instead of here
sugarlime -version
```

### Keys

| Key | What it does |
| :- | :- |
| `↑` `↓` | move the cursor |
| `enter` `→` | open a group, or edit the key |
| `esc` `←` | back one level, or clear the filter; `esc` at the top of a tab returns to the file picker |
| `tab` `shift+tab`, `1`–`9` | switch tabs |
| `space` | cycle a Boolean: true, false, not set |
| `x` | unset the key, so Claude Code uses its default |
| `a` / `d` | add or delete a list entry or object key |
| `J` / `K` | reorder a list entry |
| `/` | filter the list you are on |
| `f` | find any key in the settings reference |
| `c` | back to the file picker, to switch, add or scan |
| `s` `ctrl+s` | save |
| `r` / `R` | discard edits / re-read the file from disk |
| `?` | help |
| `q` | quit, asking first if anything is unsaved |

On the file picker: `enter` or `1`–`9` opens a file, `a` adds one by path, `s`
scans a directory, `r` looks again in the usual places, `d` removes a file you
added, and `esc` goes back to the file you were editing.

In the JSON editor, `ctrl+s` applies and `esc` cancels.

### Editing values

Each key gets the editor its type calls for, taken from the reference:

| Type | Editor |
| :- | :- |
| Boolean | `space` cycles true, false and not set |
| one of a fixed set | a picker listing the accepted values and what each means |
| array of strings | a list: `a` add, `enter` edit, `d` delete, `J`/`K` reorder |
| number | a text field, parsed and checked |
| object with known keys | opens its keys, so `sandbox.filesystem` is a group |
| flat object, such as `env` | a `name=value` editor, pair by pair |
| anything more complex | a JSON editor with a starting template and parse checking |

An empty value in a text or number field unsets the key, as does `x` anywhere.

## Which files it edits

sugarlime opens on a picker of every settings file it found, so you always know
which one you are about to change. It looks in:

| Where | What it is |
| :- | :- |
| `~/.claude/settings.json` | your settings for every project |
| `$CLAUDE_CONFIG_DIR` | the Claude directory a session is actually using |
| `~/.claude*`, `~/.config/claude*` | separate environments, such as `~/.claude-work` |
| `.claude/settings.json` | this project, shared with the repository |
| `.claude/settings.local.json` | this project, yours and not committed |
| managed settings | deployed by your organization; opened read-only |
| `~/.claude.json` | the few keys Claude Code keeps outside settings files |

### Several Claude environments

If you keep separate Claude setups — a work one and a personal one, say — they
are usually separate configuration directories selected with
`CLAUDE_CONFIG_DIR`:

```sh
CLAUDE_CONFIG_DIR=~/.claude-work claude
```

sugarlime lists all of them. Whatever `$CLAUDE_CONFIG_DIR` points at comes
first, since that is the one a session in this shell would use, and each
directory is named after itself: `~/.claude-work/settings.json` appears as
`claude-work`, and a `settings.local.json` beside it as `claude-work local`.
A directory with no settings file in it yet is skipped.

`sugarlime -list` prints what it found and exits, which is the quickest way to
check it sees all of yours.

### Files it cannot guess

Press `a` on the picker to add a file by path, or `s` to scan a directory — a
folder of repositories, say — for `.claude/settings.json` files below it. The
scan skips `node_modules`, `.git`, `vendor` and the like, stops five levels
down, and follows no symlinks. Added files are remembered in
`~/.config/sugarlime/contexts.json` (honouring `$XDG_CONFIG_HOME`), and `d`
removes one from the list without touching the file. `r` looks again in the
usual places, for a file created since sugarlime started.

Only the file you are editing is written. Managed settings are never written.
The detail pane names the other open files that set the same key, so you can see
when a project file is about to be overridden by a local one.

## Saving

A settings file usually holds more than you are editing, so saving is
deliberately conservative:

- the file is parsed into an ordered tree, so keys keep their place and their
  formatting, and a save rewrites only what you changed
- keys sugarlime does not recognise are kept, and stay visible and editable
  under **Keys not in the reference**
- the rendered JSON is re-parsed before anything is written
- the write goes to a temporary file in the same directory, is `fsync`ed, then
  renamed over the target and the directory `fsync`ed, so an interrupted save
  leaves the previous file intact
- the file's permissions are preserved; a file that does not exist yet is
  created on first save, along with its directory

There is a test asserting that re-encoding an untouched settings file reproduces
it byte for byte.

## Keeping up with Claude Code

`settings_values.md` is the Claude Code settings reference.
[`cmd/gen-schema`](cmd/gen-schema) parses it into
`internal/schema/registry_gen.go`: one entry per key with its type, accepted
values, default, scope, documentation and any deprecation notice. The UI builds
its tabs, lists, editors and help from that registry, so refreshing the
reference is the whole update:

```sh
# replace settings_values.md with the current reference, then
make gen && make test
```

CI fails if the checked-in registry does not match the reference.

## Platforms

Builds are published for macOS and Linux, on arm64 and x86-64.

Developed and tested on macOS. Linux should work — CI runs the test suite there,
and the Linux paths for managed settings and sandbox warnings are handled — but
it has not been used in anger. Reports welcome. Windows is not supported: the
code assumes POSIX paths in places, and nothing is tested there.

## Known limits

- Complex keys (`hooks`, `statusLine`, `modelPicker`, `sshConfigs` and the other
  deeply nested shapes) are edited as JSON, with a template and parse checking,
  rather than through a purpose-built form.
- Values are shown per file. sugarlime does not compute the merged result Claude
  Code would actually use across files.
- It edits settings files only. `.mcp.json`, agent and skill definitions and the
  rest of `.claude/` are out of scope.

## Development

```sh
make test      # go test ./...
make cover     # total statement coverage
make lint      # golangci-lint
make fmt       # gofmt
make gen       # regenerate the settings registry
```

`go test ./internal/ui -run TestRenderFrames -v` prints a frame of each screen,
for looking at the layout by eye.

[docs/RELEASING.md](docs/RELEASING.md) covers cutting a release, testing the
packaging locally, and the Homebrew tap setup.

[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) explains how it is put together and
why. [CONTRIBUTING.md](CONTRIBUTING.md) covers what to run before a pull
request.

## License

MIT — see [LICENSE](LICENSE).

`settings_values.md` is Anthropic's Claude Code documentation, vendored as the
generator's input and not covered by this project's license; see
[NOTICE](NOTICE). sugarlime is not affiliated with or endorsed by Anthropic.
