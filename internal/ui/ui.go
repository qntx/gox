package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorPrimary = lipgloss.Color("#7C3AED")
	colorSuccess = lipgloss.Color("#10B981")
	colorError   = lipgloss.Color("#EF4444")
	colorWarning = lipgloss.Color("#F59E0B")
	colorInfo    = lipgloss.Color("#3B82F6")
	colorMuted   = lipgloss.Color("#6B7280")
	colorSubtle  = lipgloss.Color("#9CA3AF")
	colorText    = lipgloss.Color("#F9FAFB")
)

var (
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	styleError   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	styleInfo    = lipgloss.NewStyle().Foreground(colorInfo).Bold(true)
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDim     = lipgloss.NewStyle().Foreground(colorMuted)
	stylePrimary = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleLabel   = lipgloss.NewStyle().Foreground(colorSubtle).Width(12)
	styleValue   = lipgloss.NewStyle().Foreground(colorText)
	styleHeader  = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).MarginBottom(1)
)

const (
	iconSuccess = "✓"
	iconError   = "✗"
	iconWarning = "!"
	iconInfo    = "●"
	iconArrow   = "→"
	iconBuild   = "⚙"
)

// Success prints a success message.
func Success(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", styleSuccess.Render(iconSuccess), fmt.Sprintf(msg, args...))
}

// Error prints an error message.
func Error(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", styleError.Render(iconError), fmt.Sprintf(msg, args...))
}

// Warn prints a warning message.
func Warn(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", styleWarn.Render(iconWarning), fmt.Sprintf(msg, args...))
}

// Info prints an info message.
func Info(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", styleInfo.Render(iconInfo), fmt.Sprintf(msg, args...))
}

// Header prints a section header.
func Header(title string) {
	fmt.Fprintf(os.Stderr, "\n%s\n", styleHeader.Render(title))
}

// Label prints a key-value pair with consistent formatting.
func Label(key, value string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", styleLabel.Render(key), styleValue.Render(value))
}

// Divider prints a horizontal divider.
func Divider() {
	fmt.Fprintf(os.Stderr, "%s\n", styleDim.Render(strings.Repeat("─", 50)))
}

// Target prints a build target header.
func Target(idx, total int, goos, goarch string) {
	target := fmt.Sprintf("%s/%s", goos, goarch)
	if total > 1 {
		fmt.Fprintf(os.Stderr, "\n%s %s\n",
			styleDim.Render(fmt.Sprintf("[%d/%d]", idx+1, total)),
			stylePrimary.Render(target))
	} else {
		fmt.Fprintf(os.Stderr, "\n%s %s\n", styleInfo.Render(iconArrow), stylePrimary.Render(target))
	}
}

// Building prints build start message.
func Building(target string) {
	fmt.Fprintf(os.Stderr, "%s %s %s\n",
		styleInfo.Render(iconBuild),
		styleDim.Render("Building"),
		styleBold.Render(target))
}

// Built prints build completion message.
func Built(output string, duration time.Duration) {
	prefix := styleSuccess.Render(iconSuccess)
	if output != "" {
		fmt.Fprintf(os.Stderr, "%s %s %s\n", prefix, output,
			styleDim.Render(fmt.Sprintf("(%s)", FormatDuration(duration))))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s %s\n", prefix,
			styleDim.Render("Built in"), FormatDuration(duration))
	}
}

// BuildFailed prints build failure message.
func BuildFailed() {
	fmt.Fprintf(os.Stderr, "%s %s\n", styleError.Render(iconError), "Build failed")
}

// Table renders a simple table.
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

// NewTable creates a new table with headers.
func NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{headers: headers, widths: widths}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cols ...string) {
	for i, c := range cols {
		if i < len(t.widths) && len(c) > t.widths[i] {
			t.widths[i] = len(c)
		}
	}
	t.rows = append(t.rows, cols)
}

// Render prints the table.
func (t *Table) Render() {
	var sb strings.Builder

	for i, h := range t.headers {
		if i > 0 {
			sb.WriteString("  ")
		}
		fmt.Fprintf(&sb, "%-*s", t.widths[i], h)
	}
	fmt.Fprintf(os.Stderr, "  %s\n", styleDim.Render(sb.String()))

	sb.Reset()
	for i, w := range t.widths {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(strings.Repeat("─", w))
	}
	fmt.Fprintf(os.Stderr, "  %s\n", styleDim.Render(sb.String()))

	for _, row := range t.rows {
		sb.Reset()
		for i, col := range row {
			if i > 0 {
				sb.WriteString("  ")
			}
			if i < len(t.widths) {
				fmt.Fprintf(&sb, "%-*s", t.widths[i], col)
			} else {
				sb.WriteString(col)
			}
		}
		fmt.Fprintf(os.Stderr, "  %s\n", sb.String())
	}
}

// FormatSize formats bytes as human readable string.
func FormatSize(b int64) string {
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

// FormatDuration formats duration as human readable string.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
