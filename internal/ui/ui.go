package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Modern color palette
var (
	// Primary colors
	colorPrimary   = lipgloss.Color("#7C3AED") // violet
	colorSecondary = lipgloss.Color("#06B6D4") // cyan

	// Status colors
	colorSuccess = lipgloss.Color("#10B981") // emerald
	colorError   = lipgloss.Color("#EF4444") // red
	colorWarning = lipgloss.Color("#F59E0B") // amber
	colorInfo    = lipgloss.Color("#3B82F6") // blue

	// Neutral colors
	colorMuted   = lipgloss.Color("#6B7280") // gray-500
	colorSubtle  = lipgloss.Color("#9CA3AF") // gray-400
	colorSurface = lipgloss.Color("#374151") // gray-700
	colorText    = lipgloss.Color("#F9FAFB") // gray-50
)

// Styles
var (
	// Status styles
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	styleError   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	styleInfo    = lipgloss.NewStyle().Foreground(colorInfo).Bold(true)

	// Text styles
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDim     = lipgloss.NewStyle().Foreground(colorMuted)
	styleSubtle  = lipgloss.NewStyle().Foreground(colorSubtle)
	stylePrimary = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	// Label styles
	styleLabel = lipgloss.NewStyle().Foreground(colorSubtle).Width(12)
	styleValue = lipgloss.NewStyle().Foreground(colorText)

	// Header style
	styleHeader = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			MarginBottom(1)

	// Box style for sections
	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSurface).
			Padding(0, 1)
)

// Icons
const (
	iconSuccess  = "✓"
	iconError    = "✗"
	iconWarning  = "!"
	iconInfo     = "●"
	iconArrow    = "→"
	iconBullet   = "•"
	iconSpinner  = "◐"
	iconDownload = "↓"
	iconPackage  = "◫"
	iconBuild    = "⚙"
	iconClean    = "♻"
)

// Prefix functions return styled prefix strings.
func SuccessPrefix() string { return styleSuccess.Render(iconSuccess) }
func ErrorPrefix() string   { return styleError.Render(iconError) }
func WarnPrefix() string    { return styleWarn.Render(iconWarning) }
func InfoPrefix() string    { return styleInfo.Render(iconInfo) }

// Success prints a success message.
func Success(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", SuccessPrefix(), fmt.Sprintf(msg, args...))
}

// Error prints an error message.
func Error(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", ErrorPrefix(), fmt.Sprintf(msg, args...))
}

// Warn prints a warning message.
func Warn(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", WarnPrefix(), fmt.Sprintf(msg, args...))
}

// Info prints an info message.
func Info(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", InfoPrefix(), fmt.Sprintf(msg, args...))
}

// Step prints a step message with indentation.
func Step(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", styleDim.Render(iconBullet), fmt.Sprintf(msg, args...))
}

// Label prints a key-value pair with consistent formatting.
func Label(key, value string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n",
		styleLabel.Render(key),
		styleValue.Render(value))
}

// Dim prints dimmed text.
func Dim(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s\n", styleDim.Render(fmt.Sprintf(msg, args...)))
}

// Header prints a section header.
func Header(title string) {
	fmt.Fprintf(os.Stderr, "\n%s\n", styleHeader.Render(title))
}

// Divider prints a horizontal divider.
func Divider() {
	fmt.Fprintf(os.Stderr, "%s\n", styleDim.Render(strings.Repeat("─", 50)))
}

// Target prints a build target header.
func Target(idx, total int, goos, goarch string) {
	target := fmt.Sprintf("%s/%s", goos, goarch)
	if total > 1 {
		counter := styleDim.Render(fmt.Sprintf("[%d/%d]", idx+1, total))
		fmt.Fprintf(os.Stderr, "\n%s %s\n", counter, stylePrimary.Render(target))
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
	if output != "" {
		fmt.Fprintf(os.Stderr, "%s %s %s\n",
			SuccessPrefix(),
			output,
			styleDim.Render(fmt.Sprintf("(%s)", FormatDuration(duration))))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s %s\n",
			SuccessPrefix(),
			styleDim.Render("Built in"),
			FormatDuration(duration))
	}
}

// BuildFailed prints build failure message.
func BuildFailed() {
	fmt.Fprintf(os.Stderr, "%s %s\n", ErrorPrefix(), "Build failed")
}

// Downloading prints download start message.
func Downloading(name string, count int) {
	if count > 1 {
		fmt.Fprintf(os.Stderr, "%s %s %d %s\n",
			styleInfo.Render(iconDownload),
			styleDim.Render("Downloading"),
			count,
			styleDim.Render("packages..."))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s %s\n",
			styleInfo.Render(iconDownload),
			styleDim.Render("Downloading"),
			name)
	}
}

// Downloaded prints download completion message.
func Downloaded(name string, size int64) {
	fmt.Fprintf(os.Stderr, "  %s %s %s\n",
		styleSuccess.Render(iconSuccess),
		name,
		styleDim.Render(fmt.Sprintf("(%s)", FormatSize(size))))
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
	// Header
	var hdr strings.Builder
	for i, h := range t.headers {
		if i > 0 {
			hdr.WriteString("  ")
		}
		hdr.WriteString(fmt.Sprintf("%-*s", t.widths[i], h))
	}
	fmt.Fprintf(os.Stderr, "  %s\n", styleDim.Render(hdr.String()))

	// Separator
	var sep strings.Builder
	for i, w := range t.widths {
		if i > 0 {
			sep.WriteString("  ")
		}
		sep.WriteString(strings.Repeat("─", w))
	}
	fmt.Fprintf(os.Stderr, "  %s\n", styleDim.Render(sep.String()))

	// Rows
	for _, row := range t.rows {
		var line strings.Builder
		for i, col := range row {
			if i > 0 {
				line.WriteString("  ")
			}
			if i < len(t.widths) {
				line.WriteString(fmt.Sprintf("%-*s", t.widths[i], col))
			} else {
				line.WriteString(col)
			}
		}
		fmt.Fprintf(os.Stderr, "  %s\n", line.String())
	}
}

// Tracker tracks concurrent progress.
type Tracker struct {
	mu    sync.Mutex
	start time.Time
}

// NewTracker creates a new progress tracker.
func NewTracker() *Tracker {
	return &Tracker{start: time.Now()}
}

// Done marks a task as complete.
func (t *Tracker) Done(name string, size int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	Downloaded(name, size)
}

// Elapsed returns time since tracker started.
func (t *Tracker) Elapsed() time.Duration {
	return time.Since(t.start)
}

// NopWriter returns a writer that discards all data.
func NopWriter() io.Writer {
	return io.Discard
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
