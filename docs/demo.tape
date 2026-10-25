# Records docs/assets/demo.gif with vhs (https://github.com/charmbracelet/vhs):
# a new preset from the Bug Hunter template, then equipped, then given back.
#
#   brew install vhs
#   make build && make sandbox-reset      # an empty sandbox: no presets yet
#   vhs docs/demo.tape
#
# It runs against the sandbox, never against your real Claude config. HOME is
# pointed into the sandbox, so every path on screen is written as ~/…, and the
# prompt is a bare "$": nothing in the recording names your account or folders.
Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1560
Set Height 940
Set Padding 20
Set TypingSpeed 70ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD/bin:$PATH HOME=$PWD/.sandbox CLAUDE_CONFIG_DIR=$PWD/.sandbox/claude SQUADUP_HOME=$PWD/.sandbox/squadup PS1='$ '"
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

# 3. Bug Hunter: four agents with their roles
Enter
Sleep 3s

# 4. The preset, agent by agent
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 1500ms
Escape
Sleep 2s

# 5. The main screen: the team as tiles, the flow as a diagram
Right
Sleep 900ms
Right
Sleep 900ms
Right
Sleep 1200ms

# 6. Equip it: the screen shows exactly what would change, then y equips
Type "e"
Sleep 4s
Type "y"
Sleep 5s
Escape
Sleep 3500ms

# 7. The team is equipped: the agents are at work (the orange mascots)
Right
Sleep 1500ms
Right
Sleep 2500ms

# 8. Proof in the shell: its agents are now in the Claude config, and restore puts yours back
Type "q"
Sleep 600ms
Type "ls $CLAUDE_CONFIG_DIR/agents"
Enter
Sleep 3s
Type "squad restore --yes"
Enter
Sleep 3s
Type "ls $CLAUDE_CONFIG_DIR/agents"
Enter
Sleep 3s
