package ui

import (
	"testing"
	"time"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatSize(tt.bytes); got != tt.want {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{0, "0ms"},
		{500 * time.Millisecond, "500ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
		{60 * time.Second, "60.0s"},
		{90 * time.Second, "90.0s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatDuration(tt.duration); got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestTable(t *testing.T) {
	t.Run("basic table", func(t *testing.T) {
		tbl := NewTable("NAME", "SIZE", "COUNT")

		if len(tbl.headers) != 3 {
			t.Errorf("len(headers) = %d, want 3", len(tbl.headers))
		}
		if len(tbl.widths) != 3 {
			t.Errorf("len(widths) = %d, want 3", len(tbl.widths))
		}

		// Initial widths should match header lengths
		if tbl.widths[0] != 4 { // "NAME"
			t.Errorf("widths[0] = %d, want 4", tbl.widths[0])
		}
	})

	t.Run("add row updates widths", func(t *testing.T) {
		tbl := NewTable("A", "B")
		tbl.AddRow("longer-value", "x")

		if tbl.widths[0] != 12 { // "longer-value"
			t.Errorf("widths[0] = %d, want 12", tbl.widths[0])
		}
		if tbl.widths[1] != 1 { // "B" is longer than "x"
			t.Errorf("widths[1] = %d, want 1", tbl.widths[1])
		}
	})

	t.Run("multiple rows", func(t *testing.T) {
		tbl := NewTable("COL1", "COL2")
		tbl.AddRow("a", "b")
		tbl.AddRow("aa", "bb")
		tbl.AddRow("aaa", "bbb")

		if len(tbl.rows) != 3 {
			t.Errorf("len(rows) = %d, want 3", len(tbl.rows))
		}
		if tbl.widths[0] != 4 { // "COL1" is still longest
			t.Errorf("widths[0] = %d, want 4", tbl.widths[0])
		}
	})

	t.Run("row longer than header", func(t *testing.T) {
		tbl := NewTable("X")
		tbl.AddRow("very-long-value")

		if tbl.widths[0] != 15 {
			t.Errorf("widths[0] = %d, want 15", tbl.widths[0])
		}
	})
}

func TestColorConstants(t *testing.T) {
	// Verify color constants are defined
	colors := []struct {
		name  string
		color interface{}
	}{
		{"colorPrimary", colorPrimary},
		{"colorSuccess", colorSuccess},
		{"colorError", colorError},
		{"colorWarning", colorWarning},
		{"colorInfo", colorInfo},
		{"colorMuted", colorMuted},
		{"colorSubtle", colorSubtle},
		{"colorText", colorText},
	}

	for _, c := range colors {
		t.Run(c.name, func(t *testing.T) {
			if c.color == nil {
				t.Errorf("%s is nil", c.name)
			}
		})
	}
}

func TestIconConstants(t *testing.T) {
	icons := []struct {
		name string
		icon string
	}{
		{"iconSuccess", iconSuccess},
		{"iconError", iconError},
		{"iconWarning", iconWarning},
		{"iconInfo", iconInfo},
		{"iconArrow", iconArrow},
		{"iconBuild", iconBuild},
	}

	for _, i := range icons {
		t.Run(i.name, func(t *testing.T) {
			if i.icon == "" {
				t.Errorf("%s is empty", i.name)
			}
		})
	}
}
