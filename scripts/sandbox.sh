#!/bin/sh
# Creates a throwaway Claude Code config that looks like a real user's setup:
# three agents plus a settings.json with managed and foreign keys. Nothing here
# touches the real ~/.claude. Existing sandboxes are left as they are.
set -eu

sandbox="${1:?usage: sandbox.sh <sandbox-dir>}"
claude_dir="$sandbox/claude"
home_dir="$sandbox/squadup"

mkdir -p "$home_dir"
if [ -d "$claude_dir" ]; then
  exit 0
fi

mkdir -p "$claude_dir/agents"

cat > "$claude_dir/agents/reviewer.md" <<'EOF'
---
name: reviewer
description: Reviews diffs for bugs and risky changes. Use after any code change, before commit.
tools: Read, Grep, Glob
model: sonnet
color: green
---
You are a strict code reviewer. Report findings ranked by severity.
EOF

cat > "$claude_dir/agents/tester.md" <<'EOF'
---
name: tester
description: Writes and runs tests. Use when a change needs test coverage or a test fails.
model: haiku
color: yellow
---
You write focused tests and report what they prove.
EOF

cat > "$claude_dir/agents/docs-writer.md" <<'EOF'
---
name: docs-writer
description: Updates README and docs. Use after a user-facing change.
color: cyan
---
You keep documentation short, accurate and current.
EOF

cat > "$claude_dir/settings.json" <<'EOF'
{
  "theme": "dark",
  "model": "opus",
  "env": {
    "SANDBOX_MARKER": "squadup-sandbox"
  },
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(npm run *)", "Bash(git status)", "Bash(git diff *)", "Read(./docs/**)"],
    "ask": ["Bash(git push *)"],
    "deny": ["Read(./.env)", "Read(./.env.*)"]
  },
  "hooks": {
    "SessionStart": [
      { "hooks": [{ "type": "command", "command": "true" }] }
    ]
  },
  "effortLevel": "high"
}
EOF

echo "sandbox ready: $sandbox"
