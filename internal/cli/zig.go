package cli

import (
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/zig"
)

// ----------------------------------------------------------------------------
// Commands
// ----------------------------------------------------------------------------

var (
	zigCmd = &cobra.Command{
		Use:   "zig",
		Short: "Manage Zig compiler installations",
	}

	zigUpdateCmd = &cobra.Command{
		Use:   "update [version]",
		Short: "Update or install a Zig version",
		Long: `Download and install a Zig compiler version.
If no version is specified, updates the 'master' version to latest.
Use --force to re-download even if already installed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runZigUpdate,
	}

	zigListCmd = &cobra.Command{
		Use:   "list",
		Short: "List installed Zig versions",
		RunE:  runZigList,
	}

	zigCleanCmd = &cobra.Command{
		Use:   "clean [version]",
		Short: "Remove cached Zig installations",
		Long: `Remove cached Zig compiler installations.
If no version is specified, removes all cached versions.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runZigClean,
	}
)

func init() {
	zigUpdateCmd.Flags().BoolP("force", "f", false, "force re-download")

	zigCmd.AddCommand(zigUpdateCmd, zigListCmd, zigCleanCmd)
	rootCmd.AddCommand(zigCmd)
}

// ----------------------------------------------------------------------------
// Handlers
// ----------------------------------------------------------------------------

func runZigUpdate(cmd *cobra.Command, args []string) error {
	version := firstOr(args, "master")
	force, _ := cmd.Flags().GetBool("force")

	if force {
		if err := zig.Remove(version); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove: %w", err)
		}
	}

	path, err := zig.Ensure(cmd.Context(), version)
	if err != nil {
		return err
	}

	fmt.Printf("zig %s: %s\n", version, path)
	return nil
}

func runZigList(_ *cobra.Command, _ []string) error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Println("no zig versions installed")
		return nil
	}

	slices.Sort(versions)
	fmt.Println("installed:")
	for _, v := range versions {
		fmt.Printf("  %s\t%s\n", v, zig.Path(v))
	}
	return nil
}

func runZigClean(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cleanOne(args[0])
	}
	return cleanAll()
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func cleanOne(version string) error {
	err := zig.Remove(version)
	if os.IsNotExist(err) {
		fmt.Printf("zig %s: not installed\n", version)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("removed: %s\n", version)
	return nil
}

func cleanAll() error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Println("nothing to clean")
		return nil
	}

	if err := zig.RemoveAll(); err != nil {
		return err
	}
	fmt.Printf("removed %d version(s)\n", len(versions))
	return nil
}

func firstOr(s []string, def string) string {
	if len(s) > 0 {
		return s[0]
	}
	return def
}
