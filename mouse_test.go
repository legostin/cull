package main

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRowsStartY(t *testing.T) {
	m := newTestModel()
	if got := m.rowsStartY(); got != 8 {
		t.Errorf("rowsStartY = %d, want 8", got)
	}
	m.errMsg = "boom"
	if got := m.rowsStartY(); got != 9 {
		t.Errorf("rowsStartY with error = %d, want 9", got)
	}
}

func TestTabAtX(t *testing.T) {
	m := newTestModel()
	m.trashRegistry = &TrashRegistry{} // no HISTORY tab
	// Layout: " [BROWSE] LARGEST CACHES PROJECTS" (active first tab).
	// First part starts at x=1; "[BROWSE]" is 8 wide.
	if got, ok := m.tabAtX(2); !ok || got != tabBrowse {
		t.Errorf("x=2: got %v/%v, want tabBrowse", got, ok)
	}
	// " LARGEST " starts at x=10 (1 + 8 + 1 gap)
	if got, ok := m.tabAtX(11); !ok || got != tabLargest {
		t.Errorf("x=11: got %v/%v, want tabLargest", got, ok)
	}
	if _, ok := m.tabAtX(500); ok {
		t.Error("x beyond the bar must miss")
	}
}

func TestMouseClickSetsCursorAndDoubleClickEnters(t *testing.T) {
	m := newTestModel()
	tab := m.tab()
	tab.allEntries = []Entry{
		{Name: "..", Path: "/tmp", IsDir: true, IsParent: true},
		{Name: "aaa", Path: "/tmp/test/aaa", Size: 300, Sized: true},
		{Name: "sub", Path: "/tmp/test/sub", Size: 200, Sized: true, IsDir: true},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)

	click := tea.MouseMsg{X: 5, Y: m.rowsStartY() + 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	result, _ := m.Update(click)
	rm := result.(model)
	if rm.tab().cursor != 2 {
		t.Fatalf("cursor = %d, want 2 after click", rm.tab().cursor)
	}
	// second click within the window enters the dir (navigateInto issues a scan cmd)
	result2, cmd := rm.Update(click)
	rm2 := result2.(model)
	if cmd == nil && rm2.path == "/tmp/test" {
		t.Error("double-click on a dir must navigate into it")
	}
}

func TestMouseWheelMovesCursor(t *testing.T) {
	m := newKeysTestModel()
	m.tab().cursor = 0
	down := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	result, _ := m.Update(down)
	rm := result.(model)
	if rm.tab().cursor != 3 {
		t.Errorf("cursor = %d, want 3 after wheel down", rm.tab().cursor)
	}
	result2, _ := rm.Update(down)
	rm2 := result2.(model)
	if rm2.tab().cursor != 3 {
		t.Errorf("cursor = %d, want clamped at 3", rm2.tab().cursor)
	}
	up := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
	result3, _ := rm2.Update(up)
	rm3 := result3.(model)
	if rm3.tab().cursor != 0 {
		t.Errorf("cursor = %d, want 0 after wheel up", rm3.tab().cursor)
	}
}

func TestMouseRightClickTogglesSelect(t *testing.T) {
	m := newKeysTestModel()
	rc := tea.MouseMsg{X: 5, Y: m.rowsStartY() + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	result, _ := m.Update(rc)
	rm := result.(model)
	if !rm.tab().selected["/tmp/test/aaa"] {
		t.Fatal("right click must select the row")
	}
	result2, _ := rm.Update(rc)
	rm2 := result2.(model)
	if rm2.tab().selected["/tmp/test/aaa"] {
		t.Error("second right click must deselect")
	}
	// read-only: no selection
	ro := newKeysTestModel()
	ro.readOnly = true
	result3, _ := ro.Update(rc)
	rm3 := result3.(model)
	if len(rm3.tab().selected) != 0 {
		t.Error("read-only must ignore right click")
	}
}

func TestMouseTabBarClickSwitches(t *testing.T) {
	m := newTestModel()
	m.trashRegistry = &TrashRegistry{}
	click := tea.MouseMsg{X: 11, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	result, _ := m.Update(click)
	if result.(model).activeTab != tabLargest {
		t.Errorf("activeTab = %v, want tabLargest", result.(model).activeTab)
	}
}

func TestMouseMapClickSetsCursor(t *testing.T) {
	m := newTestModel()
	m.browseMap = true
	tab := &m.tabs[tabBrowse]
	tab.allEntries = []Entry{
		{Name: "big", Path: "/p/big", Size: 1 << 30, Sized: true, IsDir: true},
		{Name: "small", Path: "/p/small", Size: 100 << 20, Sized: true, IsDir: true},
	}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.cursor = 0

	rects := m.browseMapLayout()
	var smallRect mapRect
	for _, r := range rects {
		if r.Index == 1 {
			smallRect = r
		}
	}
	click := tea.MouseMsg{
		X:      smallRect.X + smallRect.W/2,
		Y:      m.rowsStartY() + smallRect.Y + smallRect.H/2,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}
	result, _ := m.Update(click)
	rm := result.(model)
	if rm.tab().cursor != 1 {
		t.Errorf("cursor = %d, want 1 after map click", rm.tab().cursor)
	}
}

func TestDoubleClickWindow(t *testing.T) {
	m := newTestModel()
	m.lastClickRow = 2
	m.lastClickAt = time.Now().Add(-time.Second)
	if m.isDoubleClick(2) {
		t.Error("1s-old click must not count as double")
	}
	m.lastClickAt = time.Now().Add(-100 * time.Millisecond)
	if !m.isDoubleClick(2) {
		t.Error("100ms-old click on same row must count as double")
	}
	if m.isDoubleClick(3) {
		t.Error("different row must not count as double")
	}
}
