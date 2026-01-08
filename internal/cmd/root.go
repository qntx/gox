package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gox",
	Short: "Cross-platform CGO build tool powered by Zig",
	Long:  `gox simplifies CGO cross-compilation by leveraging Zig's C/C++ toolchain.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
