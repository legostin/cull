package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func specialKeyMsg(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}

func newKeysTestModel() model {
	m := newModel("/tmp/test", 100, false, false)
	m.width = 120
	m.height = 40
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "aaa", Path: "/tmp/test/aaa", Size: 300},
		{Name: "bbb", Path: "/tmp/test/bbb", Size: 200},
		{Name: "ccc", Path: "/tmp/test/ccc", Size: 100},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.cursor = 0
	return m
}

func TestKey_J_MovesDown(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 0
	result, _ := m.Update(keyMsg("j"))
	rm := result.(model)
	if rm.tab().cursor != 1 {
		t.Errorf("cursor = %d, want 1", rm.tab().cursor)
	}
}

func TestKey_K_MovesUp(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 2
	result, _ := m.Update(keyMsg("k"))
	rm := result.(model)
	if rm.tab().cursor != 1 {
		t.Errorf("cursor = %d, want 1", rm.tab().cursor)
	}
}

func TestKey_J_BoundaryClamping(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = len(m.tab().entries) - 1
	result, _ := m.Update(keyMsg("j"))
	rm := result.(model)
	if rm.tab().cursor != len(m.tab().entries)-1 {
		t.Errorf("cursor should not go past last entry")
	}
}

func TestKey_K_BoundaryClamping(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 0
	result, _ := m.Update(keyMsg("k"))
	rm := result.(model)
	if rm.tab().cursor != 0 {
		t.Errorf("cursor should not go below 0")
	}
}

func TestKey_G_JumpToTop(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 3
	result, _ := m.Update(keyMsg("g"))
	rm := result.(model)
	if rm.tab().cursor != 0 {
		t.Errorf("cursor = %d, want 0", rm.tab().cursor)
	}
}

func TestKey_ShiftG_JumpToBottom(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 0
	result, _ := m.Update(keyMsg("G"))
	rm := result.(model)
	if rm.tab().cursor != 3 {
		t.Errorf("cursor = %d, want 3", rm.tab().cursor)
	}
}

func TestKey_S_ToggleSelect(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 1 // "aaa"

	// Select
	result, _ := m.Update(keyMsg("s"))
	rm := result.(model)
	if !rm.tab().selected["/tmp/test/aaa"] {
		t.Error("aaa should be selected")
	}

	// Deselect
	result, _ = rm.Update(keyMsg("s"))
	rm = result.(model)
	if rm.tab().selected["/tmp/test/aaa"] {
		t.Error("aaa should be deselected")
	}
}

func TestKey_S_CannotSelectParent(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 0 // parent ".."

	result, _ := m.Update(keyMsg("s"))
	rm := result.(model)
	if len(rm.tab().selected) != 0 {
		t.Error("parent should not be selectable")
	}
}

func TestKey_ShiftS_RangeSelect(t *testing.T) {
	m := newKeysTestModel()
	tab := m.tab()
	tab.cursor = 1
	tab.lastSelect = 1

	// Move cursor to 3 and range select
	tab.cursor = 3
	result, _ := m.Update(keyMsg("S"))
	rm := result.(model)
	rt := rm.tab()
	if !rt.selected["/tmp/test/aaa"] || !rt.selected["/tmp/test/bbb"] || !rt.selected["/tmp/test/ccc"] {
		t.Error("range select should select entries 1-3")
	}
}

func TestKey_ShiftTab_SwitchesTab_NoHistory(t *testing.T) {
	m := newKeysTestModel()
	// Empty registry → 4 tabs (BROWSE / LARGEST / CACHES / PROJECTS)
	m.trashRegistry = &TrashRegistry{}
	m.activeTab = tabBrowse

	want := []tabID{tabLargest, tabCaches, tabProjects, tabBrowse}
	for _, w := range want {
		result, _ := m.Update(keyMsg("shift+tab"))
		m = result.(model)
		if m.activeTab != w {
			t.Fatalf("activeTab = %d, want %d", m.activeTab, w)
		}
	}
}

func TestKey_ShiftTab_SwitchesTab_WithHistory(t *testing.T) {
	m := newKeysTestModel()
	// Add a record so HISTORY tab appears
	m.trashRegistry = &TrashRegistry{
		Records: []TrashRecord{
			{OriginalPath: "/tmp/gone.txt", TrashPath: "/trash/gone.txt", DeletedAt: time.Now()},
		},
	}
	m.activeTab = tabBrowse

	want := []tabID{tabLargest, tabCaches, tabProjects, tabHistory, tabBrowse}
	for _, w := range want {
		result, _ := m.Update(keyMsg("shift+tab"))
		m = result.(model)
		if m.activeTab != w {
			t.Fatalf("activeTab = %d, want %d", m.activeTab, w)
		}
	}
}

func TestKey_D_EntersConfirmMode(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 1

	result, _ := m.Update(keyMsg("d"))
	rm := result.(model)
	if rm.mode != modeConfirm {
		t.Errorf("mode = %d, want modeConfirm", rm.mode)
	}
	// Should auto-select cursor item
	if !rm.tab().selected["/tmp/test/aaa"] {
		t.Error("cursor item should be auto-selected")
	}
}

func TestKey_D_OnParent_NoOp(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 0 // parent

	result, _ := m.Update(keyMsg("d"))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Error("d on parent should not enter confirm mode")
	}
}

func TestKey_F_EntersFilterMode(t *testing.T) {
	m := newKeysTestModel()
	result, _ := m.Update(keyMsg("f"))
	rm := result.(model)
	if rm.mode != modeFilter {
		t.Errorf("mode = %d, want modeFilter", rm.mode)
	}
}

func TestKey_Question_EntersHelp(t *testing.T) {
	m := newKeysTestModel()
	result, _ := m.Update(keyMsg("?"))
	rm := result.(model)
	if rm.mode != modeHelp {
		t.Errorf("mode = %d, want modeHelp", rm.mode)
	}
}

func TestKey_HelpMode_AnyKeyExits(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeHelp

	result, _ := m.Update(keyMsg("x"))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after key in help", rm.mode)
	}
}

func TestKey_Q_Quits(t *testing.T) {
	m := newKeysTestModel()
	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Error("q should return a quit command")
	}
}

func TestKey_CtrlC_Quits(t *testing.T) {
	m := newKeysTestModel()
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestKey_Tab_ToggleDeleteMode(t *testing.T) {
	m := newKeysTestModel()
	if m.deleteType != deleteTrash {
		t.Fatal("default should be trash")
	}

	result, _ := m.Update(keyMsg("tab"))
	rm := result.(model)
	if rm.deleteType != deletePermanent {
		t.Error("tab should switch to permanent")
	}

	result, _ = rm.Update(keyMsg("tab"))
	rm = result.(model)
	if rm.deleteType != deleteTrash {
		t.Error("tab again should switch to trash")
	}
}

func TestKey_T_CycleSort(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabBrowse
	if m.sortBy != sortSizeDesc {
		t.Fatal("default sort should be size desc")
	}

	result, _ := m.Update(keyMsg("t"))
	rm := result.(model)
	if rm.sortBy != sortNameAsc {
		t.Errorf("sortBy = %d, want sortNameAsc", rm.sortBy)
	}

	result, _ = rm.Update(keyMsg("t"))
	rm = result.(model)
	if rm.sortBy != sortUpdatedDesc {
		t.Errorf("sortBy = %d, want sortUpdatedDesc", rm.sortBy)
	}

	result, _ = rm.Update(keyMsg("t"))
	rm = result.(model)
	if rm.sortBy != sortCreatedDesc {
		t.Errorf("sortBy = %d, want sortCreatedDesc", rm.sortBy)
	}

	result, _ = rm.Update(keyMsg("t"))
	rm = result.(model)
	if rm.sortBy != sortSizeDesc {
		t.Errorf("sortBy = %d, want sortSizeDesc (wrapped)", rm.sortBy)
	}
}

func TestKey_T_NotInLargestTab(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabLargest
	m.sortBy = sortSizeDesc

	result, _ := m.Update(keyMsg("t"))
	rm := result.(model)
	if rm.sortBy != sortSizeDesc {
		t.Error("t in largest tab should be a no-op")
	}
}

func TestKey_H_ToggleHidden(t *testing.T) {
	m := newKeysTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: ".hidden", Path: "/tmp/test/.hidden"},
		{Name: "visible", Path: "/tmp/test/visible"},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)

	if !m.showHidden {
		t.Fatal("default showHidden should be true")
	}

	result, _ := m.Update(keyMsg("h"))
	rm := result.(model)
	if rm.showHidden {
		t.Error("h should toggle hidden off")
	}
	if len(rm.tab().entries) != 1 {
		t.Errorf("entries = %d, want 1 (hidden filtered)", len(rm.tab().entries))
	}
}

func TestKey_E_DryRunMode(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 1

	result, _ := m.Update(keyMsg("e"))
	rm := result.(model)
	if rm.mode != modeDryRun {
		t.Errorf("mode = %d, want modeDryRun", rm.mode)
	}
}

func TestKey_DryRun_EscExits(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeDryRun

	result, _ := m.Update(specialKeyMsg(tea.KeyEsc))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Error("esc should exit dry run mode")
	}
}

// --- Filter mode tests ---

func TestFilterMode_TypingAddsChars(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeFilter

	result, _ := m.Update(keyMsg("a"))
	rm := result.(model)
	if rm.tab().filterText != "a" {
		t.Errorf("filterText = %q, want 'a'", rm.tab().filterText)
	}

	result, _ = rm.Update(keyMsg("b"))
	rm = result.(model)
	if rm.tab().filterText != "ab" {
		t.Errorf("filterText = %q, want 'ab'", rm.tab().filterText)
	}
}

func TestFilterMode_BackspaceRemoves(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeFilter
	m.tab().filterText = "abc"

	result, _ := m.Update(specialKeyMsg(tea.KeyBackspace))
	rm := result.(model)
	if rm.tab().filterText != "ab" {
		t.Errorf("filterText = %q, want 'ab'", rm.tab().filterText)
	}
}

func TestFilterMode_BackspaceEmpty(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeFilter
	m.tab().filterText = ""

	result, _ := m.Update(specialKeyMsg(tea.KeyBackspace))
	rm := result.(model)
	if rm.tab().filterText != "" {
		t.Errorf("filterText = %q, want empty", rm.tab().filterText)
	}
}

func TestFilterMode_EscClears(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeFilter
	m.tab().filterText = "search"

	result, _ := m.Update(specialKeyMsg(tea.KeyEsc))
	rm := result.(model)
	if rm.tab().filterText != "" {
		t.Errorf("esc should clear filter, got %q", rm.tab().filterText)
	}
	if rm.mode != modeNormal {
		t.Error("esc should exit filter mode")
	}
}

func TestFilterMode_EnterKeepsFilter(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeFilter
	m.tab().filterText = "search"

	result, _ := m.Update(specialKeyMsg(tea.KeyEnter))
	rm := result.(model)
	if rm.tab().filterText != "search" {
		t.Error("enter should keep filter text")
	}
	if rm.mode != modeNormal {
		t.Error("enter should exit filter mode")
	}
}

// --- Confirm mode tests ---

func TestConfirmMode_N_Cancels(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeConfirm
	m.tab().selected["/tmp/test/aaa"] = true

	result, _ := m.Update(keyMsg("n"))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Error("n should cancel confirm mode")
	}
}

func TestConfirmMode_Esc_Cancels(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeConfirm

	result, _ := m.Update(specialKeyMsg(tea.KeyEsc))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Error("esc should cancel confirm mode")
	}
}

func TestConfirmMode_Y_ReturnsCmd(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeConfirm
	m.tab().selected["/tmp/test/aaa"] = true

	_, cmd := m.Update(keyMsg("y"))
	if cmd == nil {
		t.Error("y in confirm mode should return a delete command")
	}
}

func TestConfirmMode_Q_Quits(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeConfirm

	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Error("q in confirm mode should quit")
	}
}

func TestKey_ShiftS_NoLastSelect(t *testing.T) {
	m := newKeysTestModel()
	tab := m.tab()
	tab.cursor = 2
	tab.lastSelect = -1

	result, _ := m.Update(keyMsg("S"))
	rm := result.(model)
	rt := rm.tab()
	// With lastSelect=-1, start should be 0, so range is 0..2
	// But entry 0 is parent and should not be selected
	if rt.selected["/tmp"] {
		t.Error("parent should not be selected in range select")
	}
	if !rt.selected["/tmp/test/aaa"] || !rt.selected["/tmp/test/bbb"] {
		t.Error("non-parent entries in range should be selected")
	}
}

func TestKey_D_EmptyEntries(t *testing.T) {
	m := newKeysTestModel()
	m.tab().entries = nil

	result, _ := m.Update(keyMsg("d"))
	rm := result.(model)
	if rm.mode != modeNormal {
		t.Error("d with no entries should be no-op")
	}
}

func TestFilterMode_FiltersEntries(t *testing.T) {
	m := newKeysTestModel()
	m.mode = modeFilter

	// Type "bbb" to filter
	for _, r := range "bbb" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(model)
	}

	tab := m.tab()
	// Should have parent + bbb
	found := false
	for _, e := range tab.entries {
		if e.Name == "bbb" {
			found = true
		}
		if e.Name == "aaa" || e.Name == "ccc" {
			t.Error("filtered entries should not include non-matching items")
		}
	}
	if !found {
		t.Error("filter should include matching entry 'bbb'")
	}
}

func TestDockerRowNotSelectable(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabCaches
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "Docker (system prune -a)", Path: dockerEntryPath}}
	ct.entries = ct.allEntries
	ct.cursor = 0

	updated, _ := m.Update(keyMsg("s"))
	m2 := updated.(model)
	if len(m2.tabs[tabCaches].selected) != 0 {
		t.Error("docker row must not be selectable with s")
	}
}

func TestDockerPruneConfirmFlow(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabCaches
	m.skipConfirm = true // -y must be ignored for docker prune
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "Docker (system prune -a)", Path: dockerEntryPath, Sized: true}}
	ct.entries = ct.allEntries
	ct.cursor = 0

	updated, _ := m.Update(keyMsg("d"))
	m2 := updated.(model)
	if m2.mode != modeConfirm || !m2.confirmDocker {
		t.Fatalf("mode=%v confirmDocker=%v, want confirm+docker", m2.mode, m2.confirmDocker)
	}

	// n cancels and clears the docker flag
	updated, _ = m2.Update(keyMsg("n"))
	m3 := updated.(model)
	if m3.mode != modeNormal || m3.confirmDocker {
		t.Errorf("after n: mode=%v confirmDocker=%v, want normal+false", m3.mode, m3.confirmDocker)
	}

	// y returns the prune command; the flag stays set until the prune
	// finishes (dockerPruneDoneMsg/ErrMsg clear it). Do NOT execute cmd —
	// it would really run docker prune.
	updated, cmd := m2.Update(keyMsg("y"))
	m4 := updated.(model)
	if cmd == nil {
		t.Fatal("y must return the docker prune command")
	}
	if !m4.confirmDocker || m4.mode != modeConfirm {
		t.Errorf("after y: mode=%v confirmDocker=%v, want confirm+true until prune completes",
			m4.mode, m4.confirmDocker)
	}
}

func TestDockerPruneReadOnly(t *testing.T) {
	m := newKeysTestModel()
	m.readOnly = true
	m.activeTab = tabCaches
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "Docker (system prune -a)", Path: dockerEntryPath, Sized: true}}
	ct.entries = ct.allEntries
	ct.cursor = 0

	updated, cmd := m.Update(keyMsg("d"))
	m2 := updated.(model)
	if m2.mode != modeNormal || m2.confirmDocker || cmd != nil {
		t.Error("read-only mode must ignore d on the docker row")
	}
}

func TestBuildDeleteCmd_ExpandsCacheGroups(t *testing.T) {
	dir1 := t.TempDir()
	sub1 := filepath.Join(dir1, "cache-a")
	sub2 := filepath.Join(dir1, "cache-b")
	mustMkdir(t, sub1)
	mustMkdir(t, sub2)
	mustWriteFile(t, filepath.Join(sub1, "f"), 10)
	mustWriteFile(t, filepath.Join(sub2, "f"), 10)

	m := newKeysTestModel()
	m.activeTab = tabCaches
	m.deleteType = deletePermanent
	m.cachePathGroups = map[string][]string{sub1: {sub1, sub2}}
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "grouped cache", Path: sub1, IsDir: true, Size: 20, Sized: true}}
	ct.entries = ct.allEntries
	ct.selected = map[string]bool{sub1: true}

	cmd := m.buildDeleteCmd(ct)
	msg := cmd()
	done, ok := msg.(deleteDoneMsg)
	if !ok {
		t.Fatalf("got %T (%v), want deleteDoneMsg", msg, msg)
	}
	if len(done.deleted) != 2 {
		t.Fatalf("deleted = %v, want both group paths", done.deleted)
	}
	for _, p := range []string{sub1, sub2} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists", p)
		}
	}
}

func TestKey_ShiftTab_StartsProjectsScanOnce(t *testing.T) {
	m := newKeysTestModel()
	m.trashRegistry = &TrashRegistry{}
	m.activeTab = tabCaches

	result, cmd := m.Update(keyMsg("shift+tab"))
	m2 := result.(model)
	if m2.activeTab != tabProjects {
		t.Fatalf("activeTab = %d, want tabProjects", m2.activeTab)
	}
	if !m2.projectsScanning || cmd == nil {
		t.Error("first entry must start the projects scan")
	}

	// second entry after load: no rescan
	m2.projectsLoaded = true
	m2.projectsScanning = false
	m2.activeTab = tabCaches
	result3, _ := m2.Update(keyMsg("shift+tab"))
	if result3.(model).projectsScanning {
		t.Error("re-entering a loaded PROJECTS tab must not rescan")
	}
}

func TestKey_R_ProjectsRescan(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabProjects
	m.projectsLoaded = true

	result, cmd := m.Update(keyMsg("r"))
	m2 := result.(model)
	if !m2.projectsScanning || cmd == nil {
		t.Error("r on PROJECTS tab must trigger a rescan")
	}
	if m2.projectsLoaded {
		t.Error("rescan must reset projectsLoaded")
	}
}

func TestKey_T_ProjectsSortToggle(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabProjects
	now := time.Now()
	pt := &m.tabs[tabProjects]
	pt.allEntries = []Entry{
		{Name: "small-old", Path: "/p/a", Size: 1, Sized: true, ModTime: now.Add(-100 * time.Hour)},
		{Name: "big-fresh", Path: "/p/b", Size: 100, Sized: true, ModTime: now},
	}
	m.applyFilterForTab(tabProjects)

	result, _ := m.Update(keyMsg("t"))
	m2 := result.(model)
	if !m2.projectsSortIdle {
		t.Fatal("t must switch PROJECTS to idle sort")
	}
	if m2.tabs[tabProjects].entries[0].Path != "/p/a" {
		t.Error("idle sort must put oldest project first")
	}

	result3, _ := m2.Update(keyMsg("t"))
	m3 := result3.(model)
	if m3.projectsSortIdle {
		t.Fatal("second t must switch back to size sort")
	}
	if m3.tabs[tabProjects].entries[0].Path != "/p/b" {
		t.Error("size sort must put biggest artifact first")
	}
}

func TestKey_M_TogglesMapOnBrowseOnly(t *testing.T) {
	m := newKeysTestModel()
	result, _ := m.Update(keyMsg("m"))
	if !result.(model).browseMap {
		t.Fatal("m must enable map mode on BROWSE")
	}
	result2, _ := result.(model).Update(keyMsg("m"))
	if result2.(model).browseMap {
		t.Fatal("second m must disable map mode")
	}
	m2 := newKeysTestModel()
	m2.activeTab = tabCaches
	result3, _ := m2.Update(keyMsg("m"))
	if result3.(model).browseMap {
		t.Error("m must be a no-op outside BROWSE")
	}
}

func TestKey_MapMode_SpatialMove(t *testing.T) {
	m := newKeysTestModel() // aaa 300, bbb 200, ccc 100 + parent
	m.browseMap = true
	for i := range m.tab().allEntries {
		m.tab().allEntries[i].Sized = true
	}
	m.tab().entries = append([]Entry{}, m.tab().allEntries...)
	m.tab().cursor = 1 // aaa — largest, leftmost rect
	result, _ := m.Update(keyMsg("l"))
	rm := result.(model)
	if rm.tab().cursor == 1 {
		t.Error("l must move cursor spatially to a neighbor rect")
	}
	if rm.tab().entries[rm.tab().cursor].IsParent {
		t.Error("cursor must never land on the parent entry")
	}
}

func TestKey_MapMode_EnterOnCollapsedReturnsToList(t *testing.T) {
	m := newKeysTestModel()
	m.browseMap = true
	m.width, m.height = 42, 21 // small content area forces collapsing
	tab := m.tab()
	tab.allEntries = []Entry{{Name: "big", Path: "/tmp/test/big", Size: 1 << 30, Sized: true, IsDir: true}}
	for i := 0; i < 30; i++ {
		tab.allEntries = append(tab.allEntries, Entry{Name: "tiny", Path: "/tmp/test/t", Size: 1, Sized: true})
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.cursor = len(tab.entries) - 1 // a collapsed tiny entry
	result, _ := m.Update(specialKeyMsg(tea.KeyEnter))
	if result.(model).browseMap {
		t.Error("enter on the +N more rect must return to list view")
	}
}

func TestKey_M_ParksCursorOffParent(t *testing.T) {
	m := newKeysTestModel()
	for i := range m.tab().allEntries {
		m.tab().allEntries[i].Sized = true
	}
	m.tab().entries = append([]Entry{}, m.tab().allEntries...)
	m.tab().cursor = 0 // ".." parent — has no rectangle on the map
	result, _ := m.Update(keyMsg("m"))
	rm := result.(model)
	if rm.tab().entries[rm.tab().cursor].IsParent {
		t.Error("entering map mode must move the cursor off the parent entry")
	}
}

func TestProjectsTabFollowsCurrentDir(t *testing.T) {
	m := newKeysTestModel()
	m.projectsLoaded = true
	m.projectsRoots = []string{"/tmp/test"}
	m.path = "/tmp/test/sub" // user navigated deeper since the last scan

	result, cmd := m.switchToTab(tabProjects)
	rm := result.(model)
	if !rm.projectsScanning || cmd == nil {
		t.Fatal("entering PROJECTS after a path change must rescan")
	}
	if len(rm.projectsRoots) != 1 || rm.projectsRoots[0] != "/tmp/test/sub" {
		t.Errorf("projectsRoots = %v, want [/tmp/test/sub]", rm.projectsRoots)
	}

	// same path again: no rescan
	rm.projectsScanning = false
	rm.projectsLoaded = true
	result2, _ := rm.switchToTab(tabProjects)
	if result2.(model).projectsScanning {
		t.Error("entering PROJECTS with an unchanged path must not rescan")
	}
}

func TestProjectsStaleLoadDiscarded(t *testing.T) {
	m := newKeysTestModel()
	m.projectsRoots = []string{"/tmp/new"}
	msg := projectsLoadedMsg{
		root:    "/tmp/old",
		entries: []Entry{{Name: "x", Path: "/tmp/old/x/node_modules"}},
	}
	result, _ := m.Update(msg)
	rm := result.(model)
	if len(rm.tabs[tabProjects].allEntries) != 0 {
		t.Error("a load result from a stale root must be discarded")
	}
}

func TestKey_CtrlL_ForcesRepaint(t *testing.T) {
	m := newKeysTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if cmd == nil {
		t.Error("ctrl+l must issue a repaint command")
	}
}

func TestKey_D_TMSnapshotsRowConfirms(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabCaches
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "Time Machine local snapshots (3)", Path: tmSnapEntryPath, Sized: true}}
	ct.entries = ct.allEntries
	ct.cursor = 0
	m.skipConfirm = true // even -y must not skip

	result, _ := m.Update(keyMsg("d"))
	rm := result.(model)
	if rm.mode != modeConfirm || !rm.confirmSnap {
		t.Error("d on the snapshots row must open the snapshot confirm dialog")
	}
}

func TestKey_S_CannotSelectSnapshotsRow(t *testing.T) {
	m := newKeysTestModel()
	m.activeTab = tabCaches
	ct := &m.tabs[tabCaches]
	ct.allEntries = []Entry{{Name: "snapshots", Path: tmSnapEntryPath, Sized: true}}
	ct.entries = ct.allEntries
	ct.cursor = 0
	result, _ := m.Update(keyMsg("s"))
	rm := result.(model)
	if len(rm.tab().selected) != 0 {
		t.Error("snapshots row must not be selectable")
	}
}

// newPagingTestModel builds a model with enough entries to page through.
func newPagingTestModel(n int) model {
	m := newModel("/tmp/test", 100, false, false)
	m.width = 120
	m.height = 40
	tab := m.tab()
	tab.allEntries = make([]Entry, n)
	for i := range tab.allEntries {
		tab.allEntries[i] = Entry{
			Name: fmt.Sprintf("file%03d", i),
			Path: fmt.Sprintf("/tmp/test/file%03d", i),
			Size: int64(n - i),
		}
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	return m
}

func TestKey_Home_JumpToTop(t *testing.T) {
	m := newPagingTestModel(100)
	m.tab().cursor = 57
	m.clampOffset()
	result, _ := m.Update(specialKeyMsg(tea.KeyHome))
	rm := result.(model)
	if rm.tab().cursor != 0 {
		t.Errorf("cursor = %d, want 0", rm.tab().cursor)
	}
	if rm.tab().offset != 0 {
		t.Errorf("offset = %d, want 0", rm.tab().offset)
	}
}

func TestKey_End_JumpToBottom(t *testing.T) {
	m := newPagingTestModel(100)
	m.tab().cursor = 0
	result, _ := m.Update(specialKeyMsg(tea.KeyEnd))
	rm := result.(model)
	if rm.tab().cursor != 99 {
		t.Errorf("cursor = %d, want 99", rm.tab().cursor)
	}
	if got := rm.tab().offset; got != 99-rm.visibleRowCount()+1 {
		t.Errorf("offset = %d, want %d", got, 99-rm.visibleRowCount()+1)
	}
}

func TestKey_End_EmptyList(t *testing.T) {
	m := newPagingTestModel(0)
	result, _ := m.Update(specialKeyMsg(tea.KeyEnd))
	rm := result.(model)
	if rm.tab().cursor != 0 {
		t.Errorf("cursor = %d, want 0", rm.tab().cursor)
	}
}

func TestKey_PgDown_MovesOnePage(t *testing.T) {
	m := newPagingTestModel(100)
	m.tab().cursor = 0
	result, _ := m.Update(specialKeyMsg(tea.KeyPgDown))
	rm := result.(model)
	want := rm.visibleRowCount()
	if rm.tab().cursor != want {
		t.Errorf("cursor = %d, want %d (one page)", rm.tab().cursor, want)
	}
}

func TestKey_PgDown_ClampsToLastEntry(t *testing.T) {
	m := newPagingTestModel(100)
	m.tab().cursor = 95
	m.clampOffset()
	result, _ := m.Update(specialKeyMsg(tea.KeyPgDown))
	rm := result.(model)
	if rm.tab().cursor != 99 {
		t.Errorf("cursor = %d, want 99", rm.tab().cursor)
	}
}

func TestKey_PgUp_MovesOnePage(t *testing.T) {
	m := newPagingTestModel(100)
	m.tab().cursor = 60
	m.clampOffset()
	result, _ := m.Update(specialKeyMsg(tea.KeyPgUp))
	rm := result.(model)
	want := 60 - rm.visibleRowCount()
	if rm.tab().cursor != want {
		t.Errorf("cursor = %d, want %d (one page up)", rm.tab().cursor, want)
	}
}

func TestKey_PgUp_ClampsToZero(t *testing.T) {
	m := newPagingTestModel(100)
	m.tab().cursor = 3
	result, _ := m.Update(specialKeyMsg(tea.KeyPgUp))
	rm := result.(model)
	if rm.tab().cursor != 0 {
		t.Errorf("cursor = %d, want 0", rm.tab().cursor)
	}
}

func TestKey_PgDown_EmptyList(t *testing.T) {
	m := newPagingTestModel(0)
	result, _ := m.Update(specialKeyMsg(tea.KeyPgDown))
	rm := result.(model)
	if rm.tab().cursor != 0 {
		t.Errorf("cursor = %d, want 0", rm.tab().cursor)
	}
}

func TestHelp_ListsPagingKeys(t *testing.T) {
	m := newPagingTestModel(10)
	m.height = 80 // tall enough that the NAVIGATION section is fully visible
	m.mode = modeHelp
	out := m.View()
	for _, want := range []string{"home", "end", "pgup", "pgdn"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
}
