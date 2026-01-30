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

	// q quits in normal and confirm modes
	if msg.String() == "q" {
		return m, tea.Quit
	}

	if m.mode == modeConfirm {
		return m.handleConfirmKey(msg)
	}

	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "g":
		m.cursor = 0
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "G":
		if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "enter":
		if m.cursor < len(m.entries) && m.entries[m.cursor].IsParent {
			return m.navigateUp()
		}
		if m.cursor < len(m.entries) && m.entries[m.cursor].IsDir {
			return m.navigateInto(m.entries[m.cursor].Path)
		}

	case "backspace":
		return m.navigateUp()

	case "esc":
		return m.navigateUp()

	case "s":
		if m.cursor < len(m.entries) && !m.entries[m.cursor].IsParent {
			p := m.entries[m.cursor].Path
			if m.selected[p] {
				delete(m.selected, p)
			} else {
				m.selected[p] = true
			}
			m.lastSelect = m.cursor
		}

	case "S":
		if m.cursor < len(m.entries) {
			start := m.lastSelect
			if start < 0 {
				start = 0
			}
			lo, hi := start, m.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi; i++ {
				if !m.entries[i].IsParent {
					m.selected[m.entries[i].Path] = true
				}
			}
			m.lastSelect = m.cursor
		}

	case "d":
		if len(m.entries) == 0 {
			return m, nil
		}
		if len(m.selected) == 0 && m.cursor < len(m.entries) && !m.entries[m.cursor].IsParent {
			m.selected[m.entries[m.cursor].Path] = true
		}
		if len(m.selected) == 0 {
			return m, nil
		}
		m.mode = modeConfirm

	case " ":
		if m.cursor < len(m.entries) && !m.entries[m.cursor].IsParent {
			cmd := exec.Command("qlmanage", "-p", m.entries[m.cursor].Path)
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Start()
		}

	case "f":
		m.mode = modeFilter

	case "h":
		m.showHidden = !m.showHidden
		m.applyFilter()
		if m.cursor >= len(m.entries) && len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
		if len(m.entries) == 0 {
			m.cursor = 0
		}
		m.clampOffset()

	case "t":
		// Cycle sort mode: size -> name -> updated -> created -> size
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
		if m.cursor < len(m.entries) {
			cursorPath = m.entries[m.cursor].Path
		}
		sortEntries(m.allEntries, m.sortBy)
		m.applyFilter()
		// Restore cursor
		if cursorPath != "" {
			for i, e := range m.entries {
				if e.Path == cursorPath {
					m.cursor = i
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
		if len(m.selected) == 0 && m.cursor < len(m.entries) && !m.entries[m.cursor].IsParent {
			m.selected[m.entries[m.cursor].Path] = true
		}
		if len(m.selected) > 0 {
			m.mode = modeDryRun
		}
	}

	return m, nil
}

func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		paths := make([]string, 0, len(m.selected))
		for p := range m.selected {
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
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
	case "esc":
		m.filterText = ""
		m.applyFilter()
		m.cursor = 0
		m.offset = 0
		m.mode = modeNormal
	case "backspace":
		if len(m.filterText) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.filterText)
			m.filterText = m.filterText[:len(m.filterText)-size]
			m.applyFilter()
			if m.cursor >= len(m.entries) && len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
			m.clampOffset()
		}
	default:
		r := msg.Runes
		if len(r) > 0 {
			m.filterText += string(r)
			m.applyFilter()
			if m.cursor >= len(m.entries) && len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
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
