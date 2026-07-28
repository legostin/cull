package main

import (
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
	// Empty registry → 3 tabs (BROWSE / LARGEST / CACHES)
	m.trashRegistry = &TrashRegistry{}
	m.activeTab = tabBrowse

	want := []tabID{tabLargest, tabCaches, tabBrowse}
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

	want := []tabID{tabLargest, tabCaches, tabHistory, tabBrowse}
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
