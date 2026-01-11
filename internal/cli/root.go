package cli

import (
	"github.com/spf13/cobra"
)

// rootCmd is the base command for gox CLI.
var rootCmd = &cobra.Command{
	Use:   "gox",
	Short: "Cross-platform CGO build tool powered by Zig",
	Long: `gox simplifies CGO cross-compilation by leveraging Zig's C/C++ toolchain.

Update gox:  go install github.com/qntx/gox/cmd/gox@latest
Update zig:  gox zig update`,
}

// Execute runs the root command.
func Execute() error { return rootCmd.Execute() }
