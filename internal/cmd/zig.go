package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/qntx/gox/internal/zig"
	"github.com/spf13/cobra"
)

var zigCmd = &cobra.Command{
	Use:   "zig",
	Short: "Manage Zig compiler installations",
}

var zigUpdateCmd = &cobra.Command{
	Use:   "update [version]",
	Short: "Update or install a Zig version",
	Long: `Download and install a Zig compiler version.
If no version is specified, updates the 'master' version to latest.
Use --force to re-download even if already installed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runZigUpdate,
}

var zigListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed Zig versions",
	RunE:  runZigList,
}

var zigCleanCmd = &cobra.Command{
	Use:   "clean [version]",
	Short: "Remove cached Zig installations",
	Long: `Remove cached Zig compiler installations.
If no version is specified, removes all cached versions.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runZigClean,
}

func init() {
	zigUpdateCmd.Flags().BoolP("force", "f", false, "force re-download")
	zigCmd.AddCommand(zigUpdateCmd, zigListCmd, zigCleanCmd)
	rootCmd.AddCommand(zigCmd)
}

func runZigUpdate(cmd *cobra.Command, args []string) error {
	version := firstOr(args, "master")
	force, _ := cmd.Flags().GetBool("force")

	if force {
		if err := zig.Remove(version); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing: %w", err)
		}
	}

	path, err := zig.Ensure(cmd.Context(), version)
	if err != nil {
		return err
	}

	fmt.Printf("zig %s ready: %s\n", version, path)
	return nil
}

func runZigList(_ *cobra.Command, _ []string) error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Println("No Zig versions installed.")
		return nil
	}

	slices.Sort(versions)
	fmt.Println("Installed Zig versions:")
	for _, v := range versions {
		fmt.Printf("  %s\t%s\n", v, zig.Path(v))
	}
	return nil
}

func runZigClean(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cleanVersion(args[0])
	}
	return cleanAll()
}

func cleanVersion(version string) error {
	err := zig.Remove(version)
	if os.IsNotExist(err) {
		fmt.Printf("zig %s not installed\n", version)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("removed zig %s\n", version)
	return nil
}

func cleanAll() error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Println("No Zig versions to clean.")
		return nil
	}

	cacheDir := filepath.Dir(zig.Path(versions[0]))
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	fmt.Printf("removed %d version(s)\n", len(versions))
	return nil
}

func firstOr(args []string, def string) string {
	if len(args) > 0 {
		return args[0]
	}
	return def
}
