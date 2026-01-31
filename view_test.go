package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		size, total int64
		want        string
	}{
		{0, 100, "    "},
		{100, 0, "    "},
		{0, 0, "    "},
		{100, 100, "100%"},
		{50, 100, "50%"},
		{10, 100, "10%"},
		{5, 100, " 5%"},
		{1, 100, " 1%"},
		{1, 1000, " <1%"},
		{99, 100, "99%"},
		{1, 3, "33%"},
	}
	for _, tt := range tests {
		got := formatPercent(tt.size, tt.total)
		if got != tt.want {
			t.Errorf("formatPercent(%d, %d) = %q, want %q", tt.size, tt.total, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1536, "2 KB"},
		{1048576, "1 MB"},
		{10485760, "10 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
		{1099511627776, "1.0 TB"},
		{2199023255552, "2.0 TB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatDate(t *testing.T) {
	got := formatDate(time.Time{})
	if got != "          " {
		t.Errorf("formatDate(zero) = %q, want 10 spaces", got)
	}

	d := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	got = formatDate(d)
	if got != "2025-01-15" {
		t.Errorf("formatDate(2025-01-15) = %q, want %q", got, "2025-01-15")
	}
}

func TestTruncateName(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  string
	}{
		{"short", 10, "short"},
		{"exact_len!", 10, "exact_len!"},
		{"this is too long", 10, "this is t…"},
		{"", 5, ""},
		{"日本語テスト", 4, "日本語…"},
		{"abc", 3, "abc"},
		{"abcd", 3, "ab…"},
	}
	for _, tt := range tests {
		got := truncateName(tt.name, tt.width)
		if got != tt.want {
			t.Errorf("truncateName(%q, %d) = %q, want %q", tt.name, tt.width, got, tt.want)
		}
	}
}

func TestMarqueeSlice(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		offset int
		want   string
	}{
		// Fits within width — return as-is
		{"short", 10, 0, "short"},
		// Longer than width — slice from offset
		{"abcdefghij", 5, 0, "abcde"},
		{"abcdefghij", 5, 3, "defgh"},
		{"abcdefghij", 5, 5, "fghij"},
		// Offset near end — clamp to len
		{"abcdefghij", 5, 7, "hij"},
		// Unicode
		{"日本語テストデータ", 4, 0, "日本語テ"},
		{"日本語テストデータ", 4, 3, "テストデ"},
	}
	for _, tt := range tests {
		got := marqueeSlice(tt.name, tt.width, tt.offset)
		if got != tt.want {
			t.Errorf("marqueeSlice(%q, %d, %d) = %q, want %q", tt.name, tt.width, tt.offset, got, tt.want)
		}
	}
}

func TestProportionBarPlain(t *testing.T) {
	tests := []struct {
		size, maxSize int64
		barWidth      int
		desc          string
		checkFn       func(string) bool
	}{
		{0, 100, 10, "zero size", func(s string) bool { return s == strings.Repeat(" ", 10) }},
		{100, 100, 10, "full bar", func(s string) bool { return s == strings.Repeat("▐", 10) }},
		{50, 100, 10, "half bar", func(s string) bool {
			return len(s) > 0 && strings.Contains(s, "▐") && strings.Contains(s, " ")
		}},
		{0, 0, 10, "maxSize=0", func(s string) bool { return s == strings.Repeat(" ", 10) }},
		{100, 100, 0, "barWidth=0", func(s string) bool { return s == "" }},
		{1, 1000000, 10, "tiny size gets min 1 filled", func(s string) bool {
			return strings.Count(s, "▐") == 1
		}},
	}
	for _, tt := range tests {
		got := proportionBarPlain(tt.size, tt.maxSize, tt.barWidth)
		if !tt.checkFn(got) {
			t.Errorf("proportionBarPlain(%d, %d, %d) [%s] = %q", tt.size, tt.maxSize, tt.barWidth, tt.desc, got)
		}
	}
}

func TestSizeStyleFor(t *testing.T) {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)
	tests := []struct {
		bytes int64
		want  string // style identity: check by rendered output difference
	}{
		{0, "default"},
		{5 * MB, "default"},
		{10 * MB, "10MB"},
		{50 * MB, "10MB"},
		{100 * MB, "100MB"},
		{500 * MB, "100MB"},
		{GB, "1GB"},
		{5 * GB, "1GB"},
		{10 * GB, "10GB"},
		{100 * GB, "10GB"},
	}

	// Map style to a label by comparing with known styles
	styleLabel := func(bytes int64) string {
		s := sizeStyleFor(bytes)
		switch {
		case s.GetForeground() == sizeStyle10GB.GetForeground() && s.GetBold():
			return "10GB"
		case s.GetForeground() == sizeStyle1GB.GetForeground():
			return "1GB"
		case s.GetForeground() == sizeStyle100MB.GetForeground():
			return "100MB"
		case s.GetForeground() == sizeStyle10MB.GetForeground():
			return "10MB"
		default:
			return "default"
		}
	}

	for _, tt := range tests {
		got := styleLabel(tt.bytes)
		if got != tt.want {
			t.Errorf("sizeStyleFor(%d) style = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestHeaderLabels(t *testing.T) {
	if sizeHeaderLabel(sortSizeDesc) != "SIZE▼" {
		t.Error("sizeHeaderLabel(sortSizeDesc) should include arrow")
	}
	if sizeHeaderLabel(sortNameAsc) != "SIZE" {
		t.Error("sizeHeaderLabel(sortNameAsc) should not include arrow")
	}
	if nameHeaderLabel(sortNameAsc) != "NAME▲" {
		t.Error("nameHeaderLabel(sortNameAsc) should include arrow")
	}
	if nameHeaderLabel(sortSizeDesc) != "NAME" {
		t.Error("nameHeaderLabel(sortSizeDesc) should not include arrow")
	}
	if createdHeaderLabel(sortCreatedDesc) != "CREATED▼" {
		t.Error("createdHeaderLabel(sortCreatedDesc) should include arrow")
	}
	if updatedHeaderLabel(sortUpdatedDesc) != "UPDATED▼" {
		t.Error("updatedHeaderLabel(sortUpdatedDesc) should include arrow")
	}
}
