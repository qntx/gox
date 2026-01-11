package cli

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/ui"
)

var (
	pkgCmd = &cobra.Command{
		Use:   "pkg",
		Short: "Manage cached dependency packages",
	}

	pkgListCmd = &cobra.Command{
		Use:   "list",
		Short: "List cached packages",
		RunE:  runPkgList,
	}

	pkgCleanCmd = &cobra.Command{
		Use:   "clean [name]",
		Short: "Remove cached packages",
		Long: `Remove cached dependency packages.
If no name is specified, removes all cached packages.
Supports glob patterns (e.g., cuda_* to match all cuda packages).`,
		Args: cobra.MaximumNArgs(1),
		RunE: runPkgClean,
	}

	pkgInfoCmd = &cobra.Command{
		Use:   "info <name>",
		Short: "Show package details",
		Args:  cobra.ExactArgs(1),
		RunE:  runPkgInfo,
	}

	pkgInstallCmd = &cobra.Command{
		Use:   "install <source>...",
		Short: "Download packages to cache",
		Long: `Download and cache dependency packages without building.
Useful for pre-warming cache in CI/CD pipelines.

Sources can be:
  - Direct URL: https://example.com/archive.tar.gz
  - GitHub release: owner/repo@version/asset.tar.gz`,
		Args: cobra.MinimumNArgs(1),
		RunE: runPkgInstall,
	}
)

func init() {
	pkgCmd.AddCommand(pkgListCmd, pkgCleanCmd, pkgInfoCmd, pkgInstallCmd)
	rootCmd.AddCommand(pkgCmd)
}

func runPkgList(_ *cobra.Command, _ []string) error {
	pkgs, err := build.ListCached()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		ui.Info("No cached packages")
		return nil
	}

	slices.SortFunc(pkgs, func(a, b build.CacheEntry) int {
		return strings.Compare(a.Name, b.Name)
	})

	ui.Header("Cached Packages")

	tbl := ui.NewTable("NAME", "SIZE", "INCLUDE", "LIB")
	var total int64
	for _, p := range pkgs {
		tbl.AddRow(p.Name, ui.FormatSize(p.Size), fmt.Sprintf("%d", p.IncludeCount), fmt.Sprintf("%d", p.LibCount))
		total += p.Size
	}
	tbl.Render()

	fmt.Fprintln(os.Stderr)
	ui.Label("total", fmt.Sprintf("%d packages, %s", len(pkgs), ui.FormatSize(total)))
	ui.Label("path", build.CacheDir())
	return nil
}

func runPkgClean(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cleanPkg(args[0])
	}
	return cleanAllPkgs()
}

func runPkgInfo(_ *cobra.Command, args []string) error {
	pkgs, err := build.ListCached()
	if err != nil {
		return err
	}

	name := args[0]
	for _, p := range pkgs {
		if p.Name == name || matchGlob(p.Name, name) {
			ui.Header("Package Info")
			ui.Label("name", p.Name)
			ui.Label("path", p.Path)
			ui.Label("size", ui.FormatSize(p.Size))
			ui.Label("include", fmt.Sprintf("%d files", p.IncludeCount))
			ui.Label("lib", fmt.Sprintf("%d files", p.LibCount))
			return nil
		}
	}
	return fmt.Errorf("package %q not found", name)
}

func runPkgInstall(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	_, err := build.EnsureAll(ctx, args)
	return err
}

func cleanPkg(pattern string) error {
	pkgs, err := build.ListCached()
	if err != nil {
		return err
	}

	var removed int
	for _, p := range pkgs {
		if p.Name == pattern || matchGlob(p.Name, pattern) {
			if err := build.RemoveCached(p.Name); err != nil {
				return err
			}
			ui.Success("Removed %s", p.Name)
			removed++
		}
	}

	if removed == 0 {
		ui.Warn("No packages matching %q", pattern)
	}
	return nil
}

func cleanAllPkgs() error {
	pkgs, err := build.ListCached()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		ui.Info("Nothing to clean")
		return nil
	}

	if err := build.RemoveAllCached(); err != nil {
		return err
	}
	ui.Success("Removed %d package(s)", len(pkgs))
	return nil
}

func matchGlob(name, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}
	if i := strings.Index(pattern, "*"); i > 0 {
		return strings.HasPrefix(name, pattern[:i]) && strings.HasSuffix(name, pattern[i+1:])
	}
	return false
}
