# Contributing

Thanks for looking. squadup is a small Go program; the whole contract is in
[`PLAN.md`](PLAN.md), the reasons behind each choice are in
[`docs/DECISIONS.md`](docs/DECISIONS.md).

## Set up

```sh
git clone https://github.com/EdrenaVosh/squadup && cd squadup
make test        # golangci-lint, go vet and go test -race ./...
make build       # bin/squad
```

You need Go (the version in `go.mod`) and [golangci-lint](https://golangci-lint.run).

## Try it without touching your real config

Never develop against your real `~/.claude`. The sandbox targets point squad and
Claude Code at a throwaway folder:

```sh
make sandbox-reset && make sandbox-presets   # a fake Claude config and demo presets
make try                                     # the interface, on the sandbox
make try ARGS="use solo-indie"               # any command, on the sandbox
make try-claude                              # the real claude on the sandbox config
```

The tests do the same: they run against temporary directories and a fake
`claude` (`testdata/fakeclaude`), and a guard refuses any write to the real
config.

## Rules of the house

- **Tests with every change.** Screens have golden files: `go test ./internal/tui -screens`
  rewrites them; read the diff before you commit it.
- **English** in the interface, error messages, the README and code comments.
  An error says what happened and what to do next.
- **No new dependencies** beyond the ones the plan lists, unless there is a
  reason written in `docs/DECISIONS.md`.
- **Verify what you claim about Claude Code** against `claude --help` or its
  documentation before relying on a flag or a file format, and say in
  `docs/DECISIONS.md` what was checked and what was not.
- **Never run a command that publishes** (tags, releases, pushes) as part of a change.
- No telemetry, and nothing is sent anywhere without the user asking for it.

## Pull requests

Keep a change to one idea, run `make test`, and describe what you tried by hand.
Update `CHANGELOG.md` under *Unreleased* for anything a user would notice.
