# Records docs/assets/demo.gif with vhs (https://github.com/charmbracelet/vhs):
#
#   brew install vhs
#   make build && make sandbox-reset && make sandbox-presets
#   vhs docs/demo.tape
#
# It runs against the sandbox, never against your real Claude config.
Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1500
Set Height 900
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD/bin:$PATH CLAUDE_CONFIG_DIR=$PWD/.sandbox/claude SQUADUP_HOME=$PWD/.sandbox/squadup"
Enter
Type "clear"
Enter
Show

Type "squad"
Enter
Sleep 3s

# Look through the presets and the agents of each
Down
Sleep 1.5s
Down
Sleep 1.5s
Right
Sleep 1s
Right
Sleep 1.5s

# The agent editor, with the switch for an independent agent
Type "a"
Sleep 2s
Down 2
Sleep 1s
Escape
Sleep 1s

# Equip the preset: what changes is shown first
Type "e"
Sleep 2.5s
Enter
Sleep 3s
Escape
Sleep 1s

Type "q"
Sleep 500ms
Type "squad status"
Enter
Sleep 2s
Type "squad restore"
Enter
Sleep 3s
