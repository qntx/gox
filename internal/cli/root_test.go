package cli

import "testing"

func TestRootCmd(t *testing.T) {
	t.Run("use", func(t *testing.T) {
		if rootCmd.Use != "gox" {
			t.Errorf("Use = %q, want 'gox'", rootCmd.Use)
		}
	})

	t.Run("has subcommands", func(t *testing.T) {
		if len(rootCmd.Commands()) == 0 {
			t.Error("rootCmd has no subcommands")
		}
	})

	t.Run("has build command", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "build" {
				found = true
				break
			}
		}
		if !found {
			t.Error("missing 'build' subcommand")
		}
	})

	t.Run("has pkg command", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "pkg" {
				found = true
				break
			}
		}
		if !found {
			t.Error("missing 'pkg' subcommand")
		}
	})

	t.Run("has zig command", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "zig" {
				found = true
				break
			}
		}
		if !found {
			t.Error("missing 'zig' subcommand")
		}
	})
}

func TestBrandColors(t *testing.T) {
	// Verify brand colors are defined (non-empty)
	if brandPrimary == "" {
		t.Error("brandPrimary not defined")
	}
	if brandMuted == "" {
		t.Error("brandMuted not defined")
	}
}
