package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/qntx/gox/internal/build"
)

type target struct {
	name, goos, goarch string
}

var targets = []target{
	{"linux/amd64", "linux", "amd64"},
	{"linux/arm64", "linux", "arm64"},
	{"linux/386", "linux", "386"},
	{"linux/arm", "linux", "arm"},
	{"linux/riscv64", "linux", "riscv64"},
	{"linux/loong64", "linux", "loong64"},
	{"linux/ppc64le", "linux", "ppc64le"},
	{"linux/s390x", "linux", "s390x"},
	{"darwin/amd64", "darwin", "amd64"},
	{"darwin/arm64", "darwin", "arm64"},
	{"windows/amd64", "windows", "amd64"},
	{"windows/386", "windows", "386"},
	{"windows/arm64", "windows", "arm64"},
	{"freebsd/amd64", "freebsd", "amd64"},
	{"freebsd/386", "freebsd", "386"},
	{"netbsd/amd64", "netbsd", "amd64"},
	{"netbsd/arm64", "netbsd", "arm64"},
	{"netbsd/386", "netbsd", "386"},
	{"netbsd/arm", "netbsd", "arm"},
}

var linkModes = []struct {
	name  string
	value build.LinkMode
}{
	{"Auto (default)", build.LinkModeAuto},
	{"Static linking", build.LinkModeStatic},
	{"Dynamic linking", build.LinkModeDynamic},
}

func SelectTarget(opts *build.Options) (*build.Options, error) {
	var (
		targetIdx            int
		linkIdx              int
		includeDirs, libDirs string
		libs                 string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Target Platform").
				Description("Select the target OS/Architecture").
				Options(indexedOptions(targets, func(t target) string { return t.name })...).
				Value(&targetIdx),

			huh.NewSelect[int]().
				Title("Link Mode").
				Description("How to link C libraries").
				Options(indexedOptions(linkModes, func(l struct {
					name  string
					value build.LinkMode
				}) string {
					return l.name
				})...).
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
	opts.IncludeDirs = splitTrim(includeDirs)
	opts.LibDirs = splitTrim(libDirs)
	opts.Libs = splitTrim(libs)

	return opts, nil
}

func indexedOptions[T any](items []T, label func(T) string) []huh.Option[int] {
	opts := make([]huh.Option[int], len(items))
	for i, item := range items {
		opts[i] = huh.NewOption(label(item), i)
	}
	return opts
}

func splitTrim(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}
