package cli

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		// Exact match
		{"cuda-linux-amd64", "cuda-linux-amd64", true},
		{"cuda-linux-amd64", "cuda-linux-arm64", false},

		// Prefix wildcard (*suffix)
		{"cuda-linux-amd64", "*-amd64", true},
		{"cuda-linux-amd64", "*-arm64", false},
		{"openssl-3.0", "*-3.0", true},

		// Suffix wildcard (prefix*)
		{"cuda-linux-amd64", "cuda-*", true},
		{"cuda-linux-amd64", "openssl-*", false},
		{"cuda-12.0-linux", "cuda-12*", true},

		// Middle wildcard (prefix*suffix)
		{"cuda-linux-amd64", "cuda-*-amd64", true},
		{"cuda-linux-amd64", "cuda-*-arm64", false},
		{"my-lib-v1.0-linux", "my-lib-*-linux", true},

		// No wildcard
		{"lib", "lib", true},
		{"lib", "libs", false},
		{"lib", "li", false},

		// Edge cases
		{"abc", "*", true},
		{"abc", "abc*", true},
		{"abc", "*abc", true},
		{"", "", true},
	}

	for _, tt := range tests {
		testName := tt.name + "/" + tt.pattern
		t.Run(testName, func(t *testing.T) {
			if got := matchGlob(tt.name, tt.pattern); got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPkgCmd_Subcommands(t *testing.T) {
	subcommands := []string{"list", "clean", "info", "install"}

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, cmd := range pkgCmd.Commands() {
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

func TestPkgCleanCmd_Args(t *testing.T) {
	// Should accept 0 or 1 argument
	if err := pkgCleanCmd.Args(pkgCleanCmd, nil); err != nil {
		t.Errorf("Args(nil) error = %v", err)
	}
	if err := pkgCleanCmd.Args(pkgCleanCmd, []string{"pkg1"}); err != nil {
		t.Errorf("Args([pkg1]) error = %v", err)
	}
	if err := pkgCleanCmd.Args(pkgCleanCmd, []string{"pkg1", "pkg2"}); err == nil {
		t.Error("Args([pkg1, pkg2]) should return error")
	}
}

func TestPkgInfoCmd_Args(t *testing.T) {
	// Should require exactly 1 argument
	if err := pkgInfoCmd.Args(pkgInfoCmd, nil); err == nil {
		t.Error("Args(nil) should return error")
	}
	if err := pkgInfoCmd.Args(pkgInfoCmd, []string{"pkg1"}); err != nil {
		t.Errorf("Args([pkg1]) error = %v", err)
	}
	if err := pkgInfoCmd.Args(pkgInfoCmd, []string{"pkg1", "pkg2"}); err == nil {
		t.Error("Args([pkg1, pkg2]) should return error")
	}
}

func TestPkgInstallCmd_Args(t *testing.T) {
	// Should require at least 1 argument
	if err := pkgInstallCmd.Args(pkgInstallCmd, nil); err == nil {
		t.Error("Args(nil) should return error")
	}
	if err := pkgInstallCmd.Args(pkgInstallCmd, []string{"pkg1"}); err != nil {
		t.Errorf("Args([pkg1]) error = %v", err)
	}
	if err := pkgInstallCmd.Args(pkgInstallCmd, []string{"pkg1", "pkg2"}); err != nil {
		t.Errorf("Args([pkg1, pkg2]) error = %v", err)
	}
}
