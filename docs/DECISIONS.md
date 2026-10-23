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

## Stage 1

- **`claude auth status` migrates the config; `claude -v` does not.** Verified on Claude Code 2.1.309 against a sandbox: `auth status` is a full Claude Code startup. It rewrote `settings.json` (reformatted it, turned `"model": "opus"` into `"opus[1m]"`, moved `theme` to the end) and created `.claude.json`, `backups/`, `projects/`, `sessions/`. Consequences: (1) squad captures Initial *before* any command runs, so the snapshot is never taken after such a migration by squad itself; (2) `doctor` and `status` call `auth status`, which has the same effect as the user starting `claude` once; (3) in stage 2 drift detection for `settings.json` must compare the parsed values of the managed keys, not bytes, because Claude Code reformats the file on its own; (4) `squad restore` writes the snapshot values back, so it also undoes such a migration of a managed key.
- **First-run summary goes to stderr**, so `--json` output on stdout stays machine-readable. The plan only says "before the command output".
- **Initial layout.** Besides `preset.yaml` and `agents/**/*.md` the snapshot holds `initial.json` (what was captured: existence of the agents dir and settings.json, per-file sha256 and validity, raw JSON text of each managed key) and `settings.orig.json` (the original settings.json byte for byte, only if it existed). Restore needs both. Files are 0400. `settings.orig.json` may contain the user's `env` values; nothing is ever printed from it and the directory stays under `~/.squadup`.
- **Managed values are kept as raw JSON text** inside `initial.json` (a string per key). `encoding/json` would compact a `RawMessage`, losing the original formatting that byte-exact restore needs.
- **goccy/go-yaml v1.19.2 panics** when a struct has an `inline` map field and decoding is `Strict()`. Unknown keys of the `permissions` object are therefore stored under an explicit `extra:` key in `preset.yaml`. The library's last release is from January 2026; `tidwall/sjson` has had no release since 2022 (stable, but noted for stage 2).
- **Frontmatter key order** follows the field list in the plan's table of Claude Code fields (`... permissionMode, maxTurns, ..., effort, ..., color`). The example agent in the plan puts `effort` before `permissionMode`; the table wins. An untouched agent file is always written back unchanged, so the order only matters for files squad rewrites.
- **Agent scanning** follows symlinked `.md` files and skips symlinked directories and dangling links. Files are sorted by path; when two files carry the same agent name the first one by path wins in `capture` and the other is reported as skipped.
- **Exit code 2** is used for an invalid or reserved preset name (validation failure); unknown commands and flag errors stay at 1.
- **Terminal detection** is a character-device check that excludes the null device. It replaces a terminal library until the Charm stack arrives in stage 4.
- **Recapture** is refused while a preset is equipped: the live config then contains the preset, not the original setup.
- **Not verifiable on this machine:** Windows behaviour (paths, read-only files, rename over existing files) is covered only by the CI matrix once the repository is published.
