# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- First working version: a tabbed terminal editor for Claude Code settings
  files, covering every key in the bundled settings reference.
- Opens on a file picker listing every settings file found, so which file is
  about to be edited is always a deliberate choice. `c` returns to it.
- Discovery of several Claude environments: `~/.claude`, whatever
  `$CLAUDE_CONFIG_DIR` points at, any `~/.claude*` or `~/.config/claude*`
  sibling, this project's `.claude`, managed settings and `~/.claude.json`.
  Files you add by hand are remembered in `~/.config/sugarlime/contexts.json`.
- `s` on the picker scans a directory for `.claude/settings.json` files, for a
  folder of repositories.
- The file being edited is the header's headline, with `c` and `esc` at the top
  of a tab both returning to the picker. The footer keeps `s save`,
  `c change file`, `? help` and `q exit` at any window width, dropping the
  other hints as the window narrows.
- Editors per value kind: Boolean cycling, value pickers carrying the
  reference's own notes, text and number fields, a list editor with add, edit,
  delete and reorder, a key/value editor for flat objects such as `env`, and a
  JSON editor with templates for complex keys such as `hooks`.
- Full CRUD for `permissions.allow` / `ask` / `deny` with rule validation,
  tool-name completion, and an explanation of what each rule matches.
- Full CRUD for the `sandbox` object, including `filesystem`, `network` and
  `credentials`, with path-prefix help and domain validation.
- `f` finds any settings key and jumps to it; `/` filters the list you are on.
- Durable saves: validate, write a temporary file, `fsync`, rename, `fsync` the
  directory, and preserve the file's permissions. Keys sugarlime does not
  recognise are kept and stay editable.
- `internal/schema` generated from `settings_values.md` by `cmd/gen-schema`, so
  new Claude Code settings arrive with `make gen`.
- Release pipeline: a `v*` tag builds static binaries for macOS and Linux on
  arm64 and x86-64, plus `.deb`, `.rpm` and `.apk` packages and checksums, and
  publishes them as a GitHub Release.
- `-version` reports the build's version, commit and date.
