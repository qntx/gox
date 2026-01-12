package cli

import (
	"runtime"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
)

func TestSplitRunArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPkgs     []string
		wantProgArgs []string
	}{
		{
			name:         "no args",
			args:         nil,
			wantPkgs:     nil,
			wantProgArgs: nil,
		},
		{
			name:         "package only",
			args:         []string{"."},
			wantPkgs:     []string{"."},
			wantProgArgs: nil,
		},
		{
			name:         "package with separator",
			args:         []string{".", "--"},
			wantPkgs:     []string{"."},
			wantProgArgs: nil,
		},
		{
			name:         "package with prog args",
			args:         []string{".", "--", "-v", "--config", "test.json"},
			wantPkgs:     []string{"."},
			wantProgArgs: []string{"-v", "--config", "test.json"},
		},
		{
			name:         "multiple packages with prog args",
			args:         []string{"./cmd/app", "./pkg/lib", "--", "arg1", "arg2"},
			wantPkgs:     []string{"./cmd/app", "./pkg/lib"},
			wantProgArgs: []string{"arg1", "arg2"},
		},
		{
			name:         "separator only",
			args:         []string{"--"},
			wantPkgs:     nil,
			wantProgArgs: nil,
		},
		{
			name:         "prog args only",
			args:         []string{"--", "-h"},
			wantPkgs:     nil,
			wantProgArgs: []string{"-h"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs, progArgs := splitRunArgs(tt.args)

			if !slices.Equal(pkgs, tt.wantPkgs) {
				t.Errorf("pkgs = %v, want %v", pkgs, tt.wantPkgs)
			}
			if !slices.Equal(progArgs, tt.wantProgArgs) {
				t.Errorf("progArgs = %v, want %v", progArgs, tt.wantProgArgs)
			}
		})
	}
}

func TestValidateRunTarget(t *testing.T) {
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
			err := validateRunTarget(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRunTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyRunFlagOverrides(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		setup    func(*runFlags)
		check    func(*testing.T, *build.Options)
	}{
		{
			name:     "zig-version override",
			flagName: "zig-version",
			setup:    func(f *runFlags) { f.opts.ZigVersion = "0.11.0" },
			check: func(t *testing.T, o *build.Options) {
				if o.ZigVersion != "0.11.0" {
					t.Errorf("ZigVersion = %q, want 0.11.0", o.ZigVersion)
				}
			},
		},
		{
			name:     "linkmode override",
			flagName: "linkmode",
			setup:    func(f *runFlags) { f.linkMode = "static" },
			check: func(t *testing.T, o *build.Options) {
				if o.LinkMode != build.LinkStatic {
					t.Errorf("LinkMode = %q, want static", o.LinkMode)
				}
			},
		},
		{
			name:     "verbose override",
			flagName: "verbose",
			setup:    func(f *runFlags) { f.opts.Verbose = true },
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
			cmd.Flags().String("exec", "", "")
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

			oldFlags := rFlags
			defer func() { rFlags = oldFlags }()
			rFlags = runFlags{}
			tt.setup(&rFlags)

			opts := &build.Options{}
			applyRunFlagOverrides(cmd, opts)

			tt.check(t, opts)
		})
	}
}

func TestApplyRunFlagOverrides_ClearsInvalidFields(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("target", "", "")
	cmd.Flags().String("exec", "", "")
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

	applyRunFlagOverrides(cmd, opts)

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

func TestRunCmd_Flags(t *testing.T) {
	expectedFlags := []string{
		"config", "target", "exec", "zig-version", "linkmode",
		"include", "lib", "link", "pkg", "flags", "verbose",
	}

	for _, name := range expectedFlags {
		t.Run(name, func(t *testing.T) {
			if runCmd.Flags().Lookup(name) == nil {
				t.Errorf("missing flag: %s", name)
			}
		})
	}
}

func TestRunCmd_ShortFlags(t *testing.T) {
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
			flag := runCmd.Flags().Lookup(long)
			if flag == nil {
				t.Fatalf("flag %s not found", long)
			}
			if flag.Shorthand != short {
				t.Errorf("flag %s shorthand = %q, want %q", long, flag.Shorthand, short)
			}
		})
	}
}

func TestRunCmd_NoUnsupportedFlags(t *testing.T) {
	unsupportedFlags := []string{
		"os", "arch", "output", "prefix", "pack", "no-rpath", "strip", "parallel",
	}

	for _, name := range unsupportedFlags {
		t.Run(name, func(t *testing.T) {
			if runCmd.Flags().Lookup(name) != nil {
				t.Errorf("run command should not have flag: %s", name)
			}
		})
	}
}
