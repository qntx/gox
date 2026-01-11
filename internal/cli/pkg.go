package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
)

// ----------------------------------------------------------------------------
// Commands
// ----------------------------------------------------------------------------

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

// ----------------------------------------------------------------------------
// Handlers
// ----------------------------------------------------------------------------

func runPkgList(_ *cobra.Command, _ []string) error {
	pkgs, err := build.ListCached()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Println("no cached packages")
		return nil
	}

	slices.SortFunc(pkgs, func(a, b build.CachedPkg) int {
		return strings.Compare(a.Name, b.Name)
	})

	var total int64
	fmt.Println("cached packages:")
	for _, p := range pkgs {
		fmt.Printf("  %-50s %s\n", p.Name, fmtSize(p.Size))
		total += p.Size
	}
	fmt.Printf("\ntotal: %d packages, %s\n", len(pkgs), fmtSize(total))
	fmt.Printf("path:  %s\n", build.CacheDir())
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
			fmt.Printf("name:    %s\n", p.Name)
			fmt.Printf("path:    %s\n", p.Path)
			fmt.Printf("size:    %s\n", fmtSize(p.Size))
			fmt.Printf("include: %d files\n", p.Include)
			fmt.Printf("lib:     %d files\n", p.Lib)
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

	for _, src := range args {
		pkg, err := build.Parse(src)
		if err != nil {
			return err
		}
		if err := pkg.Ensure(ctx); err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		fmt.Printf("installed: %s\n", pkg.Dir)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

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
			fmt.Printf("removed: %s\n", p.Name)
			removed++
		}
	}

	if removed == 0 {
		fmt.Printf("no packages matching %q\n", pattern)
	}
	return nil
}

func cleanAllPkgs() error {
	pkgs, err := build.ListCached()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Println("nothing to clean")
		return nil
	}

	if err := build.RemoveAllCached(); err != nil {
		return err
	}
	fmt.Printf("removed %d package(s)\n", len(pkgs))
	return nil
}

func matchGlob(name, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}
	// Simple glob: prefix* or *suffix or prefix*suffix
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

func fmtSize(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
