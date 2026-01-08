package prompt

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"gox.qntx.fun/internal/build"
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
				Placeholder("-I/path/to/headers").
				Value(&includeDirs),

			huh.NewInput().
				Title("Library Directories").
				Description("Library search paths (comma-separated)").
				Placeholder("-L/path/to/libs").
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
	for _, part := range split(s, ",") {
		if p := trim(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}

func split(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
