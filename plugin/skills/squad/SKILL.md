---
description: Show which squad-up agent preset is equipped, list the presets, or switch to one
argument-hint: "[preset]"
disable-model-invocation: true
allowed-tools: Bash(command -v squad) Bash(squad status:*) Bash(squad list:*) Bash(squad use:*)
---
You manage the user's squad-up agent presets with the `squad` command-line tool. The user asked:

> $ARGUMENTS

Do this, in order:

1. Run `command -v squad`. If it prints nothing, squad is not installed. Tell the user to install it with
   `curl -fsSL https://raw.githubusercontent.com/EdrenaVosh/squad-up/main/scripts/install.sh | sh`
   and stop.

2. If the request above is empty, show the situation:
   - run `squad status --json` and `squad list --json`;
   - answer in a few lines: the equipped preset, whether anything was edited by hand (`drift`), and the presets with their titles and number of agents, with the equipped one marked;
   - if `pinned` is present and does not match, say that this project pins another preset.

3. If the request is the name of a preset (lowercase letters, digits and dashes that appear in `squad list --json`), switch to it: run `squad use <name> --yes` and report what it printed. The user asked for this switch by name, so it needs no further confirmation.

4. If the request is anything else, treat it as a question about the presets and answer it from `squad status --json` and `squad list --json`.

Rules:
- Never add `--force`, `--discard-edits` or `--save-edits` on your own. If `squad use` stops because of manual edits or conflicting files, show its message and let the user choose.
- Never edit files in `~/.claude` or `~/.squad-up` yourself; only `squad` does that.
- After a switch, say that agent files are picked up within seconds in a running session, while the model and permission settings of a preset are only certain to apply in a new session.
