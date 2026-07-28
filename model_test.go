package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() model {
	m := newModel("/tmp/test", 100, false, false)
	m.width = 120
	m.height = 40
	return m
}

func TestNameColWidth(t *testing.T) {
	tests := []struct {
		width   int
		wantMin int
		wantMax int
		desc    string
	}{
		{40, 10, 10, "very narrow terminal — clamped to min 10"},
		{80, 10, 40, "normal terminal"},
		{200, 40, 40, "wide terminal — clamped to max 40"},
	}
	for _, tt := range tests {
		m := newTestModel()
		m.width = tt.width
		got := m.nameColWidth()
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("nameColWidth() with width=%d [%s]: got %d, want [%d, %d]",
				tt.width, tt.desc, got, tt.wantMin, tt.wantMax)
		}
	}
}

func TestNameScrollOffset(t *testing.T) {
	m := newTestModel()

	// Before pause
	m.nameScroll = 0
	off := m.nameScrollOffset(50, 20)
	if off != 0 {
		t.Errorf("before pause: offset = %d, want 0", off)
	}

	// During pause (tick 5, still within startPause=7)
	m.nameScroll = 5
	off = m.nameScrollOffset(50, 20)
	if off != 0 {
		t.Errorf("during pause: offset = %d, want 0", off)
	}

	// After pause starts scrolling
	m.nameScroll = 10
	off = m.nameScrollOffset(50, 20)
	if off != 3 { // 10 - 7 = 3
		t.Errorf("scrolling: offset = %d, want 3", off)
	}

	// At max offset
	m.nameScroll = 40 // 7 + 30 = 37, but 40-7=33 > maxOffset=30, so clamped
	off = m.nameScrollOffset(50, 20)
	if off != 30 {
		t.Errorf("at max: offset = %d, want 30", off)
	}

	// No scrolling needed (name fits)
	m.nameScroll = 10
	off = m.nameScrollOffset(15, 20)
	if off != 0 {
		t.Errorf("fits: offset = %d, want 0", off)
	}
}

func TestEntryDisplayName(t *testing.T) {
	m := newTestModel()
	m.path = "/tmp/test"

	// File entry on browse tab
	m.activeTab = tabBrowse
	e := Entry{Name: "file.txt", Path: "/tmp/test/file.txt"}
	got := m.entryDisplayName(e)
	if got != "file.txt" {
		t.Errorf("browse file: got %q, want %q", got, "file.txt")
	}

	// Dir entry on browse tab
	e = Entry{Name: "subdir", Path: "/tmp/test/subdir", IsDir: true}
	got = m.entryDisplayName(e)
	if got != "subdir/" {
		t.Errorf("browse dir: got %q, want %q", got, "subdir/")
	}

	// Largest tab shows relative path
	m.activeTab = tabLargest
	e = Entry{Name: "deep.txt", Path: "/tmp/test/sub/deep.txt"}
	got = m.entryDisplayName(e)
	if got != "sub/deep.txt" {
		t.Errorf("largest tab: got %q, want %q", got, "sub/deep.txt")
	}
}

func TestApplyFilter(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "readme.md", Path: "/tmp/test/readme.md"},
		{Name: "main.go", Path: "/tmp/test/main.go"},
		{Name: ".hidden", Path: "/tmp/test/.hidden"},
	}

	// No filter
	m.applyFilter()
	if len(tab.entries) != 4 {
		t.Errorf("no filter: got %d entries, want 4", len(tab.entries))
	}

	// Text filter
	tab.filterText = "main"
	m.applyFilter()
	if len(tab.entries) != 2 { // parent + main.go
		t.Errorf("filter 'main': got %d entries, want 2", len(tab.entries))
	}
	if !tab.entries[0].IsParent {
		t.Error("parent should always be included")
	}

	// Hidden files filter
	tab.filterText = ""
	m.showHidden = false
	m.applyFilter()
	if len(tab.entries) != 3 { // parent + readme + main (not .hidden)
		t.Errorf("hidden off: got %d entries, want 3", len(tab.entries))
	}
}

func TestApplyFilterForTab(t *testing.T) {
	m := newTestModel()
	lt := &m.tabs[tabLargest]
	lt.allEntries = []Entry{
		{Name: "big.zip", Path: "/big.zip"},
		{Name: "small.txt", Path: "/small.txt"},
	}
	lt.filterText = "big"
	m.applyFilterForTab(tabLargest)
	if len(lt.entries) != 1 {
		t.Errorf("filter on largest tab: got %d entries, want 1", len(lt.entries))
	}
}

func TestClampOffset(t *testing.T) {
	m := newTestModel()
	m.height = 20 // visibleRows = 20 - 11 = 9
	tab := m.tab()
	tab.entries = make([]Entry, 30)

	// Cursor below viewport
	tab.cursor = 15
	tab.offset = 0
	m.clampOffset()
	if tab.offset > tab.cursor || tab.cursor >= tab.offset+9 {
		t.Errorf("cursor below: offset=%d, cursor=%d", tab.offset, tab.cursor)
	}

	// Cursor above viewport
	tab.cursor = 2
	tab.offset = 10
	m.clampOffset()
	if tab.offset != 2 {
		t.Errorf("cursor above: offset=%d, want 2", tab.offset)
	}
}

func TestResetNameScroll(t *testing.T) {
	m := newTestModel()
	m.width = 120
	tab := m.tab()
	tab.entries = []Entry{
		{Name: "short.txt", Path: "/short.txt"},
	}
	tab.cursor = 0
	gen := m.nameScrollGen

	cmd := m.resetNameScroll()
	if m.nameScrollGen != gen+1 {
		t.Error("resetNameScroll should increment generation")
	}
	if m.nameScroll != 0 {
		t.Error("resetNameScroll should reset scroll to 0")
	}
	// Short name — no tick command needed
	if cmd != nil {
		t.Error("short name should not start tick")
	}

	// Long name — should return a command
	tab.entries = []Entry{
		{Name: "this_is_a_very_long_filename_that_exceeds_the_column_width.txt", Path: "/long.txt"},
	}
	cmd = m.resetNameScroll()
	if cmd == nil {
		t.Error("long name should start tick")
	}
}

func TestHandleNameScrollTick(t *testing.T) {
	m := newTestModel()
	m.width = 120
	tab := m.tab()
	tab.entries = []Entry{
		{Name: "this_is_a_very_long_filename_that_exceeds_the_column_width.txt",
			Path: "/long.txt"},
	}
	tab.cursor = 0
	m.nameScrollPath = "/long.txt"
	m.nameScrollGen = 5

	// Stale generation
	result, cmd := m.handleNameScrollTick(nameScrollTickMsg{gen: 3})
	if cmd != nil {
		t.Error("stale generation should return nil cmd")
	}
	_ = result

	// Matching generation
	m2 := m
	result2, cmd2 := m2.handleNameScrollTick(nameScrollTickMsg{gen: 5})
	if cmd2 == nil {
		t.Error("matching generation should return a cmd")
	}
	resultModel := result2.(model)
	if resultModel.nameScroll != 1 {
		t.Errorf("nameScroll = %d, want 1", resultModel.nameScroll)
	}
}

func TestCachedDirSize(t *testing.T) {
	m := newTestModel()

	// Cache miss
	_, ok := m.cachedDirSize("/nonexistent")
	if ok {
		t.Error("cache miss should return false")
	}

	// Cache hit
	m.cache["/some/dir"] = dirCacheEntry{
		browseEntries: []Entry{
			{IsParent: true, Size: 0},
			{Size: 100, Name: "a"},
			{Size: 200, Name: "b"},
		},
		deepScanDone: true,
	}
	size, ok := m.cachedDirSize("/some/dir")
	if !ok {
		t.Error("cache hit should return true")
	}
	if size != 300 {
		t.Errorf("cached size = %d, want 300", size)
	}

	// Deep scan not done
	m.cache["/partial"] = dirCacheEntry{
		browseEntries: []Entry{{Size: 100}},
		deepScanDone:  false,
	}
	_, ok = m.cachedDirSize("/partial")
	if ok {
		t.Error("incomplete deep scan should return false")
	}
}

func TestRemoveDeleted(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "a.txt", Path: "/tmp/test/a.txt"},
		{Name: "b.txt", Path: "/tmp/test/b.txt"},
		{Name: "c.txt", Path: "/tmp/test/c.txt"},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.selected["/tmp/test/b.txt"] = true
	tab.cursor = 3

	m.removeDeleted([]string{"/tmp/test/b.txt", "/tmp/test/c.txt"})

	if len(tab.allEntries) != 2 {
		t.Errorf("allEntries len = %d, want 2", len(tab.allEntries))
	}
	if len(tab.entries) != 2 {
		t.Errorf("entries len = %d, want 2", len(tab.entries))
	}
	if _, ok := tab.selected["/tmp/test/b.txt"]; ok {
		t.Error("deleted entry should be removed from selection")
	}
	// Cursor should be clamped
	if tab.cursor >= len(tab.entries) {
		t.Errorf("cursor = %d, should be < %d", tab.cursor, len(tab.entries))
	}
}

func TestUpdateDirSizes(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "sub1", Path: "/tmp/test/sub1", IsDir: true},
		{Name: "sub2", Path: "/tmp/test/sub2", IsDir: true},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.cursor = 1 // pointing at sub1

	sizes := map[string]int64{
		"/tmp/test/sub1": 5000,
		"/tmp/test/sub2": 3000,
	}
	m.updateDirSizes(sizes)

	// Check sizes were updated
	found := false
	for _, e := range tab.allEntries {
		if e.Path == "/tmp/test/sub1" {
			if e.Size != 5000 || !e.Sized {
				t.Errorf("sub1: size=%d, sized=%v", e.Size, e.Sized)
			}
			found = true
		}
	}
	if !found {
		t.Error("sub1 not found in allEntries")
	}

	// Cursor should still point to sub1 after re-sort
	if tab.cursor < len(tab.entries) {
		if tab.entries[tab.cursor].Path != "/tmp/test/sub1" {
			t.Errorf("cursor moved to %q, want sub1", tab.entries[tab.cursor].Path)
		}
	}
}

func TestUpdateDirSizes_Empty(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "sub", Path: "/tmp/sub", IsDir: true, Size: 10},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)

	// Empty sizes map should be a no-op
	m.updateDirSizes(nil)
	if tab.allEntries[0].Size != 10 {
		t.Error("empty updateDirSizes should not modify entries")
	}
}

func TestNewModel(t *testing.T) {
	m := newModel("/tmp", 50, false, false)
	if m.path != "/tmp" {
		t.Errorf("path = %q, want /tmp", m.path)
	}
	if m.topN != 50 {
		t.Errorf("topN = %d, want 50", m.topN)
	}
	if m.sortBy != sortSizeDesc {
		t.Error("default sort should be sortSizeDesc")
	}
	if !m.showHidden {
		t.Error("showHidden should default to true")
	}
	if m.deleteType != deleteTrash {
		t.Error("deleteType should default to deleteTrash")
	}
}

func TestNewMultiRootModel(t *testing.T) {
	paths := []string{"/a", "/b"}
	m := newMultiRootModel(paths, 100, false, false)
	if !m.isVirtualRoot {
		t.Error("should be virtual root")
	}
	if len(m.rootPaths) != 2 {
		t.Errorf("rootPaths len = %d, want 2", len(m.rootPaths))
	}
	bt := &m.tabs[tabBrowse]
	if len(bt.allEntries) != 2 {
		t.Errorf("browse entries = %d, want 2", len(bt.allEntries))
	}
}

func TestCollectUnsizedDirs(t *testing.T) {
	m := newTestModel()
	bt := &m.tabs[tabBrowse]
	bt.allEntries = []Entry{
		{Name: "..", IsDir: true, IsParent: true},
		{Name: "sized", Path: "/sized", IsDir: true, Sized: true},
		{Name: "unsized", Path: "/unsized", IsDir: true, Sized: false},
		{Name: "file.txt", Path: "/file.txt", Sized: true},
	}
	dirs := m.collectUnsizedDirs()
	if len(dirs) != 1 || dirs[0] != "/unsized" {
		t.Errorf("collectUnsizedDirs = %v, want [/unsized]", dirs)
	}
}

func TestInternEntries(t *testing.T) {
	m := newTestModel()
	entries := []Entry{
		{Path: "/a/b/file1.txt"},
		{Path: "/a/b/file2.txt"},
		{Path: "/c/d/file3.txt"},
		{Path: ""},
	}
	m.internEntries(entries)

	// Same dir should have same ID
	if entries[0].DirID != entries[1].DirID {
		t.Error("same directory should have same DirID")
	}
	// Different dir should have different ID
	if entries[0].DirID == entries[2].DirID {
		t.Error("different directories should have different DirID")
	}
	// Empty path should not be interned
	if entries[3].DirID != 0 {
		t.Errorf("empty path DirID = %d, want 0", entries[3].DirID)
	}
}

func TestWindowSizeUpdate(t *testing.T) {
	m := newTestModel()
	m.width = 0
	m.height = 0

	msg := windowSizeMsg{Width: 120, Height: 40}
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.width != 120 || rm.height != 40 {
		t.Errorf("after WindowSizeMsg: width=%d height=%d", rm.width, rm.height)
	}
}

// windowSizeMsg wraps tea.WindowSizeMsg for testing
type windowSizeMsg = tea.WindowSizeMsg

func TestDeleteDoneMsg(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "a.txt", Path: "/tmp/test/a.txt"},
		{Name: "b.txt", Path: "/tmp/test/b.txt"},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.selected["/tmp/test/a.txt"] = true
	m.mode = modeConfirm

	result, _ := m.Update(deleteDoneMsg{deleted: []string{"/tmp/test/a.txt"}})
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Error("mode should be normal after delete done")
	}
	rt := rm.tab()
	if len(rt.allEntries) != 1 {
		t.Errorf("allEntries len = %d, want 1", len(rt.allEntries))
	}
}

func TestDeleteErrMsg(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "a.txt", Path: "/a.txt"},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	m.mode = modeConfirm

	result, _ := m.Update(deleteErrMsg{
		err:     &testError{"fail"},
		deleted: []string{"/a.txt"},
	})
	rm := result.(model)
	if rm.errMsg != "fail" {
		t.Errorf("errMsg = %q, want 'fail'", rm.errMsg)
	}
	if rm.mode != modeNormal {
		t.Error("mode should be normal after delete error")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestNameScrollTickMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.nameScrollGen = 5
	tab := m.tab()
	tab.entries = []Entry{{Name: "x", Path: "/x"}}

	_, cmd := m.Update(nameScrollTickMsg{gen: 3})
	if cmd != nil {
		t.Error("stale tick should produce nil cmd")
	}
}

// Test that Init returns nil for virtual root
func TestInit_VirtualRoot(t *testing.T) {
	m := newMultiRootModel([]string{"/a"}, 100, false, false)
	cmd := m.Init()
	if cmd != nil {
		t.Error("virtual root Init should return nil")
	}
}

// Test tab() pointer returns correct tab
func TestTabPointer(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabBrowse
	bt := m.tab()
	bt.cursor = 5
	if m.tabs[tabBrowse].cursor != 5 {
		t.Error("tab() should return a pointer to the active tab")
	}

	m.activeTab = tabLargest
	lt := m.tab()
	lt.cursor = 3
	if m.tabs[tabLargest].cursor != 3 {
		t.Error("tab() should return pointer to largest tab")
	}
}

func TestNewTabState(t *testing.T) {
	ts := newTabState()
	if ts.selected == nil {
		t.Error("selected map should be initialized")
	}
	if ts.lastSelect != -1 {
		t.Errorf("lastSelect = %d, want -1", ts.lastSelect)
	}
}

func TestScanErrMsg(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(scanErrMsg{err: &testError{"scan failed"}})
	rm := result.(model)
	if rm.errMsg != "scan failed" {
		t.Errorf("errMsg = %q, want 'scan failed'", rm.errMsg)
	}
}

func TestDeepScanMsg_StaleRoot(t *testing.T) {
	m := newTestModel()
	m.path = "/current"
	// Message from a different root should be discarded
	result, _ := m.Update(deepScanMsg{rootPath: "/old"})
	rm := result.(model)
	_ = rm
	// Should not crash and should be a no-op
}

func TestNameScrollPause(t *testing.T) {
	m := newTestModel()
	// Verify startPause constant behavior
	m.nameScroll = 6 // just before startPause (7)
	off := m.nameScrollOffset(50, 20)
	if off != 0 {
		t.Errorf("tick 6 should still be in pause, got offset %d", off)
	}

	m.nameScroll = 7 // exactly at startPause
	off = m.nameScrollOffset(50, 20)
	if off != 0 {
		t.Errorf("tick 7 should produce offset 0, got %d", off)
	}

	m.nameScroll = 8
	off = m.nameScrollOffset(50, 20)
	if off != 1 {
		t.Errorf("tick 8 should produce offset 1, got %d", off)
	}
}

func TestEntryDisplayName_EmptyPath(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabLargest
	// Entry with empty path should just return name
	e := Entry{Name: "nopath", IsDir: true}
	got := m.entryDisplayName(e)
	if got != "nopath/" {
		t.Errorf("empty path dir: got %q, want %q", got, "nopath/")
	}
}

func TestNavigateToVirtualRoot(t *testing.T) {
	paths := []string{"/a", "/b"}
	m := newMultiRootModel(paths, 100, false, false)
	m.isVirtualRoot = false
	m.path = "/a"
	m.tabs[tabBrowse].allEntries = []Entry{{Name: "file", Path: "/a/file"}}
	m.tabs[tabBrowse].entries = m.tabs[tabBrowse].allEntries

	result := m.navigateToVirtualRoot()
	if !result.isVirtualRoot {
		t.Error("should be virtual root")
	}
	if result.path != "/ (multiple roots)" {
		t.Errorf("path = %q, want '/ (multiple roots)'", result.path)
	}
	bt := &result.tabs[tabBrowse]
	if len(bt.entries) != 2 {
		t.Errorf("entries = %d, want 2", len(bt.entries))
	}
}

func TestRemoveDeleted_EmptyEntries(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "only.txt", Path: "/only.txt"},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.cursor = 0

	m.removeDeleted([]string{"/only.txt"})

	if len(tab.entries) != 0 {
		t.Errorf("entries should be empty, got %d", len(tab.entries))
	}
	if tab.cursor != 0 {
		t.Errorf("cursor should be 0 for empty list, got %d", tab.cursor)
	}
}

func TestClampOffset_ConfirmMode(t *testing.T) {
	m := newTestModel()
	m.height = 20
	m.mode = modeConfirm // reduces visible rows by 2
	tab := m.tab()
	tab.entries = make([]Entry, 30)
	tab.cursor = 10
	tab.offset = 0
	m.clampOffset()
	// visibleRows = 20 - 11 - 2 = 7
	if tab.cursor >= tab.offset+7 {
		t.Errorf("confirm mode: offset=%d, cursor=%d, should fit in 7 rows", tab.offset, tab.cursor)
	}
}

func TestApplyFilter_CombinedHiddenAndText(t *testing.T) {
	m := newTestModel()
	m.showHidden = false
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: ".hidden_match", Path: "/tmp/.hidden_match"},
		{Name: "visible_match", Path: "/tmp/visible_match"},
		{Name: "visible_other", Path: "/tmp/visible_other"},
	}
	tab.filterText = "match"
	m.applyFilter()
	// Should get parent + visible_match (not .hidden_match due to showHidden=false)
	if len(tab.entries) != 2 {
		t.Errorf("combined filter: got %d entries, want 2", len(tab.entries))
	}
}

func TestView_ZeroWidth(t *testing.T) {
	m := newTestModel()
	m.width = 0
	got := m.View()
	if got != "" {
		t.Error("View with zero width should return empty string")
	}
}

func TestDeepScanMsg_Done(t *testing.T) {
	m := newTestModel()
	m.path = "/test"
	m.deepScanning = true
	m.deepScanCh = make(chan deepScanMsg, 1)

	entries := []Entry{{Name: "big.txt", Path: "/test/big.txt", Size: 1000, Sized: true}}
	result, _ := m.Update(deepScanMsg{
		rootPath: "/test",
		entries:  entries,
		dirSizes: map[string]int64{},
		done:     true,
	})
	rm := result.(model)
	if rm.deepScanning {
		t.Error("deepScanning should be false after done")
	}
	if !rm.deepScanDone {
		t.Error("deepScanDone should be true after done")
	}
	lt := &rm.tabs[tabLargest]
	if len(lt.allEntries) != 1 {
		t.Errorf("largest tab should have 1 entry, got %d", len(lt.allEntries))
	}
}

// quickScanDoneMsg integration test
func TestQuickScanDoneMsg(t *testing.T) {
	m := newTestModel()
	entries := []Entry{
		{Name: "..", Path: "/", IsDir: true, IsParent: true},
		{Name: "file.txt", Path: "/tmp/test/file.txt", Size: 100, Sized: true,
			ModTime: time.Now()},
	}
	result, _ := m.Update(quickScanDoneMsg{entries: entries, path: "/tmp/test"})
	rm := result.(model)
	bt := &rm.tabs[tabBrowse]
	if len(bt.allEntries) != 2 {
		t.Errorf("browse entries = %d, want 2", len(bt.allEntries))
	}
	if rm.path != "/tmp/test" {
		t.Errorf("path = %q, want /tmp/test", rm.path)
	}
}

func TestTabCachesID(t *testing.T) {
	if tabBrowse != 0 || tabLargest != 1 || tabCaches != 2 || tabHistory != 3 {
		t.Errorf("tab IDs = %d %d %d %d, want 0 1 2 3",
			tabBrowse, tabLargest, tabCaches, tabHistory)
	}
	m := newTestModel()
	if len(m.tabs) != 4 {
		t.Errorf("len(tabs) = %d, want 4", len(m.tabs))
	}
}

func TestCachesLoadedMsg(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabCaches
	msg := cachesLoadedMsg{
		entries: []Entry{
			{Name: "npm cache", Path: "/home/u/.npm/_cacache", IsDir: true},
			{Name: "Docker (system prune -a)", Path: dockerEntryPath},
		},
		pathGroups: map[string][]string{
			"/home/u/.npm/_cacache": {"/home/u/.npm/_cacache"},
		},
	}
	updated, cmd := m.Update(msg)
	m2 := updated.(model)
	ct := m2.tabs[tabCaches]
	if len(ct.allEntries) != 2 || len(ct.entries) != 2 {
		t.Fatalf("entries = %d/%d, want 2/2", len(ct.allEntries), len(ct.entries))
	}
	if m2.cachePathGroups == nil || len(m2.cachePathGroups["/home/u/.npm/_cacache"]) != 1 {
		t.Errorf("cachePathGroups not stored: %+v", m2.cachePathGroups)
	}
	if cmd == nil {
		t.Error("expected batch of size commands, got nil")
	}
}

func TestCacheSizeMsg(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabCaches
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{
		{Name: "small", Path: "/c/small", IsDir: true},
		{Name: "big", Path: "/c/big", IsDir: true},
	}
	ct.entries = ct.allEntries

	updated, _ := m.Update(cacheSizeMsg{path: "/c/big", size: 999, ok: true})
	m2 := updated.(model)
	ct2 := m2.tabs[tabCaches]
	if ct2.entries[0].Path != "/c/big" || ct2.entries[0].Size != 999 || !ct2.entries[0].Sized {
		t.Errorf("after size msg, first entry = %+v, want /c/big sized 999 first (size-desc)", ct2.entries[0])
	}
}

func TestCacheSizeMsg_DropRow(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabCaches
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{
		{Name: "npm cache", Path: "/c/npm", IsDir: true},
		{Name: "Docker (system prune -a)", Path: dockerEntryPath},
	}
	ct.entries = ct.allEntries

	updated, _ := m.Update(cacheSizeMsg{path: dockerEntryPath, ok: false})
	m2 := updated.(model)
	ct2 := m2.tabs[tabCaches]
	if len(ct2.allEntries) != 1 || ct2.allEntries[0].Path != "/c/npm" {
		t.Errorf("docker row not dropped: %+v", ct2.allEntries)
	}
}

func TestDockerPruneDoneMsg(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabCaches
	m.mode = modeConfirm
	m.confirmDocker = true
	updated, cmd := m.Update(dockerPruneDoneMsg{reclaimed: 4_200_000_000})
	m2 := updated.(model)
	if m2.mode != modeNormal || m2.confirmDocker {
		t.Errorf("mode=%v confirmDocker=%v, want modeNormal+false", m2.mode, m2.confirmDocker)
	}
	if m2.cachesNote == "" {
		t.Error("cachesNote empty, want reclaimed note")
	}
	if cmd == nil {
		t.Error("expected dockerSizeCmd refresh, got nil")
	}
}

func TestDockerPruneErrMsg(t *testing.T) {
	m := newTestModel()
	m.mode = modeConfirm
	m.confirmDocker = true
	updated, _ := m.Update(dockerPruneErrMsg{err: errors.New("daemon down")})
	m2 := updated.(model)
	if m2.mode != modeNormal || m2.confirmDocker || m2.errMsg == "" {
		t.Errorf("mode=%v confirmDocker=%v errMsg=%q, want modeNormal+false with error",
			m2.mode, m2.confirmDocker, m2.errMsg)
	}
}

func TestEntryDisplayName_Caches(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabCaches
	e := Entry{Name: "npm cache", Path: "/home/u/.npm/_cacache", IsDir: true}
	if got := m.entryDisplayName(e); got != "npm cache · /home/u/.npm/_cacache" {
		t.Errorf("entryDisplayName = %q", got)
	}
	d := Entry{Name: "Docker (system prune -a)", Path: dockerEntryPath}
	if got := m.entryDisplayName(d); got != "Docker (system prune -a)" {
		t.Errorf("docker entryDisplayName = %q", got)
	}
}

func TestViewConfirm_Docker(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabCaches
	m.mode = modeConfirm
	m.confirmDocker = true
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "Docker (system prune -a)", Path: dockerEntryPath, Sized: true}}
	ct.entries = ct.allEntries

	out := m.View()
	if !strings.Contains(out, "docker system prune -a -f") {
		t.Error("confirm dialog must show the exact prune command")
	}
}
