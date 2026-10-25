# Records docs/assets/demo.gif with vhs (https://github.com/charmbracelet/vhs):
# a new preset from the Bug Hunters template, then equipped, then given back.
#
#   brew install vhs
#   make build && make sandbox-reset      # an empty sandbox: no presets yet
#   vhs docs/demo.tape
#
# It runs against the sandbox, never against your real Claude config.
Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1560
Set Height 940
Set Padding 20
Set TypingSpeed 70ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD/bin:$PATH CLAUDE_CONFIG_DIR=$PWD/.sandbox/claude SQUADUP_HOME=$PWD/.sandbox/squadup"
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
Type "bugs"
Sleep 600ms
Enter
Sleep 1800ms
Enter
Sleep 2s

# 3. Bug Hunters: four agents with their roles
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

# 6. Equip it: what would change is shown first
Type "e"
Sleep 2500ms
Enter
Sleep 4500ms
Escape
Sleep 3500ms

# 7. The agents are at work (the orange mascots), then back to how it was
Type "q"
Sleep 600ms
Type "squad restore"
Enter
Sleep 600ms
Type "y"
Sleep 3s
