package cli

import "testing"

func TestZigCmd_Subcommands(t *testing.T) {
	subcommands := []string{"update", "list", "clean"}

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, cmd := range zigCmd.Commands() {
				if cmd.Name() == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing subcommand: %s", name)
			}
		})
	}
}

func TestZigUpdateCmd_Args(t *testing.T) {
	// Should accept 0 or 1 argument
	if err := zigUpdateCmd.Args(zigUpdateCmd, nil); err != nil {
		t.Errorf("Args(nil) error = %v", err)
	}
	if err := zigUpdateCmd.Args(zigUpdateCmd, []string{"0.15.0"}); err != nil {
		t.Errorf("Args([0.15.0]) error = %v", err)
	}
	if err := zigUpdateCmd.Args(zigUpdateCmd, []string{"0.15.0", "0.14.0"}); err == nil {
		t.Error("Args([0.15.0, 0.14.0]) should return error")
	}
}

func TestZigUpdateCmd_ForceFlag(t *testing.T) {
	flag := zigUpdateCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("missing --force flag")
	}
	if flag.Shorthand != "f" {
		t.Errorf("force shorthand = %q, want 'f'", flag.Shorthand)
	}
}

func TestZigCleanCmd_Args(t *testing.T) {
	// Should accept 0 or 1 argument
	if err := zigCleanCmd.Args(zigCleanCmd, nil); err != nil {
		t.Errorf("Args(nil) error = %v", err)
	}
	if err := zigCleanCmd.Args(zigCleanCmd, []string{"0.15.0"}); err != nil {
		t.Errorf("Args([0.15.0]) error = %v", err)
	}
	if err := zigCleanCmd.Args(zigCleanCmd, []string{"a", "b"}); err == nil {
		t.Error("Args([a, b]) should return error")
	}
}

func TestZigListCmd_NoArgs(t *testing.T) {
	// zigListCmd has no Args validator (accepts any args by default)
	// Just verify the command exists and has correct Use
	if zigListCmd.Use != "list" {
		t.Errorf("Use = %q, want 'list'", zigListCmd.Use)
	}
}
