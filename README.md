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
| `squad new [name]` | Create a preset: blank, or from `--from <preset\|initial\|current>` or `--template <id>` |
| `squad clone <preset> <new>` · `squad rename` · `squad rm` | Copy, rename or delete a preset (Initial is never changed) |
| `squad edit <preset> [agent]` | Edit in the terminal interface (`--raw`: in `$EDITOR`; `--prompt-file <file.md>`: replace the agent's prompt with the text of a Markdown file) |
| `squad templates` | The five starter presets that ship with squad |
| `squad export [preset]` | Write a preset to `<name>.squad.json` (`-o -` for stdout, `--all`, `--gist` for a secret gist via `gh`) |
| `squad import <file\|https-link\|gist:id\|->` | Add a shared preset after a preview; it is never equipped (`--as`, `--force`, `--allow-hooks`, `--accept-risks`) |
| `squad card <agent>` | An agent as a card: role, model, tools, permission strength (`--preset`) |
| `squad pin <preset>` | Pin a preset to this project (a `.squadup` file with its name; `--clear` removes it). `squad run` with no name then uses the nearest one |
| `squad statusline enable\|disable\|status` | Show ◆ and the equipped preset in the Claude Code status line; your own line keeps running first |
| `squad log` | Operations that changed your config, newest first |
| `squad initial [--recapture]` | Show your original setup, or capture it again |
| `squad doctor` | Check that Claude Code is installed and logged in |

## The interface

Run `squad` in a terminal. Arrow keys or the mouse select, `enter` opens, `e` equips (with a preview), `t` tries a preset for one session, `n` creates, `i` imports, `x` exports, `?` helps. The interface animates; set `SQUADUP_NO_ANIM=1` to keep it still. `NO_COLOR`, `CI`, a dumb terminal or a pipe get plain text instead.

## Claude Code plugin

`plugin/` holds a small Claude Code plugin, and `.claude-plugin/marketplace.json` makes this repository a marketplace for it. Once the repository is published:

```
/plugin marketplace add EdrenaVosh/squadup
/plugin install squadup@squadup
```

`/squadup:squad` then shows the equipped preset and the list; `/squadup:squad bug-hunt` switches. It only calls the `squad` tool, so squad has to be installed. Try it before publishing with `claude plugin marketplace add .` from this folder.

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
