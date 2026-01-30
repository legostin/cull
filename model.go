package main

import (
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// nameScrollTickMsg fires periodically to advance the marquee.
type nameScrollTickMsg struct{}

// nameScrollTickCmd returns a command that fires after the marquee interval.
func nameScrollTickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return nameScrollTickMsg{}
	})
}

type mode int

const (
	modeNormal mode = iota
	modeConfirm
	modeFilter
	modeDryRun
)

type sortMode int

const (
	sortSizeDesc sortMode = iota
	sortNameAsc
	sortUpdatedDesc
	sortCreatedDesc
)

type deleteMode int

const (
	deleteTrash     deleteMode = iota
	deletePermanent
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
	offset     int             // scroll offset for viewport
	selected   map[string]bool // paths of selected entries
	lastSelect int             // last toggled index for range-select
	mode       mode
	width      int
	height     int
	errMsg     string

	// Filter state
	filterText string

	// Hidden files toggle
	showHidden bool

	// Sort mode
	sortBy sortMode

	// Delete mode (trash vs permanent)
	deleteType deleteMode

	// Disk free space (bytes) for the filesystem containing path
	diskFree uint64

	// Progressive scanning state
	scanning    bool
	scanQueue   []string
	scanningDir string
	dirsTotal   int
	dirsDone    int

	// Cache: directory path -> scanned entries
	cache map[string][]Entry

	// Name column marquee scroll state
	nameScroll     int    // tick counter for marquee animation
	nameScrollPath string // path of entry being scrolled (reset on cursor change)

	// Multi-root support
	rootPaths     []string
	isVirtualRoot bool
}

func newModel(path string) model {
	return model{
		path:       path,
		selected:   make(map[string]bool),
		lastSelect: -1,
		scanning:   true,
		showHidden: true,
		sortBy:     sortSizeDesc,
		deleteType: deleteTrash,
		cache:      make(map[string][]Entry),
	}
}

func newMultiRootModel(paths []string) model {
	m := model{
		path:          "/ (multiple roots)",
		selected:      make(map[string]bool),
		lastSelect:    -1,
		showHidden:    true,
		sortBy:        sortSizeDesc,
		deleteType:    deleteTrash,
		cache:         make(map[string][]Entry),
		rootPaths:     paths,
		isVirtualRoot: true,
	}
	// Build virtual root entries
	entries := make([]Entry, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, Entry{
			Name:  p,
			Path:  p,
			IsDir: true,
		})
	}
	m.allEntries = entries
	m.entries = entries
	return m
}

// applyFilter rebuilds entries from allEntries based on filterText and showHidden.
func (m *model) applyFilter() {
	lower := strings.ToLower(m.filterText)
	filtered := make([]Entry, 0, len(m.allEntries))
	for _, e := range m.allEntries {
		// Always include parent entry
		if e.IsParent {
			filtered = append(filtered, e)
			continue
		}
		// Hidden files filter
		if !m.showHidden && strings.HasPrefix(e.Name, ".") {
			continue
		}
		// Text filter
		if m.filterText != "" && !strings.Contains(strings.ToLower(e.Name), lower) {
			continue
		}
		filtered = append(filtered, e)
	}
	m.entries = filtered
}

// clampOffset adjusts m.offset so the cursor stays within the visible viewport.
func (m *model) clampOffset() {
	visibleRows := m.height - 9
	if m.mode == modeConfirm {
		visibleRows -= 2
	}
	if visibleRows < 1 {
		visibleRows = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
}

func (m model) Init() tea.Cmd {
	if m.isVirtualRoot {
		return nil
	}
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
		m.offset = 0
		m.errMsg = ""
		m.diskFree = diskFreeSpace(msg.path)
		sortEntries(m.allEntries, m.sortBy)
		m.applyFilter()
		m.cache[msg.path] = m.allEntries
		scrollCmd := m.resetNameScroll()
		mdl, sizeCmd := m.startSizingUnsized()
		return mdl, tea.Batch(sizeCmd, scrollCmd)

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
		m.diskFree = diskFreeSpace(m.path)
		m.mode = modeNormal
		return m, nil

	case deleteErrMsg:
		m.removeDeleted(msg.deleted)
		m.diskFree = diskFreeSpace(m.path)
		m.errMsg = msg.err.Error()
		m.mode = modeNormal
		return m, nil

	case nameScrollTickMsg:
		return m.handleNameScrollTick()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleNameScrollTick advances the marquee animation for the cursor row name.
func (m model) handleNameScrollTick() (tea.Model, tea.Cmd) {
	// Check the cursor entry still matches
	if m.cursor >= len(m.entries) {
		return m, nil
	}
	e := m.entries[m.cursor]
	if e.Path != m.nameScrollPath {
		return m, nil
	}

	name := e.Name
	if e.IsDir {
		name += "/"
	}

	nameWidth := m.nameColWidth()
	nameRunes := []rune(name)
	maxOffset := len(nameRunes) - nameWidth
	if maxOffset <= 0 {
		return m, nil // no scrolling needed
	}

	const startPause = 7 // ticks to pause at start (~1s)
	const endPause = 7   // ticks to pause at end

	m.nameScroll++
	total := startPause + maxOffset + endPause
	if m.nameScroll >= total {
		m.nameScroll = 0
	}

	return m, nameScrollTickCmd()
}

// nameColWidth returns the name column width based on current terminal width.
func (m model) nameColWidth() int {
	contentWidth := m.width - 2
	barWidth := 10
	if contentWidth > 100 {
		barWidth = 15
	} else if contentWidth < 60 {
		barWidth = 5
	}
	// marker(2) + bar + space + size(9) + gap(2) + name + gap(2) + created(10) + gap(2) + updated(10)
	fixedWidth := 2 + barWidth + 1 + 9 + 2 + 2 + 10 + 2 + 10
	nameWidth := contentWidth - fixedWidth
	if nameWidth < 10 {
		nameWidth = 10
	}
	if nameWidth > 40 {
		nameWidth = 40
	}
	return nameWidth
}

// nameScrollOffset returns the current rune offset for marquee display.
func (m model) nameScrollOffset(nameRuneLen, nameWidth int) int {
	maxOffset := nameRuneLen - nameWidth
	if maxOffset <= 0 {
		return 0
	}
	const startPause = 7
	tick := m.nameScroll
	if tick < startPause {
		return 0
	}
	off := tick - startPause
	if off > maxOffset {
		return maxOffset
	}
	return off
}

// resetNameScroll resets marquee state and starts ticking if needed.
func (m *model) resetNameScroll() tea.Cmd {
	m.nameScroll = 0

	if m.cursor >= len(m.entries) {
		m.nameScrollPath = ""
		return nil
	}
	e := m.entries[m.cursor]
	m.nameScrollPath = e.Path

	name := e.Name
	if e.IsDir {
		name += "/"
	}
	nameWidth := m.nameColWidth()
	if len([]rune(name)) > nameWidth {
		return nameScrollTickCmd()
	}
	return nil
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

	// Re-sort allEntries using current sort mode
	sortEntries(m.allEntries, m.sortBy)

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
	m.clampOffset()
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
	m.clampOffset()
}

// navigateInto enters a subdirectory, using cache if available.
func (m model) navigateInto(path string) (tea.Model, tea.Cmd) {
	m.selected = make(map[string]bool)
	m.lastSelect = -1
	m.filterText = ""
	m.isVirtualRoot = false

	// Save current entries to cache before leaving
	if m.path != "/ (multiple roots)" {
		m.cache[m.path] = m.allEntries
	}

	if cached, ok := m.cache[path]; ok {
		m.path = path
		m.allEntries = cached
		m.diskFree = diskFreeSpace(path)
		sortEntries(m.allEntries, m.sortBy)
		m.applyFilter()
		m.cursor = 0
		m.offset = 0
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
	// If we have root paths and we're at one of them, go back to virtual root
	if len(m.rootPaths) > 0 {
		for _, rp := range m.rootPaths {
			if m.path == rp {
				return m.navigateToVirtualRoot(), nil
			}
		}
	}

	parent := filepath.Dir(m.path)
	if parent == m.path {
		// At filesystem root, if multi-root go to virtual root
		if len(m.rootPaths) > 0 {
			return m.navigateToVirtualRoot(), nil
		}
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
		m.diskFree = diskFreeSpace(parent)
		m.errMsg = ""

		// Mark the directory we came from as unsized so it gets re-indexed
		for i := range m.allEntries {
			if m.allEntries[i].Path == prevDir {
				m.allEntries[i].Sized = false
				break
			}
		}
		sortEntries(m.allEntries, m.sortBy)
		m.applyFilter()

		// Position cursor on the directory we came from
		m.cursor = 0
		m.offset = 0
		for i, e := range m.entries {
			if e.Path == prevDir {
				m.cursor = i
				break
			}
		}
		m.clampOffset()

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

// navigateToVirtualRoot returns to the virtual multi-root view.
func (m model) navigateToVirtualRoot() model {
	m.cache[m.path] = m.allEntries
	m.selected = make(map[string]bool)
	m.lastSelect = -1
	m.filterText = ""
	m.isVirtualRoot = true
	m.path = "/ (multiple roots)"
	m.scanning = false
	m.scanQueue = nil
	m.scanningDir = ""

	entries := make([]Entry, 0, len(m.rootPaths))
	for _, p := range m.rootPaths {
		entries = append(entries, Entry{
			Name:  p,
			Path:  p,
			IsDir: true,
		})
	}
	m.allEntries = entries
	m.applyFilter()
	m.cursor = 0
	m.offset = 0
	return m
}
