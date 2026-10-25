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
| `squad flow <preset>` | Show or set how the agents work as a team: `--order`, or `--gate <agent> --fix <agent> [--rounds n]` for a review loop, `--off` |
| `squad usage` | The tokens each agent of a Claude Code session has spent, read from its own transcripts (`--watch` follows it live in the interface, `--json`, `--session`, `--dir`) |
| `squad notes` | Show the squad's shared notes of this project (`--clear` starts the next task fresh) |
| `squad pin <preset>` | Pin a preset to this project (a `.squadup` file with its name; `--clear` removes it). `squad run` with no name then uses the nearest one |
| `squad statusline enable\|disable\|status` | Show ◆ and the equipped preset in the Claude Code status line; your own line keeps running first |
| `squad log` | Operations that changed your config, newest first |
| `squad initial [--recapture]` | Show your original setup, or capture it again |
| `squad doctor` | Check that Claude Code is installed and logged in |

## The interface

Run `squad` in a terminal. Arrow keys or the mouse select, `enter` opens, `e` equips (with a preview), `t` tries a preset for one session, `n` creates, `i` imports, `x` exports, `?` helps. The interface animates; set `SQUADUP_NO_ANIM=1` to keep it still. `NO_COLOR`, `CI`, a dumb terminal or a pipe get plain text instead.

## Squads: shared notes, a flow and a review loop

Claude Code starts every subagent with a clean window: it sees only the task the main agent gives it. squadup adds the rest in the prompts it hands to Claude Code (your stored files stay as you wrote them).

- **Shared context.** Each agent has a switch, *Shared context*, on by default (`enter` or `space` in the agent editor; `squad edit <preset> <agent> --isolate` / `--share`). A sharing agent reads `.squad/notes.md` first and leaves what it learned last. An isolated agent neither reads nor writes it, and the main agent is told to hand it the task or the diff only, which is what an independent code review wants. It takes two agents for any of this to matter.
- **The notes are tiny on purpose.** At most 25 lines of 100 characters, one tagged fact per line: `D` decision and why, `X` tried and ruled out, `!` open defect with `path:line` and an owner, `?` question, `>` a one-line handoff. Code is never pasted and nothing git already holds is repeated; fixed items are deleted. The instruction costs an agent about 130 tokens. `squad notes` shows the file, `squad notes --clear` starts fresh.
- **A flow** makes the agents a team: the main agent hands work down the slots in order. In the preset screen, *Team flow* offers *In order* and *Review loop*: pick who checks (the gate) and who fixes. The gate runs last; the main agent then counts the `!` lines, sends only those to the fixer, and runs the gate again, up to the number of rounds you set (3 by default), then stops and shows what is still open.
- **Where it runs.** The flow is told to the main agent of the preset. With no main agent, `squad run` passes it to Claude Code on the command line; an equipped preset needs a main agent to carry it (the notes work either way).
- **A read-only agent** (tools without Write, Edit or Bash) can read the notes but not change them, so it ends its answer with the lines to add and the main agent records them.

## Tokens, live

`squad usage --watch` (or `u` on the main screen) opens a live report of what each agent of the newest Claude Code session in the folder has spent: the main session and every subagent run, with its state (a working mascot while it runs), runs, tokens sent, read from the cache, produced, the total, its share of the squad, a sparkline of the pace and the cache hit rate. `z` starts counting from now, which measures one task. The main screen has the same numbers as a *TOKENS · LIVE* panel, and `squad run` prints the table when claude exits.

The numbers are not estimated: Claude Code records the API's own token counts for every message, and squadup reads them from `~/.claude/projects/<project>/<session>.jsonl` and `<session>/subagents/agent-<id>.jsonl`. Like Claude Code's own count, TOTAL is fresh input plus cache writes plus output; cache reads, which cost a fraction, are shown beside it.

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
