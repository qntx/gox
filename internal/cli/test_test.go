package cli

import (
	"runtime"
	"testing"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
)

func TestSplitTestArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPkgs     []string
		wantTestArgs []string
	}{
		{
			name:         "no args",
			args:         nil,
			wantPkgs:     nil,
			wantTestArgs: nil,
		},
		{
			name:         "package only",
			args:         []string{"./..."},
			wantPkgs:     []string{"./..."},
			wantTestArgs: nil,
		},
		{
			name:         "package with separator",
			args:         []string{".", "--"},
			wantPkgs:     []string{"."},
			wantTestArgs: nil,
		},
		{
			name:         "package with test args",
			args:         []string{"./...", "--", "-v", "-run", "TestFoo"},
			wantPkgs:     []string{"./..."},
			wantTestArgs: []string{"-v", "-run", "TestFoo"},
		},
		{
			name:         "multiple packages with test args",
			args:         []string{"./pkg/a", "./pkg/b", "--", "-cover"},
			wantPkgs:     []string{"./pkg/a", "./pkg/b"},
			wantTestArgs: []string{"-cover"},
		},
		{
			name:         "test args only",
			args:         []string{"--", "-v"},
			wantPkgs:     nil,
			wantTestArgs: []string{"-v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs, testArgs := splitTestArgs(tt.args)

			if !strSliceEqual(pkgs, tt.wantPkgs) {
				t.Errorf("pkgs = %v, want %v", pkgs, tt.wantPkgs)
			}
			if !strSliceEqual(testArgs, tt.wantTestArgs) {
				t.Errorf("testArgs = %v, want %v", testArgs, tt.wantTestArgs)
			}
		})
	}
}

func TestValidateTestTarget(t *testing.T) {
	tests := []struct {
		name    string
		opts    *build.Options
		wantErr bool
	}{
		{
			name:    "empty opts (current platform)",
			opts:    &build.Options{},
			wantErr: false,
		},
		{
			name: "explicit current platform",
			opts: &build.Options{
				GOOS:   runtime.GOOS,
				GOARCH: runtime.GOARCH,
			},
			wantErr: false,
		},
		{
			name: "cross-platform os",
			opts: &build.Options{
				GOOS:   "plan9",
				GOARCH: runtime.GOARCH,
			},
			wantErr: true,
		},
		{
			name: "cross-platform arch",
			opts: &build.Options{
				GOOS:   runtime.GOOS,
				GOARCH: "mips64",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTestTarget(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTestTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyTestFlagOverrides(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		setup    func(*testFlags)
		check    func(*testing.T, *build.Options)
	}{
		{
			name:     "zig-version override",
			flagName: "zig-version",
			setup:    func(f *testFlags) { f.opts.ZigVersion = "0.11.0" },
			check: func(t *testing.T, o *build.Options) {
				if o.ZigVersion != "0.11.0" {
					t.Errorf("ZigVersion = %q, want 0.11.0", o.ZigVersion)
				}
			},
		},
		{
			name:     "linkmode override",
			flagName: "linkmode",
			setup:    func(f *testFlags) { f.linkMode = "static" },
			check: func(t *testing.T, o *build.Options) {
				if o.LinkMode != build.LinkStatic {
					t.Errorf("LinkMode = %q, want static", o.LinkMode)
				}
			},
		},
		{
			name:     "verbose override",
			flagName: "verbose",
			setup:    func(f *testFlags) { f.opts.Verbose = true },
			check: func(t *testing.T, o *build.Options) {
				if !o.Verbose {
					t.Error("Verbose = false, want true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().String("config", "", "")
			cmd.Flags().String("target", "", "")
			cmd.Flags().String("zig-version", "", "")
			cmd.Flags().String("linkmode", "", "")
			cmd.Flags().StringSlice("include", nil, "")
			cmd.Flags().StringSlice("lib", nil, "")
			cmd.Flags().StringSlice("link", nil, "")
			cmd.Flags().StringSlice("pkg", nil, "")
			cmd.Flags().StringSlice("flags", nil, "")
			cmd.Flags().Bool("verbose", false, "")

			switch tt.flagName {
			case "zig-version":
				cmd.Flags().Set(tt.flagName, "0.11.0")
			case "linkmode":
				cmd.Flags().Set(tt.flagName, "static")
			case "verbose":
				cmd.Flags().Set(tt.flagName, "true")
			}

			oldFlags := tFlags
			defer func() { tFlags = oldFlags }()
			tFlags = testFlags{}
			tt.setup(&tFlags)

			opts := &build.Options{}
			applyTestFlagOverrides(cmd, opts)

			tt.check(t, opts)
		})
	}
}

func TestApplyTestFlagOverrides_ClearsInvalidFields(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("target", "", "")
	cmd.Flags().String("zig-version", "", "")
	cmd.Flags().String("linkmode", "", "")
	cmd.Flags().StringSlice("include", nil, "")
	cmd.Flags().StringSlice("lib", nil, "")
	cmd.Flags().StringSlice("link", nil, "")
	cmd.Flags().StringSlice("pkg", nil, "")
	cmd.Flags().StringSlice("flags", nil, "")
	cmd.Flags().Bool("verbose", false, "")

	opts := &build.Options{
		Output:  "/some/path",
		Prefix:  "./dist",
		Pack:    true,
		NoRpath: true,
	}

	applyTestFlagOverrides(cmd, opts)

	if opts.Output != "" {
		t.Errorf("Output = %q, want empty", opts.Output)
	}
	if opts.Prefix != "" {
		t.Errorf("Prefix = %q, want empty", opts.Prefix)
	}
	if opts.Pack {
		t.Error("Pack = true, want false")
	}
	if opts.NoRpath {
		t.Error("NoRpath = true, want false")
	}
}

func TestTestCmd_Flags(t *testing.T) {
	expectedFlags := []string{
		"config", "target", "zig-version", "linkmode",
		"include", "lib", "link", "pkg", "flags", "verbose",
	}

	for _, name := range expectedFlags {
		t.Run(name, func(t *testing.T) {
			if testCmd.Flags().Lookup(name) == nil {
				t.Errorf("missing flag: %s", name)
			}
		})
	}
}

func TestTestCmd_ShortFlags(t *testing.T) {
	shortFlags := map[string]string{
		"c": "config",
		"t": "target",
		"I": "include",
		"L": "lib",
		"l": "link",
		"v": "verbose",
	}

	for short, long := range shortFlags {
		t.Run(short+"->"+long, func(t *testing.T) {
			flag := testCmd.Flags().Lookup(long)
			if flag == nil {
				t.Fatalf("flag %s not found", long)
			}
			if flag.Shorthand != short {
				t.Errorf("flag %s shorthand = %q, want %q", long, flag.Shorthand, short)
			}
		})
	}
}

func strSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
