package ui

import (
	"io"
	"os"
	"path/filepath"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// Progress manages concurrent progress bars.
type Progress struct {
	p *mpb.Progress
}

// NewProgress creates a new progress container.
func NewProgress() *Progress {
	return &Progress{
		p: mpb.New(
			mpb.WithOutput(os.Stderr),
			mpb.WithWidth(40),
			mpb.WithAutoRefresh(),
		),
	}
}

// AddBar adds a new progress bar for a download task.
// Returns an io.Writer that tracks bytes written.
func (p *Progress) AddBar(name string, total int64) *Bar {
	// Truncate name if too long
	displayName := filepath.Base(name)
	if len(displayName) > 40 {
		displayName = displayName[:37] + "..."
	}

	bar := p.p.New(total,
		// Use ASCII chars for consistent width across terminals
		mpb.BarStyle().Lbound("[").Filler("=").Tip(">").Padding("-").Rbound("]"),
		mpb.PrependDecorators(
			decor.Name(displayName, decor.WC{W: 40, C: decor.DindentRight}),
		),
		mpb.AppendDecorators(
			decor.CountersKibiByte("% .1f / % .1f"),
			decor.Percentage(decor.WC{W: 5}),
		),
	)

	return &Bar{bar: bar}
}

// Wait waits for all bars to complete.
func (p *Progress) Wait() {
	p.p.Wait()
}

// Bar wraps an mpb.Bar and implements io.Writer.
type Bar struct {
	bar *mpb.Bar
}

// Write implements io.Writer for tracking download progress.
func (b *Bar) Write(p []byte) (int, error) {
	n := len(p)
	b.bar.IncrBy(n)
	return n, nil
}

// SetTotal updates the total for dynamic sizing.
func (b *Bar) SetTotal(total int64) {
	b.bar.SetTotal(total, false)
}

// Complete marks the bar as complete.
func (b *Bar) Complete() {
	b.bar.SetTotal(-1, true)
}

// Abort aborts the bar (e.g., on error).
func (b *Bar) Abort(drop bool) {
	b.bar.Abort(drop)
}

// ProxyReader wraps an io.Reader to track progress.
func (b *Bar) ProxyReader(r io.Reader) io.Reader {
	return b.bar.ProxyReader(r)
}
