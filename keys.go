package main

import (
	"os"
	"os/exec"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// ctrl+l forces a full repaint — external programs writing to the
	// terminal (kernel NFS warnings, wall) corrupt the alt screen.
	if msg.String() == "ctrl+l" {
		return m, tea.ClearScreen
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

	// Map-mode key handling on BROWSE: spatial movement + enter semantics.
	if m.activeTab == tabBrowse && m.browseMap {
		var dx, dy int
		switch msg.String() {
		case "h", "left":
			dx = -1
		case "l", "right":
			dx = 1
		case "j", "down":
			dy = 1
		case "k", "up":
			dy = -1
		case "S":
			return m, nil // no linear order on the map
		case "enter":
			rects := m.browseMapLayout()
			if len(rects) == 0 {
				return m, nil
			}
			if rects[m.mapCursorRect(rects)].Index == -1 {
				m.browseMap = false // +N more: back to the list
				return m, nil
			}
			// fall through to the normal enter handling below
		}
		if dx != 0 || dy != 0 {
			rects := m.browseMapLayout()
			if len(rects) == 0 {
				return m, nil
			}
			cur := m.mapCursorRect(rects)
			next := nearestRect(rects, cur, dx, dy)
			if idx := rects[next].Index; idx >= 0 {
				t.cursor = idx
			}
			return m, m.resetNameScroll()
		}
	}

	switch msg.String() {
	case "shift+tab":
		order := []tabID{tabBrowse, tabLargest, tabCaches, tabProjects}
		if m.trashRegistry != nil && len(m.trashRegistry.Records) > 0 {
			order = append(order, tabHistory)
		}
		idx := 0
		for i, id := range order {
			if id == m.activeTab {
				idx = i
				break
			}
		}
		return m.switchToTab(order[(idx+1)%len(order)])

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

	case "g", "home":
		t.cursor = 0
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "G", "end":
		if len(t.entries) > 0 {
			t.cursor = len(t.entries) - 1
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "pgdown":
		if n := len(t.entries); n > 0 {
			t.cursor += m.visibleRowCount()
			if t.cursor > n-1 {
				t.cursor = n - 1
			}
		}
		m.clampOffset()
		cmd := m.resetNameScroll()
		return m, cmd

	case "pgup":
		t.cursor -= m.visibleRowCount()
		if t.cursor < 0 {
			t.cursor = 0
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
		if m.readOnly {
			return m, nil
		}
		if t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent &&
			t.entries[t.cursor].Path != dockerEntryPath &&
			t.entries[t.cursor].Path != tmSnapEntryPath {
			p := t.entries[t.cursor].Path
			if t.selected[p] {
				delete(t.selected, p)
			} else {
				t.selected[p] = true
			}
			t.lastSelect = t.cursor
		}

	case "S":
		if m.readOnly {
			return m, nil
		}
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
				if !t.entries[i].IsParent && t.entries[i].Path != dockerEntryPath &&
					t.entries[i].Path != tmSnapEntryPath {
					t.selected[t.entries[i].Path] = true
				}
			}
			t.lastSelect = t.cursor
		}

	case "r":
		// Rescan on PROJECTS tab (allowed in read-only mode)
		if m.activeTab == tabProjects {
			if m.projectsScanning {
				return m, nil
			}
			m.projectsRoots = m.desiredProjectsRoots()
			m.tabs[tabProjects] = newTabState()
			m.projectsScanning = true
			m.projectsLoaded = false
			return m, loadProjectsCmd(m.projectsRoots)
		}
		// Restore: only on history tab, not read-only
		if m.readOnly || m.activeTab != tabHistory {
			return m, nil
		}
		if len(t.entries) == 0 {
			return m, nil
		}
		// Auto-select cursor item if nothing selected (skip stale)
		if len(t.selected) == 0 && t.cursor < len(t.entries) && !t.entries[t.cursor].Stale {
			t.selected[t.entries[t.cursor].Path] = true
		}
		if len(t.selected) == 0 {
			return m, nil
		}
		// Collect only non-stale selected paths
		staleSet := make(map[string]bool)
		for _, e := range t.entries {
			if e.Stale {
				staleSet[e.Path] = true
			}
		}
		paths := make([]string, 0, len(t.selected))
		for p := range t.selected {
			if !staleSet[p] {
				paths = append(paths, p)
			}
		}
		if len(paths) == 0 {
			return m, nil
		}
		reg := m.trashRegistry
		return m, func() tea.Msg {
			var restored []string
			for _, origPath := range paths {
				rec, ok := reg.LookupByOriginalPath(origPath)
				if !ok {
					continue
				}
				if err := restoreFromTrash(rec); err != nil {
					return restoreErrMsg{err: err, restored: restored}
				}
				restored = append(restored, origPath)
			}
			return restoreDoneMsg{restored: restored}
		}

	case "d":
		if m.readOnly {
			return m, nil
		}
		if len(t.entries) == 0 {
			return m, nil
		}

		// On history tab, "d" means purge from system trash (with confirmation)
		if m.activeTab == tabHistory {
			if len(t.selected) == 0 && t.cursor < len(t.entries) {
				t.selected[t.entries[t.cursor].Path] = true
			}
			if len(t.selected) == 0 {
				return m, nil
			}
			m.mode = modeConfirm
			return m, nil
		}

		// On caches tab, cursor on the Docker row triggers the prune flow.
		// Always confirmed, even with -y: prune is not restorable.
		if m.activeTab == tabCaches && len(t.selected) == 0 &&
			t.cursor < len(t.entries) && t.entries[t.cursor].Path == dockerEntryPath {
			m.mode = modeConfirm
			m.confirmDocker = true
			return m, nil
		}

		// Snapshot deletion is not restorable — always confirmed too.
		if m.activeTab == tabCaches && len(t.selected) == 0 &&
			t.cursor < len(t.entries) && t.entries[t.cursor].Path == tmSnapEntryPath {
			m.mode = modeConfirm
			m.confirmSnap = true
			return m, nil
		}

		// Normal delete flow (browse/largest/caches tabs)
		if len(t.selected) == 0 && t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent {
			t.selected[t.entries[t.cursor].Path] = true
		}
		if len(t.selected) == 0 {
			return m, nil
		}
		// -y never skips confirmation for system-managed paths.
		if m.skipConfirm && !anySystemPath(t.selected) {
			return m, m.buildDeleteCmd(t)
		}
		m.mode = modeConfirm

	case " ":
		if t.cursor < len(t.entries) && !t.entries[t.cursor].IsParent {
			cmd := exec.Command("qlmanage", "-p", t.entries[t.cursor].Path)
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Start()
		}

	case "m":
		if m.activeTab == tabBrowse {
			m.browseMap = !m.browseMap
			// Park the cursor on a mapped entry — ".." has no rectangle
			if m.browseMap && t.cursor < len(t.entries) && t.entries[t.cursor].IsParent {
				if rects := m.browseMapLayout(); len(rects) > 0 && rects[0].Index >= 0 {
					t.cursor = rects[0].Index
				}
			}
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
		// On PROJECTS tab: toggle size / idle sort
		if m.activeTab == tabProjects {
			m.projectsSortIdle = !m.projectsSortIdle
			var cursorPath string
			if t.cursor < len(t.entries) {
				cursorPath = t.entries[t.cursor].Path
			}
			sortProjectEntries(t.allEntries, m.projectsSortIdle)
			m.applyFilter()
			if cursorPath != "" {
				for i, e := range t.entries {
					if e.Path == cursorPath {
						t.cursor = i
						break
					}
				}
			}
			m.clampOffset()
			return m, m.resetNameScroll()
		}
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
		// Lazy-fill birth times on platforms where it's not gathered eagerly
		if m.sortBy == sortCreatedDesc && !eagerBirthTime {
			fillMissingBirthTimes(t.allEntries)
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
		if m.readOnly {
			return m, nil
		}
		// Toggle trash / permanent delete mode
		if m.deleteType == deleteTrash {
			m.deleteType = deletePermanent
		} else {
			m.deleteType = deleteTrash
		}

	case "e":
		if m.readOnly {
			return m, nil
		}
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

// switchToTab activates a tab with the same side effects as reaching it via
// shift+tab: history/caches reload, first projects scan. Shared by the
// keyboard cycle and tab bar clicks.
func (m model) switchToTab(id tabID) (tea.Model, tea.Cmd) {
	m.activeTab = id
	cmd := m.resetNameScroll()
	switch id {
	case tabHistory:
		return m, tea.Batch(cmd, m.loadTrashTab())
	case tabCaches:
		m.cachesNote = ""
		return m, tea.Batch(cmd, loadCachesCmd())
	case tabProjects:
		roots := m.desiredProjectsRoots()
		fresh := m.projectsLoaded || m.projectsScanning
		if fresh && joinRoots(m.projectsRoots) == joinRoots(roots) {
			break // results (or a running scan) already match the current dir
		}
		m.projectsRoots = roots
		m.tabs[tabProjects] = newTabState()
		m.projectMeta = nil
		m.projectsLoaded = false
		m.projectsScanning = true
		return m, tea.Batch(cmd, loadProjectsCmd(roots))
	}
	return m, cmd
}

// buildDeleteCmd creates a delete command that records TrashRecords when using trash mode.
func (m model) buildDeleteCmd(t *tabState) tea.Cmd {
	paths := make([]string, 0, len(t.selected))
	// Build a size/dir map from entries for trash record creation
	entryMap := make(map[string]Entry, len(t.entries))
	for _, e := range t.entries {
		entryMap[e.Path] = e
	}
	for p := range t.selected {
		// On the caches tab a selected row may cover several paths
		if m.activeTab == tabCaches {
			if group, ok := m.cachePathGroups[p]; ok {
				paths = append(paths, group...)
				continue
			}
		}
		paths = append(paths, p)
	}
	useTrash := m.deleteType == deleteTrash
	return func() tea.Msg {
		var deleted []string
		var trashRecords []TrashRecord
		for _, p := range paths {
			var err error
			if useTrash {
				var trashPath string
				trashPath, err = moveToTrash(p)
				if err == nil {
					e := entryMap[p]
					trashRecords = append(trashRecords, TrashRecord{
						OriginalPath: p,
						TrashPath:    trashPath,
						DeletedAt:    time.Now(),
						Size:         e.Size,
						IsDir:        e.IsDir,
					})
				}
			} else {
				err = os.RemoveAll(p)
			}
			if err != nil {
				return deleteErrMsg{err: err, deleted: deleted}
			}
			deleted = append(deleted, p)
		}
		return deleteDoneMsg{deleted: deleted, trashRecords: trashRecords}
	}
}

func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		t := m.tab()

		// Docker prune: the flag stays set until dockerPruneDoneMsg/ErrMsg
		// clear it, mirroring how file deletion keeps modeConfirm until done.
		if m.confirmDocker {
			return m, dockerPruneCmd()
		}
		if m.confirmSnap {
			return m, tmSnapDeleteCmd(tmSnapshotDates())
		}

		// On trash tab, confirm means purge
		if m.activeTab == tabHistory {
			paths := make([]string, 0, len(t.selected))
			for p := range t.selected {
				paths = append(paths, p)
			}
			reg := m.trashRegistry
			return m, func() tea.Msg {
				var purged []string
				for _, origPath := range paths {
					rec, ok := reg.LookupByOriginalPath(origPath)
					if !ok {
						continue
					}
					if err := purgeFromTrash(rec); err != nil {
						return purgeErrMsg{err: err, purged: purged}
					}
					purged = append(purged, origPath)
				}
				return purgeDoneMsg{purged: purged}
			}
		}

		return m, m.buildDeleteCmd(t)

	case "n", "esc":
		m.confirmDocker = false
		m.confirmSnap = false
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
