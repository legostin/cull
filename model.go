package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeNormal mode = iota
	modeConfirm
	modeFilter
)

// deleteDoneMsg is sent after deletion completes.
type deleteDoneMsg struct {
	deleted []string
}

// deleteErrMsg is sent when deletion fails.
type deleteErrMsg struct {
	err     error
	deleted []string // paths that were successfully deleted before the error
}

type model struct {
	path       string
	allEntries []Entry // full unfiltered list
	entries    []Entry // filtered view (what cursor navigates)
	cursor     int
	selected   map[string]bool // paths of selected entries
	lastSelect int             // last toggled index for range-select
	mode       mode
	width      int
	height     int
	errMsg     string

	// Filter state
	filterText string

	// Progressive scanning state
	scanning    bool
	scanQueue   []string
	scanningDir string
	dirsTotal   int
	dirsDone    int

	// Cache: directory path -> scanned entries
	cache map[string][]Entry
}

func newModel(path string) model {
	return model{
		path:       path,
		selected:   make(map[string]bool),
		lastSelect: -1,
		scanning:   true,
		cache:      make(map[string][]Entry),
	}
}

// applyFilter rebuilds entries from allEntries based on filterText.
func (m *model) applyFilter() {
	if m.filterText == "" {
		m.entries = m.allEntries
		return
	}
	lower := strings.ToLower(m.filterText)
	filtered := make([]Entry, 0, len(m.allEntries))
	for _, e := range m.allEntries {
		if e.IsParent || strings.Contains(strings.ToLower(e.Name), lower) {
			filtered = append(filtered, e)
		}
	}
	m.entries = filtered
}

func (m model) Init() tea.Cmd {
	return quickScanCmd(m.path)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case quickScanDoneMsg:
		m.allEntries = msg.entries
		m.path = msg.path
		m.cursor = 0
		m.errMsg = ""
		m.applyFilter()
		m.cache[msg.path] = m.allEntries
		return m.startSizingUnsized()

	case dirSizeStartMsg:
		m.scanningDir = filepath.Base(msg.path)
		return m, nil

	case dirSizeResultMsg:
		// Cache the sub-entries for the sized directory
		if msg.subEntries != nil {
			m.cache[msg.path] = msg.subEntries
		}

		// Check if this result is for current directory's entries (not stale)
		found := false
		for _, e := range m.allEntries {
			if e.Path == msg.path {
				found = true
				break
			}
		}
		if !found {
			return m, nil
		}

		m.updateEntrySize(msg.path, msg.size)
		m.cache[m.path] = m.allEntries
		m.dirsDone++

		if len(m.scanQueue) > 0 {
			next := m.scanQueue[0]
			m.scanQueue = m.scanQueue[1:]
			return m, computeDirSizeCmd(next)
		}
		return m, func() tea.Msg { return scanCompleteMsg{} }

	case scanCompleteMsg:
		m.scanning = false
		m.scanningDir = ""
		return m, nil

	case scanErrMsg:
		m.errMsg = msg.err.Error()
		m.scanning = false
		return m, nil

	case deleteDoneMsg:
		m.removeDeleted(msg.deleted)
		m.mode = modeNormal
		return m, nil

	case deleteErrMsg:
		m.removeDeleted(msg.deleted)
		m.errMsg = msg.err.Error()
		m.mode = modeNormal
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// startSizingUnsized kicks off background sizing for any unsized directory entries.
func (m model) startSizingUnsized() (tea.Model, tea.Cmd) {
	m.scanQueue = nil
	for _, e := range m.allEntries {
		if e.IsDir && !e.IsParent && !e.Sized {
			m.scanQueue = append(m.scanQueue, e.Path)
		}
	}
	m.dirsTotal = len(m.scanQueue)
	m.dirsDone = 0

	if len(m.scanQueue) > 0 {
		m.scanning = true
		m.scanningDir = ""
		next := m.scanQueue[0]
		m.scanQueue = m.scanQueue[1:]
		return m, computeDirSizeCmd(next)
	}
	m.scanning = false
	m.scanningDir = ""
	return m, nil
}

// updateEntrySize updates a directory entry's size and re-sorts with stable cursor.
func (m *model) updateEntrySize(path string, size int64) {
	// Remember what the cursor points to
	var cursorPath string
	if m.cursor < len(m.entries) {
		cursorPath = m.entries[m.cursor].Path
	}

	// Update the size in allEntries
	for i := range m.allEntries {
		if m.allEntries[i].Path == path {
			m.allEntries[i].Size = size
			m.allEntries[i].Sized = true
			break
		}
	}

	// Re-sort allEntries: find where non-parent entries start
	start := 0
	if len(m.allEntries) > 0 && m.allEntries[0].IsParent {
		start = 1
	}
	sub := m.allEntries[start:]
	sort.Slice(sub, func(i, j int) bool {
		return sub[i].Size > sub[j].Size
	})

	// Reapply filter to rebuild entries from sorted allEntries
	m.applyFilter()

	// Restore cursor to the same entry
	if cursorPath != "" {
		for i, e := range m.entries {
			if e.Path == cursorPath {
				m.cursor = i
				break
			}
		}
	}
}

// removeDeleted removes deleted paths from entries and selection, fixes cursor.
func (m *model) removeDeleted(paths []string) {
	deletedSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		deletedSet[p] = true
		delete(m.selected, p)
	}

	filtered := make([]Entry, 0, len(m.allEntries))
	for _, e := range m.allEntries {
		if !deletedSet[e.Path] {
			filtered = append(filtered, e)
		}
	}
	m.allEntries = filtered
	m.applyFilter()
	m.lastSelect = -1

	// Invalidate cache for current dir since contents changed
	delete(m.cache, m.path)

	if m.cursor >= len(m.entries) && len(m.entries) > 0 {
		m.cursor = len(m.entries) - 1
	}
	if len(m.entries) == 0 {
		m.cursor = 0
	}
}

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

// navigateInto enters a subdirectory, using cache if available.
func (m model) navigateInto(path string) (tea.Model, tea.Cmd) {
	m.selected = make(map[string]bool)
	m.lastSelect = -1
	m.filterText = ""

	// Save current entries to cache before leaving
	m.cache[m.path] = m.allEntries

	if cached, ok := m.cache[path]; ok {
		m.path = path
		m.allEntries = cached
		m.applyFilter()
		m.cursor = 0
		m.errMsg = ""
		return m.startSizingUnsized()
	}

	// No cache — quick scan
	m.scanning = true
	m.scanQueue = nil
	m.scanningDir = ""
	m.dirsTotal = 0
	m.dirsDone = 0
	return m, quickScanCmd(path)
}

// navigateUp goes to the parent directory, using cache and only re-indexing the dir we left.
func (m model) navigateUp() (tea.Model, tea.Cmd) {
	parent := filepath.Dir(m.path)
	if parent == m.path {
		return m, nil
	}

	prevDir := m.path

	m.selected = make(map[string]bool)
	m.lastSelect = -1
	m.filterText = ""

	// Save current entries to cache before leaving
	m.cache[m.path] = m.allEntries

	if cached, ok := m.cache[parent]; ok {
		m.path = parent
		m.allEntries = cached
		m.errMsg = ""

		// Mark the directory we came from as unsized so it gets re-indexed
		for i := range m.allEntries {
			if m.allEntries[i].Path == prevDir {
				m.allEntries[i].Sized = false
				break
			}
		}
		m.applyFilter()

		// Position cursor on the directory we came from
		m.cursor = 0
		for i, e := range m.entries {
			if e.Path == prevDir {
				m.cursor = i
				break
			}
		}

		return m.startSizingUnsized()
	}

	// No cache — full quick scan
	m.scanning = true
	m.scanQueue = nil
	m.scanningDir = ""
	m.dirsTotal = 0
	m.dirsDone = 0
	return m, quickScanCmd(parent)
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

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder
	contentWidth := m.width - 2

	// Title
	title := titleStyle.Width(contentWidth).Render(" space-free")
	b.WriteString(title)
	b.WriteString("\n")

	// Path
	pathLine := pathStyle.Width(contentWidth).Render(" " + m.path)
	b.WriteString(pathLine)
	b.WriteString("\n")

	// Separator
	b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Error
	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("  Error: " + m.errMsg))
		b.WriteString("\n")
	}

	// Header
	header := headerStyle.Render(fmt.Sprintf("  %9s  %s", "SIZE", "NAME"))
	b.WriteString(header)
	b.WriteString("\n")

	// Calculate visible rows
	usedLines := 6 // title + path + sep + header + help + status
	if m.mode == modeConfirm {
		usedLines += 2
	}
	visibleRows := m.height - usedLines
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Scroll offset
	offset := 0
	if m.cursor >= visibleRows {
		offset = m.cursor - visibleRows + 1
	}

	// Entries
	end := offset + visibleRows
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := offset; i < end; i++ {
		e := m.entries[i]
		isCursor := i == m.cursor
		isSelected := m.selected[e.Path]

		// Parent entry (..)
		if e.IsParent {
			if isCursor {
				row := cursorStyle.Width(contentWidth).Render("  " + dirStyle.Render(".."))
				b.WriteString(row)
			} else {
				b.WriteString("  " + dirStyle.Render(".."))
			}
			b.WriteString("\n")
			continue
		}

		pending := !e.Sized
		sizeFormatted := formatSize(e.Size)

		name := e.Name
		if e.IsDir {
			name += "/"
		}

		markerText := "  "
		if isSelected {
			markerText = "● "
		}

		var row string
		if isCursor {
			plain := fmt.Sprintf("%s%9s  %s", markerText, sizeFormatted, name)
			row = cursorStyle.Width(contentWidth).Render(plain)
		} else {
			marker := markerText
			if isSelected {
				marker = selectedMarkerStyle.Render(markerText)
			}

			var size string
			if pending {
				size = sizePendingStyle.Render(sizeFormatted)
			} else {
				size = sizeStyle.Render(sizeFormatted)
			}

			if e.IsDir {
				row = fmt.Sprintf("%s%s  %s", marker, size, dirStyle.Render(name))
			} else {
				row = fmt.Sprintf("%s%s  %s", marker, size, name)
			}
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	// Pad remaining space
	for i := end - offset; i < visibleRows; i++ {
		b.WriteString("\n")
	}

	// Separator
	b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Confirm dialog / filter input / help line
	if m.mode == modeConfirm {
		totalSize := int64(0)
		count := 0
		for p := range m.selected {
			for _, e := range m.entries {
				if e.Path == p {
					totalSize += e.Size
					count++
					break
				}
			}
		}
		confirmText := fmt.Sprintf("  Delete %d items (%s)? [y/n]", count, formatSize(totalSize))
		b.WriteString(confirmStyle.Width(contentWidth).Render(confirmText))
		b.WriteString("\n")
	} else if m.mode == modeFilter {
		filterLine := fmt.Sprintf(" Filter: %s█", m.filterText)
		b.WriteString(filterStyle.Width(contentWidth).Render(filterLine))
		b.WriteString("\n")
	} else {
		help := fmt.Sprintf(" %s select %s range %s delete %s filter %s open %s back %s quit",
			helpKeyStyle.Render("<s>"),
			helpKeyStyle.Render("<S>"),
			helpKeyStyle.Render("<d>"),
			helpKeyStyle.Render("<f>"),
			helpKeyStyle.Render("<enter>"),
			helpKeyStyle.Render("<bksp>"),
			helpKeyStyle.Render("<q>"),
		)
		b.WriteString(helpDescStyle.Width(contentWidth).Render(help))
		b.WriteString("\n")
	}

	// Status line
	if m.scanning {
		status := fmt.Sprintf(" Scanning: %s/ · %d/%d dirs · %d items",
			m.scanningDir, m.dirsDone, m.dirsTotal, len(m.allEntries))
		b.WriteString(scanningStyle.Width(contentWidth).Render(status))
	} else {
		var totalSelected int64
		for p := range m.selected {
			for _, e := range m.entries {
				if e.Path == p {
					totalSelected += e.Size
					break
				}
			}
		}
		status := fmt.Sprintf(" %d items", len(m.entries))
		if m.filterText != "" && m.mode != modeFilter {
			status = fmt.Sprintf(" %d of %d items · filter: \"%s\"", len(m.entries), len(m.allEntries), m.filterText)
		}
		if len(m.selected) > 0 {
			status += fmt.Sprintf(" · %d selected · %s", len(m.selected), formatSize(totalSelected))
		}
		b.WriteString(statusBarStyle.Width(contentWidth).Render(status))
	}

	return b.String()
}
