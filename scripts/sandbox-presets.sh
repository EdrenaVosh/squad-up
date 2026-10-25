#!/bin/sh
# Adds three demo presets to the sandbox squad-up home so that `squad use` has
# something to switch to. Existing presets are left alone. Nothing here touches
# the real ~/.squad-up.
set -eu

sandbox="${1:?usage: sandbox-presets.sh <sandbox-dir>}"
presets="$sandbox/squad-up/presets"

write_agent() { # dir name description body
  mkdir -p "$1/agents"
  printf -- '---\nname: %s\ndescription: %s\n---\n%s\n' "$2" "$3" "$4" > "$1/agents/$2.md"
}

if [ ! -d "$presets/solo-indie" ]; then
  dir="$presets/solo-indie"
  mkdir -p "$dir"
  cat > "$dir/preset.yaml" <<'YAML'
schema: 1
name: solo-indie
title: Solo Indie
description: Planner, reviewer and tester for one-person apps
accent: cyan
mode: replace
main: null
agents: [planner, reviewer, tester]
settings:
  model: sonnet
  effortLevel: high
  permissions:
    defaultMode: acceptEdits
    allow:
      - Bash(npm run *)
    ask:
      - Bash(git push *)
    deny:
      - Read(./.env)
      - Read(./.env.*)
YAML
  write_agent "$dir" planner "Breaks a task into steps. Use before starting any large change." "You plan the work and list the steps in order."
  write_agent "$dir" reviewer "Reviews diffs for bugs. Use after any code change, before commit." "You are a strict reviewer. Report findings ranked by severity."
  write_agent "$dir" tester "Writes and runs tests. Use when a change needs coverage." "You write focused tests and report what they prove."
fi

if [ ! -d "$presets/bug-hunt" ]; then
  dir="$presets/bug-hunt"
  mkdir -p "$dir"
  cat > "$dir/preset.yaml" <<'YAML'
schema: 1
name: bug-hunt
title: Bug Hunt
description: A debugger and a tester for tracking down a failure
accent: red
mode: replace
main: debugger
agents: [debugger, tester]
settings:
  model: haiku
YAML
  write_agent "$dir" debugger "Finds the root cause of a failure. Use when a test or the app fails." "You reproduce the failure first, then narrow down the cause."
  write_agent "$dir" tester "Writes a failing test for a bug. Use before fixing it." "You write the smallest test that reproduces the bug."
fi

if [ ! -d "$presets/extra-helper" ]; then
  dir="$presets/extra-helper"
  mkdir -p "$dir"
  cat > "$dir/preset.yaml" <<'YAML'
schema: 1
name: extra-helper
title: Extra Helper
description: Adds one agent and keeps your own agents (mode extend)
mode: extend
main: null
agents: [helper]
YAML
  write_agent "$dir" helper "Answers quick questions about the repo. Use when you need a short explanation." "You answer briefly and point to the files."
fi

echo "demo presets ready in $presets"
