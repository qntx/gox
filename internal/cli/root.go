package cli

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// Brand colors
	brandPrimary = lipgloss.Color("#7C3AED")
	brandMuted   = lipgloss.Color("#6B7280")

	styleBrand = lipgloss.NewStyle().Foreground(brandPrimary).Bold(true)
	styleMuted = lipgloss.NewStyle().Foreground(brandMuted)
)

// rootCmd is the base command for gox CLI.
var rootCmd = &cobra.Command{
	Use:   "gox",
	Short: "Cross-platform CGO build tool powered by Zig",
	Long: styleBrand.Render("gox") + styleMuted.Render(" - Cross-platform CGO build tool") + `

Simplifies CGO cross-compilation by leveraging Zig's C/C++ toolchain.
Build for any OS/arch from any host without complex toolchain setup.

` + styleMuted.Render("Quick Start:") + `
  gox build                    Build for current platform
  gox build -t linux/amd64     Build for Linux x64
  gox run .                    Compile and run current package
  gox test ./...               Run tests with CGO support
  gox install .                Install to $GOPATH/bin
  gox zig update               Install/update Zig compiler

` + styleMuted.Render("More Info:") + `
  gox build --help             Show build options
  gox run --help               Show run options
  gox test --help              Show test options
  gox install --help           Show install options
  gox pkg list                 List cached packages`,
}

// Execute runs the root command.
func Execute() error {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetOut(os.Stderr)
	return rootCmd.Execute()
}
