package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorCyan    = lipgloss.Color("6")
	colorGreen   = lipgloss.Color("2")
	colorYellow  = lipgloss.Color("3")
	colorRed     = lipgloss.Color("1")
	colorGray    = lipgloss.Color("8")
	colorMagenta = lipgloss.Color("5")

	styleSuccess = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleError   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleInfo    = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleLabel   = lipgloss.NewStyle().Foreground(colorMagenta)
	styleDim     = lipgloss.NewStyle().Foreground(colorGray)
)

// Prefix functions return styled prefix strings for use with custom writers.
func SuccessPrefix() string { return styleSuccess.Render("✓") }
func ErrorPrefix() string   { return styleError.Render("✗") }
func WarnPrefix() string    { return styleWarn.Render("!") }
func InfoPrefix() string    { return styleInfo.Render("→") }

// Success prints a success message with ✓ prefix.
func Success(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", SuccessPrefix(), fmt.Sprintf(msg, args...))
}

// Error prints an error message with ✗ prefix.
func Error(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", ErrorPrefix(), fmt.Sprintf(msg, args...))
}

// Warn prints a warning message with ! prefix.
func Warn(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", WarnPrefix(), fmt.Sprintf(msg, args...))
}

// Info prints an info message with → prefix.
func Info(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", InfoPrefix(), fmt.Sprintf(msg, args...))
}

// Step prints a step message with • prefix.
func Step(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", styleDim.Render("•"), fmt.Sprintf(msg, args...))
}

// Label prints a labeled value.
func Label(label, value string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", styleLabel.Render(label+":"), value)
}

// Dim prints dimmed text.
func Dim(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s\n", styleDim.Render(fmt.Sprintf(msg, args...)))
}

// Tracker tracks concurrent download progress.
type Tracker struct {
	mu    sync.Mutex
	start time.Time
}

// NewTracker creates a new progress tracker.
func NewTracker() *Tracker {
	return &Tracker{start: time.Now()}
}

// Done marks a download as complete and prints status.
func (t *Tracker) Done(name string, size int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(os.Stderr, "  %s %s (%s)\n", styleSuccess.Render("✓"), name, FormatSize(size))
}

// Elapsed returns time since tracker started.
func (t *Tracker) Elapsed() time.Duration {
	return time.Since(t.start)
}

// NopWriter returns a writer that discards all data (for compatibility).
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
