# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - not released yet

The first release.

### Added

- **Presets of agents.** A preset is a named team: agents, a default model and
  effort, a permission profile, an optional main agent. `squad use` equips one
  into `~/.claude` after a backup; `squad restore` and `squad undo` bring back
  your original setup or the last state. Manual edits are detected and never
  overwritten without asking.
- **`squad run`** starts Claude Code with a preset for one session, writing
  nothing to your config.
- **Terminal interface** with the presets, agent tiles, an agent editor (model
  versions, effort, permissions, tools, prompt written, edited in `$EDITOR` or
  loaded from a Markdown file), permission profiles and an equip confirmation.
- **Team flow and review loop:** the main agent hands work down the slots, and
  defects go back to the agent that fixes them for up to N rounds.
- **Independent agents:** a per-agent switch for an agent that must work from
  its task and instructions alone, not swayed by what others reasoned.
- **Live token report** per agent and for the whole squad, read from Claude
  Code's own transcripts (`squad usage`, the *Usage* screen, a panel on the main
  screen, a summary after `squad run`).
- **Models:** the aliases and every documented release by full ID, with a check
  of which ones your account can use (`squad models`).
- **Linter** for agents and presets (`squad lint`).
- **Share presets:** `squad export` and `squad import` (file, https, secret
  gist), with a security preview and `squad card`.
- **Two starter templates**, *Bug Hunter* (debugger, fixer, an independent
  reviewer, tester) and *Feature Creator* (planner, executor, an independent
  reviewer), each with a review loop; starting from one is the first option of
  the *New preset* dialog.
- **Projects and Claude Code:** a `.squad-up` file pins a preset to a project;
  `squad statusline` shows the active preset; a Claude Code plugin adds
  `/squad-up:squad`.
- **Installation:** Homebrew, `install.sh`, `go install`, and release archives
  for macOS, Linux and Windows.

[Unreleased]: https://github.com/EdrenaVosh/squad-up/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EdrenaVosh/squad-up/releases/tag/v0.1.0
