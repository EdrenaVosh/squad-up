# squadup

Presets of agent teams for [Claude Code](https://code.claude.com): switch them with one command and always get your original setup back.

> Work in progress. The full README arrives with the first release.

## Local development

Everything below runs against a throwaway sandbox, never your real `~/.claude`.

| Command | What it does |
| --- | --- |
| `make build` | Build `./bin/squad` |
| `make test` | Lint, vet and all tests |
| `make sandbox` | Create `./.sandbox/claude` with an example third-party setup (3 agents, `settings.json` with permissions and foreign keys) and an empty `./.sandbox/squadup` |
| `make try` | Run `squad` with `CLAUDE_CONFIG_DIR` and `SQUADUP_HOME` pointing into the sandbox: `make try ARGS="doctor"` |
| `make try-claude` | Open the real `claude` on the sandbox config (login is per config directory, so you sign in once more) |
| `make sandbox-reset` | Recreate the sandbox from scratch |
| `make snapshot` | Local GoReleaser build of all platforms into `dist/`, nothing published |
| `make install` | Copy `squad` to `~/.local/bin` to try it on your real config |

Requires Go (see `go.mod`), `golangci-lint` v2 and `goreleaser` v2.
