# Decisions

Deviations from PLAN.md and facts verified against the real machine.

## Stage 0

- **Module path.** `github.com/EdrenaVosh/squadup`. The owner login was read from the author's GitHub account (read-only profile lookup) and matches the project link the author gave.
- **Toolchain.** Installed with Homebrew on 2026-10-24: Go 1.27.3, golangci-lint 2.15.0, GoReleaser 2.x. `go.mod` pins `go 1.27.3`.
- **Claude Code 2.1.309 flags checked with `claude --help`.** `--agents <json>`, `--agent <agent>`, `--settings <file-or-json>` and `--setting-sources <sources>` exist. `--setting-sources` is the candidate for the `--isolated` flag of `squad run` (stage 3); it is not yet verified to hide global agents.
- **Login is per config directory.** `CLAUDE_CONFIG_DIR=<empty dir> claude auth status` prints `loggedIn: false`, `authMethod: none` and exits with code 1. Consequences: `make try-claude` asks for a fresh login inside the sandbox, and `squad doctor` run under the sandbox reports "not logged in". The JSON is still printed on exit code 1, so the parser reads stdout regardless of the exit code.
- **Test stub.** `claude` is replaced in tests by `testdata/fakeclaude`, compiled once per test run and put first on `PATH`.
- **CI is unverified.** `.github/workflows/ci.yml` cannot run until the repository is pushed (stage 8). Action versions (`checkout@v4`, `setup-go@v5`, `golangci-lint-action@v8`) should be re-checked then.
- **`squadup` alias and Homebrew tap** are release concerns and land in stage 7; the GoReleaser config currently builds `squad` only.
