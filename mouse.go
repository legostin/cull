package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// doubleClickWindow is the max delay between two clicks on the same row to
// count as a double click.
const doubleClickWindow = 400 * time.Millisecond

// tabBarY is the 0-based screen line of the tab bar (logo 0–3, path 4).
const tabBarY = 5

// rowsStartY returns the 0-based screen line of the first entry row (and of
// the treemap's top edge): logo(4) + path + tabbar + separator + header,
// shifted by one when the error line is shown.
func (m *model) rowsStartY() int {
	y := 8
	if m.errMsg != "" {
		y++
	}
	return y
}

// tabAtX maps a screen x on the tab bar line to a tab id. Mirrors
// renderTabBar's layout: leading space, parts of width len(label)+2 joined
// by single spaces.
func (m *model) tabAtX(x int) (tabID, bool) {
	pos := 1
	for _, ti := range m.tabInfos() {
		w := lipgloss.Width(ti.name) + 2
		if x >= pos && x < pos+w {
			return ti.id, true
		}
		pos += w + 1
	}
	return 0, false
}

// isDoubleClick reports whether a click on row counts as the second click
// of a double click.
func (m *model) isDoubleClick(row int) bool {
	return row == m.lastClickRow && time.Since(m.lastClickAt) <= doubleClickWindow
}

// noteClick records a click target for double-click detection.
func (m *model) noteClick(row int) {
	m.lastClickRow = row
	m.lastClickAt = time.Now()
}

// handleMouse processes press events: clicks, double clicks, wheel.
func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || m.mode != modeNormal {
		return m, nil
	}
	t := m.tab()

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if m.activeTab == tabBrowse && m.browseMap {
			return m, nil
		}
		step := 3
		if msg.Button == tea.MouseButtonWheelUp {
			step = -3
		}
		t.cursor += step
		if t.cursor < 0 {
			t.cursor = 0
		}
		if t.cursor >= len(t.entries) && len(t.entries) > 0 {
			t.cursor = len(t.entries) - 1
		}
		m.clampOffset()
		return m, m.resetNameScroll()

	case tea.MouseButtonLeft:
		if msg.Y == tabBarY {
			if id, ok := m.tabAtX(msg.X); ok && id != m.activeTab {
				return m.switchToTab(id)
			}
			return m, nil
		}
		if m.activeTab == tabBrowse && m.browseMap {
			return m.handleMapClick(msg)
		}
		row := msg.Y - m.rowsStartY() + t.offset
		if row < t.offset || row >= len(t.entries) {
			return m, nil
		}
		double := m.isDoubleClick(row) && t.cursor == row
		t.cursor = row
		m.noteClick(row)
		if double && m.activeTab == tabBrowse {
			e := t.entries[row]
			if e.IsParent {
				return m.navigateUp()
			}
			if e.IsDir {
				return m.navigateInto(e.Path)
			}
		}
		return m, m.resetNameScroll()

	case tea.MouseButtonRight:
		if m.readOnly {
			return m, nil
		}
		var path string
		if m.activeTab == tabBrowse && m.browseMap {
			rects := m.browseMapLayout()
			ri, ok := mapRectAt(rects, msg.X, msg.Y-m.rowsStartY())
			if !ok || rects[ri].Index < 0 {
				return m, nil
			}
			path = t.entries[rects[ri].Index].Path
		} else {
			row := msg.Y - m.rowsStartY() + t.offset
			if row < t.offset || row >= len(t.entries) ||
				t.entries[row].IsParent || t.entries[row].Path == dockerEntryPath {
				return m, nil
			}
			path = t.entries[row].Path
			t.lastSelect = row
		}
		if t.selected[path] {
			delete(t.selected, path)
		} else {
			t.selected[path] = true
		}
		return m, nil
	}

	return m, nil
}

// handleMapClick moves the cursor to the clicked rect; a double click zooms
// into a directory or, on the "+N more" rect, returns to the list.
func (m model) handleMapClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	t := m.tab()
	rects := m.browseMapLayout()
	ri, ok := mapRectAt(rects, msg.X, msg.Y-m.rowsStartY())
	if !ok {
		return m, nil
	}
	r := rects[ri]
	// Use the rect slot as the double-click target (offset by entry count so
	// list rows and map rects never collide).
	target := len(t.entries) + ri
	double := m.isDoubleClick(target)
	m.noteClick(target)

	if r.Index >= 0 {
		t.cursor = r.Index
		if double {
			e := t.entries[r.Index]
			if e.IsDir {
				return m.navigateInto(e.Path)
			}
		}
		return m, m.resetNameScroll()
	}
	if double {
		m.browseMap = false // +N more
	}
	return m, nil
}

// mapRectAt returns the index of the rect containing grid point (x, y).
func mapRectAt(rects []mapRect, x, y int) (int, bool) {
	for i, r := range rects {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return i, true
		}
	}
	return 0, false
}
