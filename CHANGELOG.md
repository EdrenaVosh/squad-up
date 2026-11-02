# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-11-02

### Added

- **New releases are announced.** Once a day squad asks GitHub for the newest
  release; when there is one, a blinking orange notice in the top edge of the
  main screen and a line after commands say so. It sends nothing but the
  request, runs only at a terminal (never in scripts, CI or the status line),
  and turns off with `squad update --auto-check off` or
  `SQUAD_UP_NO_UPDATE_CHECK=1`; squad says so the first time.
- **`squad update` updates.** It runs `brew upgrade`, `go install` or the
  install script, the way squad was installed, and says when you already have
  the newest release. `--print` only shows the command.
- **Commands in the help.** `?` on the main screen now also lists the main
  shell commands (`squad run`, `use`, `restore`, `update`…) and every key of
  the main screen; on a small terminal it scrolls.

### Changed

- **Every screen fills the terminal.** The colored frame of every screen takes
  the whole terminal, and the content sits in its middle; the main screen, the
  agent editor and the usage report no longer stop growing at 200×44.
- **Bigger windows and dialogs.** Screens lay their content out in up to
  180×54 (was 120×36), and the dialogs (names, questions, choices, help) are
  half as wide and half as tall again, with their content in the middle.
- **The preset screen fills the terminal.** It starts at the top left like the
  main screen; the columns of its agents table widen with the screen, and the
  line under the table's header ends where the table does.
- **A wider agent editor.** The fields on the left are half as wide again.
- **A wider preset list.** On a big terminal the list on the left of the main
  screen is 30% wider.

### Fixed

- **No flash of the main screen between steps.** Closing a dialog that opens
  the next screen (a new preset's name, "Blank preset") no longer shows the
  main screen for a moment in between.

## [0.1.0] - 2026-10-25

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

[Unreleased]: https://github.com/EdrenaVosh/squad-up/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/EdrenaVosh/squad-up/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/EdrenaVosh/squad-up/releases/tag/v0.1.0
