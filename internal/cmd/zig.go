package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/qntx/gox/internal/zig"
	"github.com/spf13/cobra"
)

var zigCmd = &cobra.Command{
	Use:   "zig",
	Short: "Manage Zig compiler installations",
	Long:  `Download, update, and manage cached Zig compiler versions.`,
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

var forceUpdate bool

func init() {
	zigUpdateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "force re-download")
	zigCmd.AddCommand(zigUpdateCmd)
	zigCmd.AddCommand(zigListCmd)
	zigCmd.AddCommand(zigCleanCmd)
	rootCmd.AddCommand(zigCmd)
}

func runZigUpdate(cmd *cobra.Command, args []string) error {
	version := versionOrDefault(args)

	if forceUpdate {
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

func runZigList(cmd *cobra.Command, args []string) error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		fmt.Println("No Zig versions installed.")
		return nil
	}

	sort.Strings(versions)
	fmt.Println("Installed Zig versions:")
	for _, v := range versions {
		path := zig.Path(v)
		info, _ := os.Stat(path)
		if info != nil {
			fmt.Printf("  %s\t%s\n", v, path)
		}
	}
	return nil
}

func runZigClean(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cleanVersion(args[0])
	}
	return cleanAll()
}

func cleanVersion(version string) error {
	if err := zig.Remove(version); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("zig %s not installed\n", version)
			return nil
		}
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

func versionOrDefault(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "master"
}
