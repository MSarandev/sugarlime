# Contributing

Thanks for looking. Bug reports, small fixes and new ideas are all welcome.

## Getting set up

Needs Go 1.27 or later.

```sh
git clone <your fork>
cd sugarlime
make test
make build && ./sugarlime
```

Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) first if you are changing more
than one file. It explains the two decisions the rest of the code follows from:
the settings file is held as an order-preserving tree, and the list of settings
keys is generated from the Claude Code reference rather than hand-written.

## Before you open a pull request

```sh
make fmt    # gofmt -l -w .
make test   # go test ./...
make lint   # golangci-lint run, if you have it
```

CI runs the same checks on macOS and Linux, with `-race`, and fails if the
checked-in registry does not match `settings_values.md`.

It also fails if coverage drops below 85%, or if the coverage badge in the
README claims more than the suite actually measures. `make cover` prints the
number; if your change moves it, update the badge in `README.md` to match:

```markdown
[![Coverage](https://img.shields.io/badge/coverage-91.9%25-brightgreen)](#development)
```

## Claude Code added a setting

Do not edit `internal/schema/registry_gen.go`. Replace `settings_values.md` with
the current reference and regenerate:

```sh
make gen
make test
```

If the new key gets the wrong editor, fix the inference in
`cmd/gen-schema/main.go` (`inferKind`) rather than special-casing the key in the
UI, and add a case to `cmd/gen-schema/main_test.go` covering the type line that
confused it. A key that genuinely has no simple shape is meant to land on
`KindRaw` and get the JSON editor.

## Where things live

- A new way to find settings files belongs in `internal/config/discover.go`,
  with a test in `discover_test.go` that lays out a fake home directory.
- A change to the opening screen belongs in `internal/ui/files.go`.
- `internal/ui/logo/logo.txt` is embedded as authored, colour escapes included.

## House style

- Comments earn their place. Say why something is the way it is, not what the
  line does; one line is usually enough, and no comment at all is the default.
- Keep validation out of `internal/ui`. Rules about what a value may be belong
  in `internal/permissions`, `internal/sandbox` or the generator, where they can
  be tested without a terminal.
- Keep `View` pure. It reads state and returns a string; anything that changes
  state belongs in an action.
- Prefer a test that drives the app through key presses over one that reaches
  into its fields. `internal/ui/helper_test.go` has the helpers for it.
- Error messages are read by someone mid-task: say what went wrong and what
  would work instead.

## Adding a bespoke editor

Complex keys (`hooks`, `statusLine`, `modelPicker`, `sshConfigs`) are edited as
JSON today. A form for one of them is a welcome change. Keep it behind the same
`openField` dispatch, keep the JSON editor as the fallback for shapes the form
does not cover, and make sure a file that already holds an unusual shape for
that key can still be opened and saved without being rewritten.

## Reporting a bug

Include the Claude Code setting involved, what you did, what you expected, and
what happened. `sugarlime -list` output and your terminal size help. Please do
not paste a whole settings file — redact anything private, and trim it to the
keys that matter.
