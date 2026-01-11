package cli

import (
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/ui"
	"github.com/qntx/gox/internal/zig"
)

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

func runZigUpdate(cmd *cobra.Command, args []string) error {
	version := "master"
	if len(args) > 0 {
		version = args[0]
	}
	force, _ := cmd.Flags().GetBool("force")

	if force {
		_ = zig.Remove(version)
	}

	path, err := zig.Ensure(cmd.Context(), version)
	if err != nil {
		return err
	}

	ui.Label("zig", path)
	return nil
}

func runZigList(_ *cobra.Command, _ []string) error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		ui.Info("No zig versions installed")
		return nil
	}

	slices.Sort(versions)
	ui.Header("Installed Zig Versions")

	tbl := ui.NewTable("VERSION", "PATH")
	for _, v := range versions {
		tbl.AddRow(v, zig.Path(v))
	}
	tbl.Render()
	return nil
}

func runZigClean(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cleanOne(args[0])
	}
	return cleanAll()
}

func cleanOne(version string) error {
	err := zig.Remove(version)
	if os.IsNotExist(err) {
		ui.Warn("zig %s not installed", version)
		return nil
	}
	if err != nil {
		return err
	}
	ui.Success("Removed zig %s", version)
	return nil
}

func cleanAll() error {
	versions, err := zig.Installed()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		ui.Info("Nothing to clean")
		return nil
	}

	if err := zig.RemoveAll(); err != nil {
		return err
	}
	ui.Success("Removed %d version(s)", len(versions))
	return nil
}
