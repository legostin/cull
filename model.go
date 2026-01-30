package main

import (
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
