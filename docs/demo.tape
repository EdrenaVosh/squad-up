# Records docs/assets/demo.gif with vhs (https://github.com/charmbracelet/vhs):
# a new preset from the Bug Hunter template, then, in the shell, the preset
# used for a session with `squad run`, with a check that its agents are there.
#
#   brew install vhs
#   make build && make sandbox-reset      # an empty sandbox: no presets yet
#   vhs docs/demo.tape
#
# Everything runs on the sandbox, and HOME points into it, so the prompt is a
# bare "$" and nothing in the recording names your account or folders.
#
# The last command asks the real Claude Code (haiku, one short question, a few
# cents) whether the agents of the preset are available. Claude keeps its login
# in your real home, so that command gets your real HOME back (saved in
# REAL_HOME before it is pointed into the sandbox); squad run writes nothing to
# any Claude config. Be logged in to claude before recording.
Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1560
Set Height 940
Set Padding 20
Set TypingSpeed 70ms
Set Theme "Catppuccin Mocha"

Hide
Type "export REAL_HOME=$HOME"
Enter
Type "export PATH=$PWD/bin:$PATH HOME=$PWD/.sandbox CLAUDE_CONFIG_DIR=$PWD/.sandbox/claude SQUAD_UP_HOME=$PWD/.sandbox/squad-up PS1='$ '"
Enter
Type "squad list > /dev/null && clear"
Enter
Show

# 1. The interface, with nothing in it yet but your original setup
Type "squad"
Enter
Sleep 3s

# 2. New preset: the first option is a template
Type "n"
Sleep 1s
Type "Bug Hunter"
Sleep 600ms
Enter
Sleep 1800ms
Enter
Sleep 2s

# 3. Bug Hunter: four agents
Enter
Sleep 3s
Escape
Sleep 1500ms

# 4. Back to the shell
Type "q"
Sleep 1s

# 5. Use the preset for this session with squad run, and ask Claude whether its agents are there
Type@6ms `env -u CLAUDE_CONFIG_DIR HOME=$REAL_HOME squad run bug-hunter -- -p "Are these subagents available to you: debugger, fixer, reviewer, tester? Answer with one line per name, like: name - yes or name - no. Nothing else." --model haiku --no-session-persistence`
Sleep 1500ms
Enter
Sleep 25s
