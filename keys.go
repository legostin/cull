package main

import (
	"os"
	"os/exec"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.mode == modeFilter {
		return m.handleFilterKey(msg)
	}

	if m.mode == modeDryRun {
		return m.handleDryRunKey(msg)
	}

	if m.mode == modeHelp {
		m.mode = modeNormal
		return m, nil
	}

	// q quits in normal and confirm modes
	if msg.String() == "q" {
		return m, tea.Quit
	}

	if m.mode == modeConfirm {
		return m.handleConfirmKey(msg)
	}

	t := m.tab()

	switch msg.String() {
	case "shift+tab":
		m.activeTab = (m.activeTab + 1) % 2
		cmd := m.resetNameScroll()
		return m, cmd

	case "j", "down":
		if t.cursor < len(t.entries)-1 {
			t.cursor++
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "k", "up":
		if t.cursor > 0 {
			t.cursor--
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "g":
		t.cursor = 0
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "G":
		if len(t.entries) > 0 {
			t.cursor = len(t.entries) - 1
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "enter":
		// Only navigate in browse tab
		if m.activeTab != tabBrowse {
			return m, nil
		}
		if t.cursor < len(t.entries) && t.entries[t.cursor].IsParent {
			return m.navigateUp()
		}
		if t.cursor < len(t.entries) && t.entries[t.cursor].IsDir {
			return m.navigateInto(t.entries[t.cursor].Path)
		}

	case "backspace":
		if m.activeTab != tabBrowse {
			return m, nil
		}
		return m.navigateUp()

	case "esc":
		if m.activeTab != tabBrowse {
			return m, nil
		}
		return m.navigateUp()

	case "s":
		if t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent {
			p := t.entries[t.cursor].Path
			if t.selected[p] {
				delete(t.selected, p)
			} else {
				t.selected[p] = true
			}
			t.lastSelect = t.cursor
		}

	case "S":
		if t.cursor < len(t.entries) {
			start := t.lastSelect
			if start < 0 {
				start = 0
			}
			lo, hi := start, t.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi; i++ {
				if !t.entries[i].IsParent {
					t.selected[t.entries[i].Path] = true
				}
			}
			t.lastSelect = t.cursor
		}

	case "d":
		if len(t.entries) == 0 {
			return m, nil
		}
		if len(t.selected) == 0 && t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent {
			t.selected[t.entries[t.cursor].Path] = true
		}
		if len(t.selected) == 0 {
			return m, nil
		}
		m.mode = modeConfirm

	case " ":
		if t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent {
			cmd := exec.Command("qlmanage", "-p", t.entries[t.cursor].Path)
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Start()
		}

	case "f":
		m.mode = modeFilter

	case "h":
		m.showHidden = !m.showHidden
		m.applyFilter()
		if t.cursor >= len(t.entries) && len(t.entries) > 0 {
			t.cursor = len(t.entries) - 1
		}
		if len(t.entries) == 0 {
			t.cursor = 0
		}
		m.clampOffset()

	case "t":
		// Cycle sort mode: size -> name -> updated -> created -> size
		// Only in browse tab
		if m.activeTab != tabBrowse {
			return m, nil
		}
		switch m.sortBy {
		case sortSizeDesc:
			m.sortBy = sortNameAsc
		case sortNameAsc:
			m.sortBy = sortUpdatedDesc
		case sortUpdatedDesc:
			m.sortBy = sortCreatedDesc
		case sortCreatedDesc:
			m.sortBy = sortSizeDesc
		}
		// Remember cursor position
		var cursorPath string
		if t.cursor < len(t.entries) {
			cursorPath = t.entries[t.cursor].Path
		}
		sortEntries(t.allEntries, m.sortBy)
		m.applyFilter()
		// Restore cursor
		if cursorPath != "" {
			for i, e := range t.entries {
				if e.Path == cursorPath {
					t.cursor = i
					break
				}
			}
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "tab":
		// Toggle trash / permanent delete mode
		if m.deleteType == deleteTrash {
			m.deleteType = deletePermanent
		} else {
			m.deleteType = deleteTrash
		}

	case "e":
		// Dry-run preview
		if len(t.selected) == 0 && t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent {
			t.selected[t.entries[t.cursor].Path] = true
		}
		if len(t.selected) > 0 {
			m.mode = modeDryRun
		}

	case "?":
		m.mode = modeHelp
	}

	return m, nil
}

func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		t := m.tab()
		paths := make([]string, 0, len(t.selected))
		for p := range t.selected {
			paths = append(paths, p)
		}
		useTrash := m.deleteType == deleteTrash
		return m, func() tea.Msg {
			var deleted []string
			for _, p := range paths {
				var err error
				if useTrash {
					err = moveToTrash(p)
				} else {
					err = os.RemoveAll(p)
				}
				if err != nil {
					return deleteErrMsg{err: err, deleted: deleted}
				}
				deleted = append(deleted, p)
			}
			return deleteDoneMsg{deleted: deleted}
		}

	case "n", "esc":
		m.mode = modeNormal
	}

	return m, nil
}

func (m model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := m.tab()
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
	case "esc":
		t.filterText = ""
		m.applyFilter()
		t.cursor = 0
		t.offset = 0
		m.mode = modeNormal
	case "backspace":
		if len(t.filterText) > 0 {
			_, size := utf8.DecodeLastRuneInString(t.filterText)
			t.filterText = t.filterText[:len(t.filterText)-size]
			m.applyFilter()
			if t.cursor >= len(t.entries) && len(t.entries) > 0 {
				t.cursor = len(t.entries) - 1
			}
			m.clampOffset()
		}
	default:
		r := msg.Runes
		if len(r) > 0 {
			t.filterText += string(r)
			m.applyFilter()
			if t.cursor >= len(t.entries) && len(t.entries) > 0 {
				t.cursor = len(t.entries) - 1
			}
			m.clampOffset()
		}
	}
	return m, nil
}

func (m model) handleDryRunKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "e", "q":
		m.mode = modeNormal
	}
	return m, nil
}
