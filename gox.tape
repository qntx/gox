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

Type "gox build . -v"
Sleep 500ms
Enter
Sleep 2s

# ============================================================
# Step 2: Select Target Platform (linux/arm64)
# ============================================================

Down
Sleep 2s
Enter
Sleep 2s
Enter
Sleep 2s

# ============================================================
# Step 3: Skip Library Configuration
# ============================================================

Enter
Sleep 2s
Enter
Sleep 2s
Enter
Sleep 2s

# ============================================================
# Step 4: Set Output Path
# ============================================================

Enter
Sleep 2s

# ============================================================
# Step 5: Build Process
# ============================================================

Sleep 2s

# ============================================================
# Step 6: Verify Build Artifacts
# ============================================================

Type "ls -lh"
Sleep 2s
Enter
Type "file minimal"
Sleep 2s
Enter
Sleep 9s
