# ============================================================
# gox Interactive Mode Demo
# Demonstrates CGO cross-compilation using gox TUI
# ============================================================

Output gox.gif

Set Shell "bash"
Set FontSize 28
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set Padding 30
Set BorderRadius 10
Set TypingSpeed 50ms

Hide
Type "cd example/minimal && clear"
Enter
Show

Sleep 1s

# ============================================================
# Step 1: Launch Interactive Build
# ============================================================

Type "gox build -v"
Sleep 500ms
Enter
Sleep 2s

# ============================================================
# Step 2: Select Target Platform (linux/arm64)
# ============================================================

Down
Sleep 1600ms
Tab
Sleep 1600ms
Tab
Sleep 1600ms

# ============================================================
# Step 3: Skip Library Configuration
# ============================================================

Tab
Sleep 1600ms
Tab
Sleep 1600ms
Tab
Sleep 1600ms

# ============================================================
# Step 4: Set Output Path
# ============================================================

Tab
Enter
Sleep 1s

# ============================================================
# Step 5: Build Process
# ============================================================

Sleep 1s

# ============================================================
# Step 6: Verify Build Artifacts
# ============================================================

Type "ls -lh && file minimal"
Sleep 500ms
Enter
Sleep 6s
