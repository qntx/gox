package build

import (
	"runtime"
	"testing"
)

func TestLinkMode_Valid(t *testing.T) {
	tests := []struct {
		mode LinkMode
		want bool
	}{
		{LinkAuto, true},
		{LinkStatic, true},
		{LinkDynamic, true},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.Valid(); got != tt.want {
				t.Errorf("LinkMode(%q).Valid() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestLinkMode_IsStatic(t *testing.T) {
	tests := []struct {
		mode LinkMode
		want bool
	}{
		{LinkStatic, true},
		{LinkAuto, false},
		{LinkDynamic, false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.IsStatic(); got != tt.want {
				t.Errorf("LinkMode(%q).IsStatic() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestOptions_Normalize(t *testing.T) {
	t.Run("empty options", func(t *testing.T) {
		o := &Options{}
		o.Normalize()

		if o.GOOS != runtime.GOOS {
			t.Errorf("GOOS = %q, want %q", o.GOOS, runtime.GOOS)
		}
		if o.GOARCH != runtime.GOARCH {
			t.Errorf("GOARCH = %q, want %q", o.GOARCH, runtime.GOARCH)
		}
		if o.LinkMode != LinkAuto {
			t.Errorf("LinkMode = %q, want %q", o.LinkMode, LinkAuto)
		}
	})

	t.Run("preserves existing values", func(t *testing.T) {
		o := &Options{
			GOOS:     "linux",
			GOARCH:   "arm64",
			LinkMode: LinkStatic,
		}
		o.Normalize()

		if o.GOOS != "linux" {
			t.Errorf("GOOS = %q, want linux", o.GOOS)
		}
		if o.GOARCH != "arm64" {
			t.Errorf("GOARCH = %q, want arm64", o.GOARCH)
		}
		if o.LinkMode != LinkStatic {
			t.Errorf("LinkMode = %q, want static", o.LinkMode)
		}
	})

	t.Run("cleans prefix path", func(t *testing.T) {
		o := &Options{Prefix: "./dist/../output/"}
		o.Normalize()

		if o.Prefix != "output" {
			t.Errorf("Prefix = %q, want 'output'", o.Prefix)
		}
	})
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name:    "valid default",
			opts:    Options{LinkMode: LinkAuto},
			wantErr: false,
		},
		{
			name:    "invalid linkmode",
			opts:    Options{LinkMode: "invalid"},
			wantErr: true,
		},
		{
			name:    "output and prefix exclusive",
			opts:    Options{Output: "bin", Prefix: "dist", LinkMode: LinkAuto},
			wantErr: true,
		},
		{
			name:    "no-rpath requires prefix",
			opts:    Options{NoRpath: true, LinkMode: LinkAuto},
			wantErr: true,
		},
		{
			name:    "no-rpath with prefix ok",
			opts:    Options{NoRpath: true, Prefix: "dist", LinkMode: LinkAuto},
			wantErr: false,
		},
		{
			name:    "pack requires output or prefix",
			opts:    Options{Pack: true, LinkMode: LinkAuto},
			wantErr: true,
		},
		{
			name:    "pack with output ok",
			opts:    Options{Pack: true, Output: "bin", LinkMode: LinkAuto},
			wantErr: false,
		},
		{
			name:    "pack with prefix ok",
			opts:    Options{Pack: true, Prefix: "dist", LinkMode: LinkAuto},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptions_ZigTarget(t *testing.T) {
	tests := []struct {
		goos, goarch string
		linkMode     LinkMode
		want         string
	}{
		{"linux", "amd64", LinkAuto, "x86_64-linux-gnu"},
		{"linux", "arm64", LinkAuto, "aarch64-linux-gnu"},
		{"linux", "386", LinkAuto, "x86-linux-gnu"},
		{"linux", "arm", LinkAuto, "arm-linux-gnueabihf"},
		{"linux", "amd64", LinkStatic, "x86_64-linux-musl"},
		{"linux", "arm", LinkStatic, "arm-linux-musleabihf"},
		{"windows", "amd64", LinkAuto, "x86_64-windows-gnu"},
		{"windows", "arm64", LinkAuto, "aarch64-windows-gnu"},
		{"darwin", "amd64", LinkAuto, "x86_64-macos"},
		{"freebsd", "amd64", LinkAuto, "x86_64-freebsd"},
		{"netbsd", "arm64", LinkAuto, "aarch64-netbsd"},
		{"linux", "riscv64", LinkAuto, "riscv64-linux-gnu"},
		{"linux", "loong64", LinkAuto, "loongarch64-linux-gnu"},
		{"linux", "ppc64le", LinkAuto, "powerpc64le-linux-gnu"},
		{"linux", "s390x", LinkAuto, "s390x-linux-gnu"},
	}

	for _, tt := range tests {
		name := tt.goos + "/" + tt.goarch
		if tt.linkMode == LinkStatic {
			name += "/static"
		}
		t.Run(name, func(t *testing.T) {
			o := &Options{GOOS: tt.goos, GOARCH: tt.goarch, LinkMode: tt.linkMode}
			if got := o.ZigTarget(); got != tt.want {
				t.Errorf("ZigTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}
