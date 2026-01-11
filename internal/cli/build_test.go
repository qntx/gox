package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
)

func TestApplyFlagOverrides(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		setup    func(*buildFlags)
		check    func(*testing.T, *build.Options)
	}{
		{
			name:     "os override",
			flagName: "os",
			setup:    func(f *buildFlags) { f.opts.GOOS = "linux" },
			check: func(t *testing.T, o *build.Options) {
				if o.GOOS != "linux" {
					t.Errorf("GOOS = %q, want linux", o.GOOS)
				}
			},
		},
		{
			name:     "arch override",
			flagName: "arch",
			setup:    func(f *buildFlags) { f.opts.GOARCH = "arm64" },
			check: func(t *testing.T, o *build.Options) {
				if o.GOARCH != "arm64" {
					t.Errorf("GOARCH = %q, want arm64", o.GOARCH)
				}
			},
		},
		{
			name:     "output override",
			flagName: "output",
			setup:    func(f *buildFlags) { f.opts.Output = "/tmp/bin" },
			check: func(t *testing.T, o *build.Options) {
				if o.Output != "/tmp/bin" {
					t.Errorf("Output = %q, want /tmp/bin", o.Output)
				}
			},
		},
		{
			name:     "prefix override",
			flagName: "prefix",
			setup:    func(f *buildFlags) { f.opts.Prefix = "./dist" },
			check: func(t *testing.T, o *build.Options) {
				if o.Prefix != "./dist" {
					t.Errorf("Prefix = %q, want ./dist", o.Prefix)
				}
			},
		},
		{
			name:     "strip override",
			flagName: "strip",
			setup:    func(f *buildFlags) { f.opts.Strip = true },
			check: func(t *testing.T, o *build.Options) {
				if !o.Strip {
					t.Error("Strip = false, want true")
				}
			},
		},
		{
			name:     "verbose override",
			flagName: "verbose",
			setup:    func(f *buildFlags) { f.opts.Verbose = true },
			check: func(t *testing.T, o *build.Options) {
				if !o.Verbose {
					t.Error("Verbose = false, want true")
				}
			},
		},
		{
			name:     "pack override",
			flagName: "pack",
			setup:    func(f *buildFlags) { f.opts.Pack = true },
			check: func(t *testing.T, o *build.Options) {
				if !o.Pack {
					t.Error("Pack = false, want true")
				}
			},
		},
		{
			name:     "linkmode override",
			flagName: "linkmode",
			setup:    func(f *buildFlags) { f.linkMode = "static" },
			check: func(t *testing.T, o *build.Options) {
				if o.LinkMode != build.LinkStatic {
					t.Errorf("LinkMode = %q, want static", o.LinkMode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh command with flags
			cmd := &cobra.Command{}
			cmd.Flags().String("os", "", "")
			cmd.Flags().String("arch", "", "")
			cmd.Flags().String("output", "", "")
			cmd.Flags().String("prefix", "", "")
			cmd.Flags().String("zig-version", "", "")
			cmd.Flags().String("linkmode", "", "")
			cmd.Flags().StringSlice("include", nil, "")
			cmd.Flags().StringSlice("lib", nil, "")
			cmd.Flags().StringSlice("link", nil, "")
			cmd.Flags().StringSlice("pkg", nil, "")
			cmd.Flags().StringSlice("flags", nil, "")
			cmd.Flags().Bool("no-rpath", false, "")
			cmd.Flags().Bool("pack", false, "")
			cmd.Flags().Bool("strip", false, "")
			cmd.Flags().Bool("verbose", false, "")

			// Mark the flag as changed
			if err := cmd.Flags().Set(tt.flagName, "true"); err != nil {
				// Some flags need actual values
				switch tt.flagName {
				case "os":
					cmd.Flags().Set(tt.flagName, "linux")
				case "arch":
					cmd.Flags().Set(tt.flagName, "arm64")
				case "output":
					cmd.Flags().Set(tt.flagName, "/tmp/bin")
				case "prefix":
					cmd.Flags().Set(tt.flagName, "./dist")
				case "linkmode":
					cmd.Flags().Set(tt.flagName, "static")
				}
			}

			// Setup test flags
			oldFlags := flags
			defer func() { flags = oldFlags }()
			flags = buildFlags{}
			tt.setup(&flags)

			// Apply overrides
			opts := &build.Options{}
			applyFlagOverrides(cmd, opts)

			// Check result
			tt.check(t, opts)
		})
	}
}

func TestBuildCmd_Flags(t *testing.T) {
	// Verify buildCmd has expected flags
	expectedFlags := []string{
		"config", "target", "os", "arch", "output", "prefix",
		"zig-version", "linkmode", "include", "lib", "link",
		"pkg", "flags", "no-rpath", "pack", "strip", "verbose", "parallel",
	}

	for _, name := range expectedFlags {
		t.Run(name, func(t *testing.T) {
			if buildCmd.Flags().Lookup(name) == nil {
				t.Errorf("missing flag: %s", name)
			}
		})
	}
}

func TestBuildCmd_ShortFlags(t *testing.T) {
	shortFlags := map[string]string{
		"c": "config",
		"t": "target",
		"o": "output",
		"I": "include",
		"L": "lib",
		"l": "link",
		"s": "strip",
		"v": "verbose",
		"j": "parallel",
	}

	for short, long := range shortFlags {
		t.Run(short+"->"+long, func(t *testing.T) {
			flag := buildCmd.Flags().Lookup(long)
			if flag == nil {
				t.Fatalf("flag %s not found", long)
			}
			if flag.Shorthand != short {
				t.Errorf("flag %s shorthand = %q, want %q", long, flag.Shorthand, short)
			}
		})
	}
}
