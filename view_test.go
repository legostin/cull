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

func TestRenderTabBar_CachesAlwaysVisible(t *testing.T) {
	m := newTestModel()
	m.trashRegistry = &TrashRegistry{}
	bar := m.renderTabBar(100)
	if !strings.Contains(bar, "CACHES") {
		t.Errorf("tab bar %q must contain CACHES", bar)
	}
	if strings.Contains(bar, "HISTORY") {
		t.Errorf("tab bar %q must not contain HISTORY when trash is empty", bar)
	}
}

func TestRenderTabBar_CachesCount(t *testing.T) {
	m := newTestModel()
	m.tabs[tabCaches].allEntries = []Entry{{Name: "npm cache", Path: "/a"}, {Name: "pip cache", Path: "/b"}}
	bar := m.renderTabBar(100)
	if !strings.Contains(bar, "CACHES (2)") {
		t.Errorf("tab bar %q must contain CACHES (2)", bar)
	}
}

func TestProjectsDisplayName(t *testing.T) {
	m := newTestModel()
	m.projectMeta = map[string]projectArtifact{
		"/p/app/node_modules": {ProjectName: "app", Kind: "node_modules"},
		"/p/api/dist":         {ProjectName: "api", Kind: "dist", Caution: true},
	}
	got := m.projectsDisplayName(Entry{Name: "app", Path: "/p/app/node_modules"})
	if got != "app · node_modules" {
		t.Errorf("got %q, want %q", got, "app · node_modules")
	}
	got = m.projectsDisplayName(Entry{Name: "api", Path: "/p/api/dist"})
	if got != "api · dist · may be needed" {
		t.Errorf("got %q, want %q", got, "api · dist · may be needed")
	}
}

func TestProjectsTabBarAndEmptyState(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabProjects
	m.projectsScanning = true
	out := m.View()
	if !strings.Contains(out, "PROJECTS") {
		t.Error("tab bar must show PROJECTS")
	}
	if !strings.Contains(out, "Scanning projects") {
		t.Error("scanning empty state missing")
	}
	m.projectsScanning = false
	m.projectsLoaded = true
	out = m.View()
	if !strings.Contains(out, "no projects found under") {
		t.Error("empty-result hint missing")
	}
}

func TestProjectsStatusBarReclaimable(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabProjects
	m.projectsLoaded = true
	pt := &m.tabs[tabProjects]
	pt.allEntries = []Entry{
		{Name: "a", Path: "/p/a", Size: 1 << 30, Sized: true},
		{Name: "b", Path: "/p/b", Size: 1 << 30, Sized: true},
	}
	m.applyFilterForTab(tabProjects)
	out := m.View()
	if !strings.Contains(out, "reclaimable: 2.0 GB") {
		t.Errorf("status bar must show total reclaimable, got:\n%s", out)
	}
}

func TestRenderMapContainsEntries(t *testing.T) {
	m := newTestModel()
	m.browseMap = true
	tab := &m.tabs[tabBrowse]
	tab.allEntries = []Entry{
		{Name: "node_modules", Path: "/p/nm", Size: 1 << 30, Sized: true, IsDir: true},
		{Name: "main.go", Path: "/p/m", Size: 500 << 20, Sized: true},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	out := m.View()
	if !strings.Contains(out, "node_modules") || !strings.Contains(out, "main.go") {
		t.Errorf("map must label rectangles, got:\n%s", out)
	}
	if !strings.Contains(out, "1.0 GB") {
		t.Error("map labels must include sizes")
	}
}

func TestRenderMapTooSmall(t *testing.T) {
	m := newTestModel()
	m.browseMap = true
	m.width, m.height = 30, 12
	m.tabs[tabBrowse].entries = []Entry{{Name: "a", Size: 5, Sized: true}}
	out := m.View()
	if !strings.Contains(out, "too small") {
		t.Error("small terminal must show a hint instead of a map")
	}
}

func TestRenderMapEmptyDir(t *testing.T) {
	m := newTestModel()
	m.browseMap = true
	m.tabs[tabBrowse].entries = []Entry{{Name: "..", IsParent: true}}
	out := m.View()
	if !strings.Contains(out, "empty directory") {
		t.Error("empty dir must show a hint")
	}
}

func TestHelpLineShowsMapToggle(t *testing.T) {
	m := newTestModel()
	m.tabs[tabBrowse].entries = []Entry{{Name: "a", Size: 5, Sized: true}}
	out := m.View()
	if !strings.Contains(out, "map") {
		t.Error("BROWSE help line must mention the m/map toggle")
	}
	m.browseMap = true
	out = m.View()
	if !strings.Contains(out, "list") {
		t.Error("map-mode help line must offer m/list")
	}
	m.activeTab = tabCaches
	m.browseMap = false
	out = m.View()
	if strings.Contains(out, "<m> map") {
		t.Error("non-BROWSE tabs must not advertise the map toggle")
	}
}

// viewLines counts the lines a rendered frame occupies.
func viewLines(out string) int {
	return strings.Count(out, "\n") + 1
}

func TestViewNeverExceedsTerminalHeight(t *testing.T) {
	base := func() model {
		m := newTestModel()
		tab := &m.tabs[tabBrowse]
		tab.allEntries = []Entry{
			{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
			{Name: "a", Path: "/t/a", Size: 100, Sized: true},
			{Name: "b", Path: "/t/b", Size: 50, Sized: true},
		}
		tab.entries = append([]Entry{}, tab.allEntries...)
		return m
	}

	cases := []struct {
		name string
		mut  func(m *model)
	}{
		{"plain", func(m *model) {}},
		{"error line", func(m *model) { m.errMsg = "permission denied" }},
		{"confirm", func(m *model) {
			m.mode = modeConfirm
			m.tab().selected["/t/a"] = true
		}},
		{"confirm with error", func(m *model) {
			m.mode = modeConfirm
			m.tab().selected["/t/a"] = true
			m.errMsg = "boom"
		}},
		{"largest scanning empty", func(m *model) {
			m.activeTab = tabLargest
			m.deepScanning = true
			m.tabs[tabLargest] = newTabState()
		}},
		{"projects scanning empty", func(m *model) {
			m.activeTab = tabProjects
			m.projectsScanning = true
		}},
		{"projects empty loaded with error", func(m *model) {
			m.activeTab = tabProjects
			m.projectsLoaded = true
			m.errMsg = "boom"
		}},
		{"map mode", func(m *model) { m.browseMap = true }},
		{"map mode with error", func(m *model) {
			m.browseMap = true
			m.errMsg = "boom"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base()
			tc.mut(&m)
			out := m.View()
			if got := viewLines(out); got > m.height {
				t.Errorf("frame is %d lines, terminal is %d — view overflows and breaks scroll", got, m.height)
			}
		})
	}
}
