package main

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// nameScrollTickMsg fires periodically to advance the marquee.
type nameScrollTickMsg struct {
	gen int // generation counter to detect stale tick chains
}

// nameScrollTickCmd returns a command that fires after the marquee interval.
func nameScrollTickCmd(gen int) tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return nameScrollTickMsg{gen: gen}
	})
}

// scanAnimTickMsg fires periodically to advance the scanning gradient wave.
type scanAnimTickMsg struct {
	gen int
}

// scanAnimTickCmd returns a command that fires after the scan animation interval.
func scanAnimTickCmd(gen int) tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return scanAnimTickMsg{gen: gen}
	})
}

type mode int

const (
	modeNormal mode = iota
	modeConfirm
	modeFilter
	modeDryRun
	modeHelp
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
	deleteTrash deleteMode = iota
	deletePermanent
)

// deleteDoneMsg is sent after deletion completes.
type deleteDoneMsg struct {
	deleted      []string
	trashRecords []TrashRecord // populated only for trash-mode deletions
}

// deleteErrMsg is sent when deletion fails.
type deleteErrMsg struct {
	err     error
	deleted []string // paths that were successfully deleted before the error
}

// restoreDoneMsg is sent after restore completes.
type restoreDoneMsg struct {
	restored []string
}

// restoreErrMsg is sent when restore fails partway.
type restoreErrMsg struct {
	err      error
	restored []string
}

// purgeDoneMsg is sent after purge completes.
type purgeDoneMsg struct {
	purged []string
}

// purgeErrMsg is sent when purge fails partway.
type purgeErrMsg struct {
	err    error
	purged []string
}

// trashLoadedMsg is sent when the trash tab entries are loaded.
type trashLoadedMsg struct {
	entries []Entry
}

type tabID int

const (
	tabBrowse tabID = iota
	tabLargest
	tabCaches
	tabHistory
	tabProjects
)

type tabState struct {
	allEntries []Entry
	entries    []Entry
	cursor     int
	offset     int
	selected   map[string]bool
	lastSelect int
	filterText string
}

func newTabState() tabState {
	return tabState{
		selected:   make(map[string]bool),
		lastSelect: -1,
	}
}

// dirCacheEntry holds cached browse and deep scan results for a directory.
type dirCacheEntry struct {
	browseEntries  []Entry
	largestEntries []Entry // nil if deep scan never completed for this dir
	deepScanDone   bool
}

type model struct {
	path      string
	activeTab tabID
	tabs      [5]tabState
	mode      mode
	width     int
	height    int
	errMsg    string

	// Hidden files toggle
	showHidden bool

	// Sort mode
	sortBy sortMode

	// Delete mode (trash vs permanent)
	deleteType deleteMode

	// Disk free space (bytes) for the filesystem containing path
	diskFree uint64

	// Deep scan state
	deepScanning bool
	deepScanDone bool
	deepScanCh   chan deepScanMsg
	deepScanDirs map[string]bool // set of first-level dir paths currently being scanned
	topN         int

	// Path interner
	interner *PathInterner

	// Cache: directory path -> scanned entries
	cache map[string]dirCacheEntry

	// Name column marquee scroll state
	nameScroll     int    // tick counter for marquee animation
	nameScrollPath string // path of entry being scrolled (reset on cursor change)
	nameScrollGen  int    // generation counter; incremented by resetNameScroll

	// Scan animation state (gradient wave on scanning row)
	scanAnimPhase int
	scanAnimGen   int

	// Multi-root support
	rootPaths     []string
	isVirtualRoot bool

	// Trash registry
	trashRegistry *TrashRegistry

	// CACHES tab state
	cachePathGroups map[string][]string // Entry.Path -> all existing paths of that cache
	confirmDocker   bool                // confirm dialog is for docker prune
	cachesNote      string              // status-bar note, e.g. reclaimed space after prune

	// PROJECTS tab state
	projectMeta      map[string]projectArtifact // Entry.Path -> artifact meta
	projectsScanning bool
	projectsLoaded   bool
	projectsSortIdle bool // t toggles size (false) / idle (true) sort
	launchRoots      []string

	// CLI flags
	readOnly    bool
	skipConfirm bool
}

// tab returns a pointer to the current tab's state.
func (m *model) tab() *tabState {
	return &m.tabs[m.activeTab]
}

func newModel(path string, topN int, readOnly, skipConfirm bool) model {
	reg, _ := loadTrashRegistry()
	m := model{
		path:          path,
		showHidden:    true,
		sortBy:        sortSizeDesc,
		deleteType:    deleteTrash,
		cache:         make(map[string]dirCacheEntry),
		topN:          topN,
		interner:      NewPathInterner(),
		trashRegistry: reg,
		readOnly:      readOnly,
		skipConfirm:   skipConfirm,
		launchRoots:   []string{path},
	}
	for i := range m.tabs {
		m.tabs[i] = newTabState()
	}
	return m
}

func newMultiRootModel(paths []string, topN int, readOnly, skipConfirm bool) model {
	reg, _ := loadTrashRegistry()
	m := model{
		path:          "/ (multiple roots)",
		showHidden:    true,
		sortBy:        sortSizeDesc,
		deleteType:    deleteTrash,
		cache:         make(map[string]dirCacheEntry),
		rootPaths:     paths,
		isVirtualRoot: true,
		topN:          topN,
		interner:      NewPathInterner(),
		trashRegistry: reg,
		readOnly:      readOnly,
		skipConfirm:   skipConfirm,
		launchRoots:   paths,
	}
	for i := range m.tabs {
		m.tabs[i] = newTabState()
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
	m.tabs[tabBrowse].allEntries = entries
	m.tabs[tabBrowse].entries = entries
	return m
}

// internEntries populates DirID on each entry using the model's path interner.
func (m *model) internEntries(entries []Entry) {
	for i := range entries {
		if entries[i].Path != "" {
			dir := filepath.Dir(entries[i].Path)
			entries[i].DirID = m.interner.Intern(dir)
		}
	}
}

// applyFilter rebuilds entries for the current tab.
func (m *model) applyFilter() {
	m.applyFilterForTab(m.activeTab)
}

// applyFilterForTab rebuilds entries from allEntries based on filterText and showHidden for a specific tab.
func (m *model) applyFilterForTab(t tabID) {
	tab := &m.tabs[t]
	lower := strings.ToLower(tab.filterText)
	filtered := make([]Entry, 0, len(tab.allEntries))
	for _, e := range tab.allEntries {
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
		if tab.filterText != "" && !strings.Contains(strings.ToLower(e.Name), lower) {
			continue
		}
		filtered = append(filtered, e)
	}
	tab.entries = filtered
}

// clampOffset adjusts offset so the cursor stays within the visible viewport.
func (m *model) clampOffset() {
	visibleRows := m.height - 11 // logo(4) + path + tabbar + sep + header + sep + help + status
	if m.mode == modeConfirm {
		visibleRows -= 2
	}
	if visibleRows < 1 {
		visibleRows = 1
	}
	t := m.tab()
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+visibleRows {
		t.offset = t.cursor - visibleRows + 1
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
		bt := &m.tabs[tabBrowse]
		bt.allEntries = msg.entries
		m.internEntries(bt.allEntries)
		m.path = msg.path
		bt.cursor = 0
		bt.offset = 0
		m.errMsg = ""
		m.diskFree = diskFreeSpace(msg.path)
		sortEntries(bt.allEntries, m.sortBy)
		m.applyFilterForTab(tabBrowse)
		// Update cache, preserving existing deep scan results
		ce := m.cache[msg.path]
		ce.browseEntries = bt.allEntries
		m.cache[msg.path] = ce
		scrollCmd := m.resetNameScroll()

		// Check if deep scan results are already cached
		if ce.deepScanDone {
			lt := &m.tabs[tabLargest]
			lt.allEntries = ce.largestEntries
			m.internEntries(lt.allEntries)
			m.applyFilterForTab(tabLargest)
			m.deepScanDone = true
			m.deepScanning = false
			m.deepScanCh = nil
			return m, scrollCmd
		}

		// Reset deep scan results when navigating
		m.deepScanDone = false
		m.tabs[tabLargest] = newTabState()
		// Start unified deep scan
		var deepCmd, animCmd tea.Cmd
		if !m.isVirtualRoot {
			unsizedDirs := m.collectUnsizedDirs()
			m.deepScanning = true
			m.deepScanCh = startDeepScan(msg.path, m.topN, unsizedDirs)
			deepCmd = pollDeepScanCmd(msg.path, m.deepScanCh)
			animCmd = m.startScanAnim()
		}
		return m, tea.Batch(scrollCmd, deepCmd, animCmd)

	case scanErrMsg:
		m.errMsg = msg.err.Error()
		return m, nil

	case deepScanMsg:
		if msg.rootPath != m.path {
			return m, nil // stale result, discard
		}
		// Update LARGEST tab
		lt := &m.tabs[tabLargest]
		lt.allEntries = msg.entries
		m.internEntries(lt.allEntries)
		m.applyFilterForTab(tabLargest)

		// Update BROWSE tab dir sizes
		m.updateDirSizes(msg.dirSizes)

		// Update set of actively scanning directories
		m.deepScanDirs = msg.scanningDirs

		if msg.done {
			m.deepScanning = false
			m.deepScanDone = true
			m.deepScanCh = nil
			m.deepScanDirs = nil
			// Save deep scan results to cache
			ce := m.cache[m.path]
			ce.largestEntries = lt.allEntries
			ce.deepScanDone = true
			m.cache[m.path] = ce
			return m, nil
		}
		return m, pollDeepScanCmd(msg.rootPath, m.deepScanCh)

	case deleteDoneMsg:
		m.removeDeleted(msg.deleted)
		m.diskFree = diskFreeSpace(m.path)
		m.mode = modeNormal
		if len(msg.trashRecords) > 0 && m.trashRegistry != nil {
			_ = m.trashRegistry.AddAll(msg.trashRecords)
		}
		return m, nil

	case deleteErrMsg:
		m.removeDeleted(msg.deleted)
		m.diskFree = diskFreeSpace(m.path)
		m.errMsg = msg.err.Error()
		m.mode = modeNormal
		return m, nil

	case trashLoadedMsg:
		tt := &m.tabs[tabHistory]
		tt.allEntries = msg.entries
		tt.entries = msg.entries
		tt.cursor = 0
		tt.offset = 0
		return m, nil

	case cachesLoadedMsg:
		ct := &m.tabs[tabCaches]
		*ct = newTabState()
		ct.allEntries = msg.entries
		m.applyFilterForTab(tabCaches)
		m.cachePathGroups = msg.pathGroups
		cmds := make([]tea.Cmd, 0, len(msg.entries))
		for _, e := range msg.entries {
			if e.Path == dockerEntryPath {
				cmds = append(cmds, dockerSizeCmd())
			} else {
				cmds = append(cmds, cacheSizeCmd(e.Path, msg.pathGroups[e.Path]))
			}
		}
		return m, tea.Batch(cmds...)

	case cacheSizeMsg:
		m.handleCacheSize(msg)
		return m, nil

	case projectsLoadedMsg:
		pt := &m.tabs[tabProjects]
		*pt = newTabState()
		pt.allEntries = msg.entries
		m.applyFilterForTab(tabProjects)
		m.projectMeta = msg.artifacts
		m.projectsScanning = false
		m.projectsLoaded = true
		cmds := make([]tea.Cmd, 0, len(msg.entries))
		for _, e := range msg.entries {
			cmds = append(cmds, projectSizeCmd(e.Path))
		}
		return m, tea.Batch(cmds...)

	case projectSizeMsg:
		m.handleProjectSize(msg)
		return m, nil

	case dockerPruneDoneMsg:
		m.mode = modeNormal
		m.confirmDocker = false
		m.cachesNote = "docker: reclaimed " + formatSize(msg.reclaimed)
		return m, dockerSizeCmd()

	case dockerPruneErrMsg:
		m.mode = modeNormal
		m.confirmDocker = false
		m.errMsg = msg.err.Error()
		return m, nil

	case restoreDoneMsg:
		m.removeFromTrashTab(msg.restored)
		if m.trashRegistry != nil {
			_ = m.trashRegistry.Remove(msg.restored)
		}
		m.mode = modeNormal
		// Bounce back if history tab is now empty
		if m.activeTab == tabHistory && len(m.trashRegistry.Records) == 0 {
			m.activeTab = tabBrowse
		}
		return m, nil

	case restoreErrMsg:
		if len(msg.restored) > 0 {
			m.removeFromTrashTab(msg.restored)
			if m.trashRegistry != nil {
				_ = m.trashRegistry.Remove(msg.restored)
			}
		}
		m.errMsg = msg.err.Error()
		m.mode = modeNormal
		return m, nil

	case purgeDoneMsg:
		m.removeFromTrashTab(msg.purged)
		if m.trashRegistry != nil {
			_ = m.trashRegistry.Remove(msg.purged)
		}
		m.mode = modeNormal
		if m.activeTab == tabHistory && len(m.trashRegistry.Records) == 0 {
			m.activeTab = tabBrowse
		}
		return m, nil

	case purgeErrMsg:
		if len(msg.purged) > 0 {
			m.removeFromTrashTab(msg.purged)
			if m.trashRegistry != nil {
				_ = m.trashRegistry.Remove(msg.purged)
			}
		}
		m.errMsg = msg.err.Error()
		m.mode = modeNormal
		return m, nil

	case nameScrollTickMsg:
		return m.handleNameScrollTick(msg)

	case scanAnimTickMsg:
		if msg.gen != m.scanAnimGen || !m.deepScanning {
			return m, nil
		}
		m.scanAnimPhase++
		return m, scanAnimTickCmd(m.scanAnimGen)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// startScanAnim resets and starts the scan animation tick chain.
func (m *model) startScanAnim() tea.Cmd {
	m.scanAnimPhase = 0
	m.scanAnimGen++
	return scanAnimTickCmd(m.scanAnimGen)
}

// entryDisplayName returns the name shown in the view for the given entry and tab.
func (m model) entryDisplayName(e Entry) string {
	name := e.Name
	if e.IsSymlink {
		name += " →"
	} else if e.IsDir {
		name += "/"
	}
	if m.activeTab == tabLargest && e.Path != "" {
		if rel, err := filepath.Rel(m.path, e.Path); err == nil {
			name = rel
		}
	}
	if m.activeTab == tabHistory && e.Path != "" {
		name = e.Path
	}
	if m.activeTab == tabCaches && e.Path != "" {
		if e.Path == dockerEntryPath {
			name = e.Name
		} else {
			name = e.Name + " · " + e.Path
		}
	}
	if m.activeTab == tabProjects && e.Path != "" {
		name = m.projectsDisplayName(e)
	}
	return name
}

// projectsDisplayName renders a PROJECTS row name: "project · artifact"
// plus a caution note for dist/vendor.
func (m model) projectsDisplayName(e Entry) string {
	meta, ok := m.projectMeta[e.Path]
	if !ok {
		return e.Name
	}
	name := e.Name + " · " + meta.Kind
	if meta.Caution {
		name += " · may be needed"
	}
	return name
}

// handleNameScrollTick advances the marquee animation for the cursor row name.
func (m model) handleNameScrollTick(msg nameScrollTickMsg) (tea.Model, tea.Cmd) {
	// Stale tick chain — let it die
	if msg.gen != m.nameScrollGen {
		return m, nil
	}

	t := m.tab()
	// Check the cursor entry still matches
	if t.cursor >= len(t.entries) {
		return m, nil
	}
	e := t.entries[t.cursor]
	if e.Path != m.nameScrollPath {
		return m, nil
	}

	name := m.entryDisplayName(e)

	nameWidth := m.nameColWidth()
	nameRunes := []rune(name)
	maxOffset := len(nameRunes) - nameWidth
	if maxOffset <= 0 {
		return m, nil // no scrolling needed
	}

	const startPause = 7 // ticks to pause at start (~1s)
	const endPause = 13  // ticks to pause at end (~2s at 150ms/tick)

	m.nameScroll++
	total := startPause + maxOffset + endPause
	if m.nameScroll >= total {
		m.nameScroll = 0
	}

	return m, nameScrollTickCmd(msg.gen)
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
	// marker(2) + bar + space + size(9) + space + pct(4) + gap(2) + name + gap(2) + created(10) + gap(2) + updated(10)
	fixedWidth := 2 + barWidth + 1 + 9 + 1 + 4 + 2 + 2 + 10 + 2 + 10
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
	m.nameScrollGen++
	t := m.tab()

	if t.cursor >= len(t.entries) {
		m.nameScrollPath = ""
		return nil
	}
	e := t.entries[t.cursor]
	m.nameScrollPath = e.Path

	name := m.entryDisplayName(e)
	nameWidth := m.nameColWidth()
	if len([]rune(name)) > nameWidth {
		return nameScrollTickCmd(m.nameScrollGen)
	}
	return nil
}

// cachedDirSize returns the total size of a directory from its cache entry.
// Returns (size, true) if the directory has a completed deep scan in cache.
func (m *model) cachedDirSize(path string) (int64, bool) {
	ce, ok := m.cache[path]
	if !ok || !ce.deepScanDone {
		return 0, false
	}
	var total int64
	for _, e := range ce.browseEntries {
		if !e.IsParent {
			total += e.Size
		}
	}
	return total, true
}

// collectUnsizedDirs returns paths of unsized directory entries in the BROWSE tab.
func (m *model) collectUnsizedDirs() []string {
	bt := &m.tabs[tabBrowse]
	var dirs []string
	for _, e := range bt.allEntries {
		if e.IsDir && !e.IsParent && !e.Sized {
			dirs = append(dirs, e.Path)
		}
	}
	return dirs
}

// updateDirSizes updates BROWSE tab directory sizes from the deep scan accumulator.
func (m *model) updateDirSizes(sizes map[string]int64) {
	if len(sizes) == 0 {
		return
	}

	bt := &m.tabs[tabBrowse]

	// Remember what the cursor points to
	var cursorPath string
	if bt.cursor < len(bt.entries) {
		cursorPath = bt.entries[bt.cursor].Path
	}

	changed := false
	for i := range bt.allEntries {
		e := &bt.allEntries[i]
		if e.IsDir && !e.IsParent {
			if sz, ok := sizes[e.Path]; ok {
				e.Size = sz
				e.Sized = true
				changed = true
			}
		}
	}

	if !changed {
		return
	}

	// Re-sort only when sorting by size (other sort keys are unaffected by size changes)
	if m.sortBy == sortSizeDesc {
		sortEntries(bt.allEntries, m.sortBy)
	}
	m.applyFilterForTab(tabBrowse)

	// Restore cursor to the same entry
	if cursorPath != "" {
		for i, e := range bt.entries {
			if e.Path == cursorPath {
				bt.cursor = i
				break
			}
		}
	}
	m.clampOffset()

	// Update cache
	ce := m.cache[m.path]
	ce.browseEntries = bt.allEntries
	m.cache[m.path] = ce
}

// removeDeleted removes deleted paths from entries and selection across all tabs, fixes cursor.
func (m *model) removeDeleted(paths []string) {
	deletedSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		deletedSet[p] = true
	}

	for ti := range m.tabs {
		t := &m.tabs[ti]
		for p := range deletedSet {
			delete(t.selected, p)
		}

		filtered := make([]Entry, 0, len(t.allEntries))
		for _, e := range t.allEntries {
			if !deletedSet[e.Path] {
				filtered = append(filtered, e)
			}
		}
		t.allEntries = filtered
		m.applyFilterForTab(tabID(ti))
		t.lastSelect = -1

		if t.cursor >= len(t.entries) && len(t.entries) > 0 {
			t.cursor = len(t.entries) - 1
		}
		if len(t.entries) == 0 {
			t.cursor = 0
		}
	}

	// Invalidate cache for current dir since contents changed
	delete(m.cache, m.path)

	m.clampOffset()
}

// navigateInto enters a subdirectory, using cache if available.
func (m model) navigateInto(path string) (tea.Model, tea.Cmd) {
	bt := &m.tabs[tabBrowse]
	bt.selected = make(map[string]bool)
	bt.lastSelect = -1
	bt.filterText = ""
	m.isVirtualRoot = false

	// Reset non-browse tabs
	m.deepScanDone = false
	m.deepScanning = false
	m.deepScanCh = nil
	m.tabs[tabLargest] = newTabState()

	// Save current entries to cache before leaving
	if m.path != "/ (multiple roots)" {
		ce := m.cache[m.path]
		ce.browseEntries = bt.allEntries
		m.cache[m.path] = ce
	}

	if ce, ok := m.cache[path]; ok {
		m.path = path
		bt.allEntries = ce.browseEntries
		m.internEntries(bt.allEntries)
		m.diskFree = diskFreeSpace(path)
		sortEntries(bt.allEntries, m.sortBy)
		m.applyFilterForTab(tabBrowse)
		bt.cursor = 0
		bt.offset = 0
		m.errMsg = ""

		// If deep scan was completed, restore LARGEST tab from cache
		if ce.deepScanDone {
			lt := &m.tabs[tabLargest]
			lt.allEntries = ce.largestEntries
			m.internEntries(lt.allEntries)
			m.applyFilterForTab(tabLargest)
			m.deepScanDone = true
			m.deepScanning = false
			return m, nil
		}

		// Otherwise start deep scan for unsized dirs only
		unsizedDirs := m.collectUnsizedDirs()
		m.deepScanning = true
		m.deepScanCh = startDeepScan(path, m.topN, unsizedDirs)
		animCmd := m.startScanAnim()
		return m, tea.Batch(pollDeepScanCmd(path, m.deepScanCh), animCmd)
	}

	// No cache — quick scan
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
	bt := &m.tabs[tabBrowse]

	bt.selected = make(map[string]bool)
	bt.lastSelect = -1
	bt.filterText = ""

	// Reset non-browse tabs
	m.deepScanDone = false
	m.deepScanning = false
	m.deepScanCh = nil
	m.tabs[tabLargest] = newTabState()

	// Save current entries to cache before leaving
	ce := m.cache[m.path]
	ce.browseEntries = bt.allEntries
	m.cache[m.path] = ce

	if ce, ok := m.cache[parent]; ok {
		m.path = parent
		bt.allEntries = ce.browseEntries
		m.diskFree = diskFreeSpace(parent)
		m.errMsg = ""

		// Try to compute prevDir size from its own cache instead of re-scanning
		if sz, ok := m.cachedDirSize(prevDir); ok {
			for i := range bt.allEntries {
				if bt.allEntries[i].Path == prevDir {
					bt.allEntries[i].Size = sz
					bt.allEntries[i].Sized = true
					break
				}
			}
		} else {
			// Cache invalidated (e.g. deletion) — mark unsized for re-scan
			for i := range bt.allEntries {
				if bt.allEntries[i].Path == prevDir {
					bt.allEntries[i].Sized = false
					break
				}
			}
		}
		sortEntries(bt.allEntries, m.sortBy)
		m.applyFilterForTab(tabBrowse)

		// Position cursor on the directory we came from
		bt.cursor = 0
		bt.offset = 0
		for i, e := range bt.entries {
			if e.Path == prevDir {
				bt.cursor = i
				break
			}
		}
		m.clampOffset()

		// If deep scan was completed, restore LARGEST tab and only re-scan if needed
		if ce.deepScanDone {
			lt := &m.tabs[tabLargest]
			lt.allEntries = ce.largestEntries
			m.internEntries(lt.allEntries)
			m.applyFilterForTab(tabLargest)
			m.deepScanDone = true

			unsizedDirs := m.collectUnsizedDirs()
			if len(unsizedDirs) > 0 {
				m.deepScanning = true
				m.deepScanCh = startDeepScan(parent, m.topN, unsizedDirs)
				animCmd := m.startScanAnim()
				return m, tea.Batch(pollDeepScanCmd(parent, m.deepScanCh), animCmd)
			}
			return m, nil
		}

		unsizedDirs := m.collectUnsizedDirs()
		m.deepScanning = true
		m.deepScanCh = startDeepScan(parent, m.topN, unsizedDirs)
		animCmd := m.startScanAnim()
		return m, tea.Batch(pollDeepScanCmd(parent, m.deepScanCh), animCmd)
	}

	// No cache — full quick scan
	return m, quickScanCmd(parent)
}

// navigateToVirtualRoot returns to the virtual multi-root view.
func (m model) navigateToVirtualRoot() model {
	bt := &m.tabs[tabBrowse]
	ce := m.cache[m.path]
	ce.browseEntries = bt.allEntries
	m.cache[m.path] = ce
	bt.selected = make(map[string]bool)
	bt.lastSelect = -1
	bt.filterText = ""
	m.isVirtualRoot = true
	m.path = "/ (multiple roots)"
	m.deepScanning = false
	m.deepScanDone = false
	m.deepScanCh = nil
	m.deepScanDirs = nil

	// Reset non-browse tabs
	m.tabs[tabLargest] = newTabState()

	entries := make([]Entry, 0, len(m.rootPaths))
	for _, p := range m.rootPaths {
		entries = append(entries, Entry{
			Name:  p,
			Path:  p,
			IsDir: true,
		})
	}
	bt.allEntries = entries
	m.applyFilterForTab(tabBrowse)
	bt.cursor = 0
	bt.offset = 0
	return m
}

// loadTrashTab returns a command that loads registry entries into the trash tab.
func (m *model) loadTrashTab() tea.Cmd {
	reg := m.trashRegistry
	if reg == nil {
		return nil
	}
	return func() tea.Msg {
		if n := reg.Cleanup(); n > 0 {
			_ = reg.Save()
		}
		entries := reg.ToEntries()
		// Sort by deletion date descending (most recent first)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ModTime.After(entries[j].ModTime)
		})
		return trashLoadedMsg{entries: entries}
	}
}

// handleCacheSize applies a cacheSizeMsg to the CACHES tab: sets the row size
// (or drops the row when ok=false), re-sorts by size and keeps the cursor on
// the same entry.
func (m *model) handleCacheSize(msg cacheSizeMsg) {
	ct := &m.tabs[tabCaches]

	var cursorPath string
	if ct.cursor < len(ct.entries) {
		cursorPath = ct.entries[ct.cursor].Path
	}

	if msg.ok {
		for i := range ct.allEntries {
			if ct.allEntries[i].Path == msg.path {
				ct.allEntries[i].Size = msg.size
				ct.allEntries[i].Sized = true
				break
			}
		}
	} else {
		filtered := make([]Entry, 0, len(ct.allEntries))
		for _, e := range ct.allEntries {
			if e.Path != msg.path {
				filtered = append(filtered, e)
			}
		}
		ct.allEntries = filtered
		delete(ct.selected, msg.path)
	}

	sortEntries(ct.allEntries, sortSizeDesc)
	m.applyFilterForTab(tabCaches)

	if cursorPath != "" {
		for i, e := range ct.entries {
			if e.Path == cursorPath {
				ct.cursor = i
				break
			}
		}
	}
	if ct.cursor >= len(ct.entries) && len(ct.entries) > 0 {
		ct.cursor = len(ct.entries) - 1
	}
	if len(ct.entries) == 0 {
		ct.cursor = 0
	}
	if m.activeTab == tabCaches {
		m.clampOffset()
	}
}

// handleProjectSize applies a projectSizeMsg to the PROJECTS tab: sets the
// row size, re-sorts by the current projects sort and keeps the cursor on
// the same entry.
func (m *model) handleProjectSize(msg projectSizeMsg) {
	pt := &m.tabs[tabProjects]

	var cursorPath string
	if pt.cursor < len(pt.entries) {
		cursorPath = pt.entries[pt.cursor].Path
	}

	for i := range pt.allEntries {
		if pt.allEntries[i].Path == msg.path {
			pt.allEntries[i].Size = msg.size
			pt.allEntries[i].Sized = true
			break
		}
	}

	sortProjectEntries(pt.allEntries, m.projectsSortIdle)
	m.applyFilterForTab(tabProjects)

	if cursorPath != "" {
		for i, e := range pt.entries {
			if e.Path == cursorPath {
				pt.cursor = i
				break
			}
		}
	}
	if pt.cursor >= len(pt.entries) && len(pt.entries) > 0 {
		pt.cursor = len(pt.entries) - 1
	}
	if len(pt.entries) == 0 {
		pt.cursor = 0
	}
	if m.activeTab == tabProjects {
		m.clampOffset()
	}
}

// removeFromTrashTab removes paths from the trash tab's entries and selection.
func (m *model) removeFromTrashTab(paths []string) {
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	t := &m.tabs[tabHistory]
	for p := range pathSet {
		delete(t.selected, p)
	}

	filtered := make([]Entry, 0, len(t.allEntries))
	for _, e := range t.allEntries {
		if !pathSet[e.Path] {
			filtered = append(filtered, e)
		}
	}
	t.allEntries = filtered
	m.applyFilterForTab(tabHistory)
	t.lastSelect = -1

	if t.cursor >= len(t.entries) && len(t.entries) > 0 {
		t.cursor = len(t.entries) - 1
	}
	if len(t.entries) == 0 {
		t.cursor = 0
	}
	m.clampOffset()
}
