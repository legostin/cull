package main

import (
	"os"
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

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "g":
		m.cursor = 0

	case "G":
		if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}

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
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
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

	case "f":
		m.mode = modeFilter
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
		return m, func() tea.Msg {
			var deleted []string
			for _, p := range paths {
				if err := os.RemoveAll(p); err != nil {
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
		m.mode = modeNormal
	case "backspace":
		if len(m.filterText) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.filterText)
			m.filterText = m.filterText[:len(m.filterText)-size]
			m.applyFilter()
			if m.cursor >= len(m.entries) && len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
		}
	default:
		r := msg.Runes
		if len(r) > 0 {
			m.filterText += string(r)
			m.applyFilter()
			if m.cursor >= len(m.entries) && len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
		}
	}
	return m, nil
}
