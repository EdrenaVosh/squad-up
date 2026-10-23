# squadup

Presets of agent teams for [Claude Code](https://code.claude.com): switch them with one command and always get your original setup back.

> Work in progress. The full README arrives with the first release.

## Commands

| Command | What it does |
| --- | --- |
| `squad list` | Presets; the equipped one is marked, Initial is last |
| `squad status` | Equipped preset, manual edits, the `claude` in use |
| `squad capture <name>` | Save the current Claude Code setup as a preset |
| `squad use <preset>` | Equip a preset (`--dry-run`, `--yes`, `--force`, `--save-edits`, `--discard-edits`) |
| `squad restore` | Return to your original setup (Initial) |
| `squad undo [--to <id>]` | Go back one step, or to a point from `squad log` |
| `squad run <preset> [-- args]` | Start `claude` with a preset for one session, writing nothing to `~/.claude` (`--dry-run`) |
| `squad models [--probe]` | Which models you can use: catalogue, your policy, and an optional probe |
| `squad lint [preset]` | Check agents and presets, and estimate their cost in context (`--strict`, `--json`) |
| `squad log` | Operations that changed your config, newest first |
| `squad initial [--recapture]` | Show your original setup, or capture it again |
| `squad doctor` | Check that Claude Code is installed and logged in |

## Local development

Everything below runs against a throwaway sandbox, never your real `~/.claude`.

| Command | What it does |
| --- | --- |
| `make build` | Build `./bin/squad` |
| `make test` | Lint, vet and all tests |
| `make sandbox` | Create `./.sandbox/claude` with an example third-party setup (3 agents, `settings.json` with permissions and foreign keys) and an empty `./.sandbox/squadup` |
| `make sandbox-presets` | Add three demo presets to the sandbox so that `squad use` has something to switch to |
| `make try` | Run `squad` with `CLAUDE_CONFIG_DIR` and `SQUADUP_HOME` pointing into the sandbox: `make try ARGS="doctor"` |
| `make try-claude` | Open the real `claude` on the sandbox config (login is per config directory, so you sign in once more) |
| `make sandbox-reset` | Recreate the sandbox from scratch |
| `make snapshot` | Local GoReleaser build of all platforms into `dist/`, nothing published |
| `make install` | Copy `squad` to `~/.local/bin` to try it on your real config |

Requires Go (see `go.mod`), `golangci-lint` v2 and `goreleaser` v2.
