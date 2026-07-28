# wink demo tape
# Record with: bash demo/record.sh   (from the repo root)
# Requires: brew install vhs   and wink on PATH

Output demo/wink.gif

Require wink

Set Shell bash
Set FontSize 15
Set Width 1100
Set Height 640
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

# start everything from the config
Type "wink up demo/wink.yaml"
Enter
Sleep 2s

# open the TUI
Type "wink"
Enter

# logs stream in, redis crashes at ~7s and goes dead, worker starts warning
Sleep 10s

# search across every service
Type "/"
Sleep 500ms
Type@110ms "error"
Sleep 3s
Escape

# sit on the full picture: 2 running, 1 dead, everything in one place
Sleep 3s
