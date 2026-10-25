# Records docs/assets/demo.gif with vhs (https://github.com/charmbracelet/vhs):
# a new preset from the Bug Hunter template, then, in the shell, the preset
# used to open Claude Code with `squad run`.
#
#   brew install vhs
#   make build && make sandbox-reset      # an empty sandbox: no presets yet
#   vhs docs/demo.tape
#
# Everything runs on the sandbox, and HOME points into it, so the prompt is a
# bare "$" and nothing in the recording names your account or folders.
#
# The last step starts the real Claude Code with the preset and the recording
# ends there. Claude keeps its login in your real home, so while hidden the
# script defines a small `squad` function that gives `squad run` your real HOME
# back (saved in REAL_HOME before it is pointed into the sandbox); every other
# command stays in the sandbox, and squad run writes nothing to any Claude
# config. What is typed on screen is just `squad run bug-hunter`. Be logged in
# to claude before recording. Nothing is sent to Claude, so it costs no tokens.
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
Type `squad() { if [ "$1" = run ]; then env -u CLAUDE_CONFIG_DIR HOME=$REAL_HOME command squad "$@"; else command squad "$@"; fi; }`
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

# 5. Use the preset for this session with squad run; the recording ends on Claude Code open
Type "squad run bug-hunter"
Sleep 1500ms
Enter
Sleep 6s
