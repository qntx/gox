package prompt

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/qntx/gox/internal/build"
)

var targets = []struct {
	name   string
	goos   string
	goarch string
}{
	{"linux/amd64", "linux", "amd64"},
	{"linux/arm64", "linux", "arm64"},
	{"linux/386", "linux", "386"},
	{"linux/arm", "linux", "arm"},
	{"linux/riscv64", "linux", "riscv64"},
	{"darwin/amd64", "darwin", "amd64"},
	{"darwin/arm64", "darwin", "arm64"},
	{"windows/amd64", "windows", "amd64"},
	{"windows/386", "windows", "386"},
	{"windows/arm64", "windows", "arm64"},
	{"freebsd/amd64", "freebsd", "amd64"},
}

var linkModes = []struct {
	name  string
	value string
}{
	{"Auto (default)", "auto"},
	{"Static linking", "static"},
	{"Dynamic linking", "dynamic"},
}

func Run(opts *build.Options) (*build.Options, error) {
	var targetIdx int
	var linkIdx int
	var includeDirs, libDirs, libs string

	targetOptions := make([]huh.Option[int], len(targets))
	for i, t := range targets {
		targetOptions[i] = huh.NewOption(t.name, i)
	}

	linkOptions := make([]huh.Option[int], len(linkModes))
	for i, l := range linkModes {
		linkOptions[i] = huh.NewOption(l.name, i)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Target Platform").
				Description("Select the target OS/Architecture").
				Options(targetOptions...).
				Value(&targetIdx),

			huh.NewSelect[int]().
				Title("Link Mode").
				Description("How to link C libraries").
				Options(linkOptions...).
				Value(&linkIdx),
		),

		huh.NewGroup(
			huh.NewInput().
				Title("Include Directories").
				Description("C header paths (comma-separated)").
				Placeholder("/usr/include/openssl, /opt/local/include").
				Value(&includeDirs),

			huh.NewInput().
				Title("Library Directories").
				Description("Library search paths (comma-separated)").
				Placeholder("/usr/lib/x86_64-linux-gnu, /opt/local/lib").
				Value(&libDirs),

			huh.NewInput().
				Title("Libraries").
				Description("Libraries to link (comma-separated)").
				Placeholder("ssl,crypto").
				Value(&libs),
		),

		huh.NewGroup(
			huh.NewInput().
				Title("Output").
				Description("Output file path").
				Placeholder("./bin/app").
				Value(&opts.Output),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("form: %w", err)
	}

	opts.GOOS = targets[targetIdx].goos
	opts.GOARCH = targets[targetIdx].goarch
	opts.LinkMode = linkModes[linkIdx].value

	if includeDirs != "" {
		opts.IncludeDirs = splitTrim(includeDirs)
	}
	if libDirs != "" {
		opts.LibDirs = splitTrim(libDirs)
	}
	if libs != "" {
		opts.Libs = splitTrim(libs)
	}

	return opts, nil
}

func splitTrim(s string) []string {
	var result []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}
