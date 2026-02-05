package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// gradientWave applies a moving color gradient to text. Non-space characters
// are colored using the scanGradient palette, with the wave shifted by phase.
func gradientWave(text string, phase int) string {
	runes := []rune(text)
	wl := len(scanGradient)
	if wl == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text) * 12) // rough estimate for ANSI overhead
	for i, r := range runes {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		idx := ((i + phase) % wl + wl) % wl
		b.WriteString(scanGradientStyles[idx].Render(string(r)))
	}
	return b.String()
}

// formatDate returns "2025-01-15" for a non-zero time, or 10 spaces for zero.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "          "
	}
	return t.Format("2006-01-02")
}

// truncateName truncates a name to fit within width runes, adding "…" if needed.
func truncateName(name string, width int) string {
	runes := []rune(name)
	if len(runes) <= width {
		return name
	}
	return string(runes[:width-1]) + "…"
}

// marqueeSlice returns a width-sized window of name at the given rune offset.
func marqueeSlice(name string, width, offset int) string {
	runes := []rune(name)
	if len(runes) <= width {
		return name
	}
	end := offset + width
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[offset:end])
}

// formatSize returns a human-readable size string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// sizeHeaderLabel returns the SIZE column header, highlighted when sorting by size.
func sizeHeaderLabel(s sortMode) string {
	if s == sortSizeDesc {
		return "SIZE▼"
	}
	return "SIZE"
}

// createdHeaderLabel returns the CREATED column header, highlighted when sorting by created.
func createdHeaderLabel(s sortMode) string {
	if s == sortCreatedDesc {
		return "CREATED▼"
	}
	return "CREATED"
}

// updatedHeaderLabel returns the UPDATED column header, highlighted when sorting by updated.
func updatedHeaderLabel(s sortMode) string {
	if s == sortUpdatedDesc {
		return "UPDATED▼"
	}
	return "UPDATED"
}

// nameHeaderLabel returns the NAME column header, highlighted when sorting by name.
func nameHeaderLabel(s sortMode) string {
	if s == sortNameAsc {
		return "NAME▲"
	}
	return "NAME"
}

// formatPercent returns a right-aligned percentage string (4 chars), e.g. " 42%".
func formatPercent(size, total int64) string {
	if total <= 0 || size <= 0 {
		return "    "
	}
	pct := float64(size) / float64(total) * 100
	if pct >= 100 {
		return "100%"
	}
	if pct >= 10 {
		return fmt.Sprintf("%2.0f%%", pct)
	}
	if pct >= 1 {
		return fmt.Sprintf(" %1.0f%%", pct)
	}
	// < 1% but > 0
	return " <1%"
}

// sizeStyleFor returns the appropriate size style based on file size.
func sizeStyleFor(bytes int64) lipgloss.Style {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)
	switch {
	case bytes >= 10*GB:
		return sizeStyle10GB
	case bytes >= GB:
		return sizeStyle1GB
	case bytes >= 100*MB:
		return sizeStyle100MB
	case bytes >= 10*MB:
		return sizeStyle10MB
	default:
		return sizeStyle
	}
}

// proportionBar renders a right-aligned bar of width barWidth proportional to size/maxSize.
func proportionBar(size, maxSize int64, barWidth int) string {
	if maxSize <= 0 || size <= 0 || barWidth <= 0 {
		return strings.Repeat(" ", barWidth)
	}
	filled := int(float64(size) / float64(maxSize) * float64(barWidth))
	if filled < 1 && size > 0 {
		filled = 1
	}
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat(" ", barWidth-filled) + barFilledStyle.Render(strings.Repeat("▐", filled))
}

// proportionBarPlain renders a right-aligned plain bar (no ANSI styling) for use in cursor rows.
func proportionBarPlain(size, maxSize int64, barWidth int) string {
	if maxSize <= 0 || size <= 0 || barWidth <= 0 {
		return strings.Repeat(" ", barWidth)
	}
	filled := int(float64(size) / float64(maxSize) * float64(barWidth))
	if filled < 1 && size > 0 {
		filled = 1
	}
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat(" ", barWidth-filled) + strings.Repeat("▐", filled)
}

// renderTabBar renders the tab bar between path and separator.
func (m model) renderTabBar(contentWidth int) string {
	type tabInfo struct {
		id   tabID
		name string
	}
	tabs := []tabInfo{
		{tabBrowse, "BROWSE"},
		{tabLargest, "LARGEST"},
	}
	if m.trashRegistry != nil && len(m.trashRegistry.Records) > 0 {
		tabs = append(tabs, tabInfo{tabHistory, "HISTORY"})
	}

	var parts []string
	for _, ti := range tabs {
		label := ti.name
		// Add scanning indicator for LARGEST tab while deep scan runs
		if ti.id == tabLargest && ti.id != m.activeTab && m.deepScanning {
			label += " ◐"
		}
		// Show item count for HISTORY tab
		if ti.id == tabHistory {
			label += fmt.Sprintf(" (%d)", len(m.trashRegistry.Records))
		}

		if ti.id == m.activeTab {
			parts = append(parts, tabActiveStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, tabInactiveStyle.Render(" "+label+" "))
		}
	}

	return " " + strings.Join(parts, " ")
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(8192)
	contentWidth := m.width - 2

	// ASCII art logo + disk free
	logo := []string{
		`   ___ _   _ | | |`,
		`  / __| | | || | |`,
		` | (__| |_| || | |`,
		`  \___|\__,_||_|_|`,
	}
	freeText := ""
	if m.diskFree > 0 {
		freeText = fmt.Sprintf("%s free", formatSize(int64(m.diskFree)))
	}
	logoStyle := titleStyle
	if m.readOnly {
		logoStyle = titleStyleReadOnly
	} else if m.deleteType == deletePermanent {
		logoStyle = titleStyleDanger
	}
	for i, line := range logo {
		styled := logoStyle.Render(line)
		if i == 1 && freeText != "" {
			pad := contentWidth - lipgloss.Width(styled) - len(freeText)
			if pad < 1 {
				pad = 1
			}
			styled += strings.Repeat(" ", pad) + freeTextStyle.Render(freeText)
		}
		b.WriteString(styled)
		b.WriteString("\n")
	}

	// Path
	var pathPrefix string
	if m.readOnly {
		pathPrefix = readOnlyBadgeStyle.Render("READ-ONLY") + " "
	} else if m.deleteType == deletePermanent {
		pathPrefix = permDeleteBadgeStyle.Render("⚠ PERMANENT DELETE") + " "
	}
	pathLine := pathStyle.Width(contentWidth).Render(" " + pathPrefix + m.path)
	b.WriteString(pathLine)
	b.WriteString("\n")

	// Tab bar
	b.WriteString(m.renderTabBar(contentWidth))
	b.WriteString("\n")

	// Separator (computed once, reused)
	separator := separatorStyle.Render(strings.Repeat("─", contentWidth))
	b.WriteString(separator)
	b.WriteString("\n")

	// Error
	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("  Error: " + m.errMsg))
		b.WriteString("\n")
	}

	// Dry-run overlay
	if m.mode == modeDryRun {
		return m.viewDryRun(&b, contentWidth)
	}

	// Help overlay
	if m.mode == modeHelp {
		return m.viewHelp(&b, contentWidth)
	}

	t := m.tab()

	// Compute max size for proportion bar and total size for percentage
	var maxSize, totalSize int64
	for _, e := range t.entries {
		if !e.IsParent && e.Sized {
			totalSize += e.Size
			if e.Size > maxSize {
				maxSize = e.Size
			}
		}
	}

	// Bar width: adaptive, halved
	barWidth := 10
	if contentWidth > 100 {
		barWidth = 15
	} else if contentWidth < 60 {
		barWidth = 5
	}

	nameWidth := m.nameColWidth()

	// Header
	switch m.activeTab {
	case tabBrowse:
		header := headerStyle.Render(fmt.Sprintf("  %*s %9s %4s  %-*s  %-10s  %-10s",
			barWidth, "",
			sizeHeaderLabel(m.sortBy), "%",
			nameWidth, nameHeaderLabel(m.sortBy),
			createdHeaderLabel(m.sortBy),
			updatedHeaderLabel(m.sortBy)))
		b.WriteString(header)
	case tabLargest:
		header := headerStyle.Render(fmt.Sprintf("  %*s %9s %4s  %-*s  %-10s  %-10s",
			barWidth, "",
			"SIZE▼", "%",
			nameWidth, "NAME",
			"CREATED",
			"UPDATED"))
		b.WriteString(header)
	case tabHistory:
		header := headerStyle.Render(fmt.Sprintf("  %*s %9s %4s  %-*s  %-10s  %-10s",
			barWidth, "",
			"SIZE", "",
			nameWidth, "NAME",
			"",
			"DELETED"))
		b.WriteString(header)
	}
	b.WriteString("\n")

	// Calculate visible rows
	usedLines := 11 // logo(4) + path + tabbar + sep + header + sep + help + status
	if m.mode == modeConfirm {
		usedLines += 2
	}
	visibleRows := m.height - usedLines
	if visibleRows < 1 {
		visibleRows = 1
	}

	offset := t.offset

	// Entries
	end := offset + visibleRows
	if end > len(t.entries) {
		end = len(t.entries)
	}

	// Empty-state message for LARGEST tab during scan
	if m.activeTab == tabLargest && len(t.entries) == 0 && m.deepScanning {
		b.WriteString(scanningStyle.Render("  Walking directory tree…"))
		b.WriteString("\n")
	}

	for i := offset; i < end; i++ {
		e := t.entries[i]
		isCursor := i == t.cursor
		isSelected := t.selected[e.Path]

		// Parent entry (..)
		if e.IsParent {
			pad := strings.Repeat(" ", barWidth)
			if isCursor {
				plain := fmt.Sprintf("  %s %9s %4s  %-*s  %10s  %10s", pad, "", "", nameWidth, "..", "", "")
				row := cursorStyle.Width(contentWidth).Render(plain)
				b.WriteString(row)
			} else {
				b.WriteString(fmt.Sprintf("  %s %9s %4s  %-*s  %10s  %10s", pad, "", "", nameWidth, dirStyle.Render(".."), "", ""))
			}
			b.WriteString("\n")
			continue
		}

		pending := !e.Sized

		var sizeFormatted string
		if pending {
			sizeFormatted = "..."
		} else {
			sizeFormatted = formatSize(e.Size)
		}

		name := e.Name
		if e.IsSymlink {
			name += " →"
		} else if e.IsDir {
			name += "/"
		}

		isActiveScanning := m.deepScanning && !e.IsParent && e.IsDir && m.deepScanDirs[e.Path]
		isUnsized := !e.IsParent && e.IsDir && !e.Sized

		markerText := "  "
		if isSelected {
			markerText = "● "
		}

		// Build proportion bar
		var bar string
		if e.Sized {
			bar = proportionBar(e.Size, maxSize, barWidth)
		} else {
			bar = strings.Repeat(" ", barWidth)
		}

		createdStr := formatDate(e.CreateTime)
		updatedStr := formatDate(e.ModTime)

		displayName := name
		if m.activeTab == tabLargest && e.Path != "" {
			if rel, err := filepath.Rel(m.path, e.Path); err == nil {
				displayName = rel
			}
		}

		var row string
		if isCursor {
			dn := displayName
			if isActiveScanning {
				dn = dn + " ◐"
			}
			if e.Stale {
				dn = dn + " (gone)"
			}
			// Marquee: scroll long names under cursor
			nameRunes := []rune(dn)
			if len(nameRunes) > nameWidth {
				off := m.nameScrollOffset(len(nameRunes), nameWidth)
				dn = marqueeSlice(dn, nameWidth, off)
			}
			// Use plain bar (no ANSI) so cursorStyle controls fg/bg uniformly
			var plainBar string
			if e.Sized {
				plainBar = proportionBarPlain(e.Size, maxSize, barWidth)
			} else {
				plainBar = strings.Repeat(" ", barWidth)
			}
			pctStr := formatPercent(e.Size, totalSize)
			plain := fmt.Sprintf("%s%s %9s %4s  %-*s  %-10s  %-10s", markerText, plainBar, sizeFormatted, pctStr, nameWidth, dn, createdStr, updatedStr)
			row = cursorStyle.Width(contentWidth).Render(plain)
		} else if e.Stale {
			// Stale history entry — render entire row in dim gray
			emptyBar := strings.Repeat(" ", barWidth)
			truncName := truncateName(displayName+" (gone)", nameWidth)
			plain := fmt.Sprintf("%s%s %9s %4s  %-*s  %-10s  %-10s",
				markerText, emptyBar, sizeFormatted, "", nameWidth, truncName, createdStr, updatedStr)
			row = staleStyle.Render(plain)
		} else if isActiveScanning {
			// Gradient wave on bar + size + pct only; name/dates use normal styles
			var plainBar string
			if e.Sized {
				plainBar = proportionBarPlain(e.Size, maxSize, barWidth)
			} else {
				plainBar = strings.Repeat(" ", barWidth)
			}
			pctStr := formatPercent(e.Size, totalSize)
			animatedPart := gradientWave(
				fmt.Sprintf("%s %9s %4s", plainBar, sizeFormatted, pctStr),
				m.scanAnimPhase,
			)

			// Name — use standard styles during scan
			truncName := truncateName(displayName+" ◐", nameWidth)
			var styledName string
			if e.IsSymlink {
				styledName = symlinkStyle.Render(truncName)
			} else if e.IsDir {
				nameStyleLocal := dirStyle
				if isUnsized {
					nameStyleLocal = dirUnsizedStyle
				}
				styledName = nameStyleLocal.Render(truncName)
			} else {
				styledName = truncName
			}
			namePad := nameWidth - lipgloss.Width(styledName)
			if namePad < 0 {
				namePad = 0
			}

			created := dateStyle.Render(createdStr)
			updated := dateStyle.Render(updatedStr)
			marker := markerText
			if isSelected {
				marker = selectedMarkerStyle.Render(markerText)
			}

			row = fmt.Sprintf("%s%s  %s%s  %s  %s",
				marker, animatedPart, styledName, strings.Repeat(" ", namePad), created, updated)
		} else {
			marker := markerText
			if isSelected {
				marker = selectedMarkerStyle.Render(markerText)
			}

			sizeCol := sizeStyleFor(e.Size)
			var size string
			if pending {
				size = sizePendingStyle.Render(sizeFormatted)
			} else {
				size = sizeCol.Render(sizeFormatted)
			}

			var pct string
			if pending {
				pct = sizePendingStyle.Width(4).Render("")
			} else {
				pct = sizeCol.Width(4).Render(formatPercent(e.Size, totalSize))
			}

			created := dateStyle.Render(createdStr)
			updated := dateStyle.Render(updatedStr)

			// Truncate name for non-cursor rows
			truncName := truncateName(displayName, nameWidth)

			if e.IsSymlink {
				styledName := symlinkStyle.Render(truncName)
				namePad := nameWidth - lipgloss.Width(styledName)
				if namePad < 0 {
					namePad = 0
				}
				row = fmt.Sprintf("%s%s %s %s  %s%s  %s  %s", marker, bar, size, pct, styledName, strings.Repeat(" ", namePad), created, updated)
			} else if e.IsDir {
				nameStyleLocal := dirStyle
				if isUnsized {
					nameStyleLocal = dirUnsizedStyle
				}
				styledName := nameStyleLocal.Render(truncName)
				namePad := nameWidth - lipgloss.Width(styledName)
				if namePad < 0 {
					namePad = 0
				}
				row = fmt.Sprintf("%s%s %s %s  %s%s  %s  %s", marker, bar, size, pct, styledName, strings.Repeat(" ", namePad), created, updated)
			} else {
				row = fmt.Sprintf("%s%s %s %s  %-*s  %s  %s", marker, bar, size, pct, nameWidth, truncName, created, updated)
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
	b.WriteString(separator)
	b.WriteString("\n")

	// Confirm dialog / filter input / help line
	if m.mode == modeConfirm {
		totalSize := int64(0)
		count := 0
		sizeMap := make(map[string]int64, len(t.entries))
		for _, e := range t.entries {
			sizeMap[e.Path] = e.Size
		}
		for p := range t.selected {
			if sz, ok := sizeMap[p]; ok {
				totalSize += sz
				count++
			}
		}
		var confirmText string
		if m.activeTab == tabHistory {
			confirmText = fmt.Sprintf("  Purge %d items (%s) permanently? [y/n]", count, formatSize(totalSize))
		} else if m.deleteType == deleteTrash {
			confirmText = fmt.Sprintf("  Trash %d items (%s)? [y/n]", count, formatSize(totalSize))
		} else {
			confirmText = fmt.Sprintf("  PERMANENTLY delete %d items (%s)? [y/n]", count, formatSize(totalSize))
		}
		b.WriteString(confirmStyle.Width(contentWidth).Render(confirmText))
		b.WriteString("\n")
	} else if m.mode == modeFilter {
		filterLine := fmt.Sprintf(" Filter: %s█", t.filterText)
		b.WriteString(filterStyle.Width(contentWidth).Render(filterLine))
		b.WriteString("\n")
	} else {
		var help string
		if m.readOnly {
			help = fmt.Sprintf(" %s filter %s sort %s tabs %s help %s quit",
				helpKeyStyle.Render("<f>"),
				helpKeyStyle.Render("<t>"),
				helpKeyStyle.Render("<shift>-<tab>"),
				helpKeyStyle.Render("<?>"),
				helpKeyStyle.Render("<q>"),
			)
		} else if m.activeTab == tabHistory {
			help = fmt.Sprintf(" %s select %s restore %s purge %s filter %s tabs %s help %s quit",
				helpKeyStyle.Render("<s>"),
				helpKeyStyle.Render("<r>"),
				helpKeyStyle.Render("<d>"),
				helpKeyStyle.Render("<f>"),
				helpKeyStyle.Render("<shift>-<tab>"),
				helpKeyStyle.Render("<?>"),
				helpKeyStyle.Render("<q>"),
			)
		} else {
			help = fmt.Sprintf(" %s select %s delete %s filter %s sort %s preview %s del mode %s tabs %s help %s quit",
				helpKeyStyle.Render("<s>"),
				helpKeyStyle.Render("<d>"),
				helpKeyStyle.Render("<f>"),
				helpKeyStyle.Render("<t>"),
				helpKeyStyle.Render("<e>"),
				helpKeyStyle.Render("<tab>"),
				helpKeyStyle.Render("<shift>-<tab>"),
				helpKeyStyle.Render("<?>"),
				helpKeyStyle.Render("<q>"),
			)
		}
		b.WriteString(helpDescStyle.Width(contentWidth).Render(help))
		b.WriteString("\n")
	}

	// Status line
	statusParts := []string{}

	if m.deepScanning {
		if len(m.deepScanDirs) == 0 {
			statusParts = append(statusParts, "scanning: … ◐")
		} else {
			names := make([]string, 0, len(m.deepScanDirs))
			for p := range m.deepScanDirs {
				names = append(names, filepath.Base(p)+"/")
			}
			sort.Strings(names)
			label := strings.Join(names, ", ")
			maxLen := contentWidth - 30 // leave room for item count etc.
			if maxLen < 20 {
				maxLen = 20
			}
			if len(label) > maxLen {
				label = label[:maxLen-3] + "..."
			}
			statusParts = append(statusParts, fmt.Sprintf("scanning: %s ◐", label))
		}
	}

	if len(statusParts) > 0 {
		status := " " + strings.Join(statusParts, " · ") + fmt.Sprintf(" · %d items", len(t.entries))
		b.WriteString(scanningStyle.Width(contentWidth).Render(status))
	} else {
		var totalSelected int64
		if len(t.selected) > 0 {
			sizeMap := make(map[string]int64, len(t.entries))
			for _, e := range t.entries {
				sizeMap[e.Path] = e.Size
			}
			for p := range t.selected {
				if sz, ok := sizeMap[p]; ok {
					totalSelected += sz
				}
			}
		}
		status := fmt.Sprintf(" %d items", len(t.entries))
		if t.filterText != "" && m.mode != modeFilter {
			status = fmt.Sprintf(" %d of %d items · filter: \"%s\"", len(t.entries), len(t.allEntries), t.filterText)
		}
		if !m.readOnly && len(t.selected) > 0 {
			status += fmt.Sprintf(" · %d selected · %s", len(t.selected), formatSize(totalSelected))
		}
		// Mode indicators
		if !m.showHidden {
			status += " · hidden:off"
		}
		if !m.readOnly && m.deleteType == deletePermanent {
			status += " · PERM DELETE"
		}
		if m.readOnly {
			status += " · " + readOnlyStatusStyle.Render("READ-ONLY")
		}
		b.WriteString(statusBarStyle.Width(contentWidth).Render(status))
	}

	return b.String()
}

// viewDryRun renders the dry-run preview overlay.
func (m model) viewDryRun(b *strings.Builder, contentWidth int) string {
	t := m.tab()
	b.WriteString(dryRunHeaderStyle.Render("  Dry-run preview — items to be deleted:"))
	b.WriteString("\n")
	b.WriteString("\n")

	usedLines := 12 // logo(4) + path + tabbar + sep + error(maybe) + header + blank + help + status
	visibleRows := m.height - usedLines
	if visibleRows < 1 {
		visibleRows = 1
	}

	count := 0
	var totalSize int64
	sizeMap := make(map[string]int64, len(t.allEntries))
	for _, e := range t.allEntries {
		sizeMap[e.Path] = e.Size
	}
	for p := range t.selected {
		if sz, ok := sizeMap[p]; ok {
			if count < visibleRows {
				line := fmt.Sprintf("  %9s  %s", formatSize(sz), p)
				b.WriteString(line)
				b.WriteString("\n")
			}
			totalSize += sz
			count++
		}
	}

	if count > visibleRows {
		b.WriteString(fmt.Sprintf("  ... and %d more\n", count-visibleRows))
	}

	// Pad
	rendered := count
	if rendered > visibleRows {
		rendered = visibleRows + 1
	}
	for i := rendered; i < visibleRows; i++ {
		b.WriteString("\n")
	}

	// Separator
	b.WriteString(separatorStyle.Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Summary + help
	modeLabel := "trash"
	if m.deleteType == deletePermanent {
		modeLabel = "PERMANENT"
	}
	summary := fmt.Sprintf(" %d items · %s · mode: %s · %s back",
		count, formatSize(totalSize), modeLabel, helpKeyStyle.Render("<esc>"))
	b.WriteString(helpDescStyle.Width(contentWidth).Render(summary))
	b.WriteString("\n")

	// Status (empty)
	b.WriteString(statusBarStyle.Width(contentWidth).Render(" dry-run preview"))

	return b.String()
}

// viewHelp renders the full help overlay.
func (m model) viewHelp(b *strings.Builder, contentWidth int) string {
	lines := []string{
		"",
		"  NAVIGATION",
		"    j / ↓        move cursor down",
		"    k / ↑        move cursor up",
		"    g            jump to top",
		"    G            jump to bottom",
		"    enter        enter directory (BROWSE tab)",
		"    backspace    go to parent directory",
		"    esc          go to parent directory",
		"    shift-tab    switch tab (BROWSE / LARGEST / HISTORY)",
	}
	if !m.readOnly {
		lines = append(lines,
			"",
			"  SELECTION & DELETION",
			"    s            toggle select on cursor item",
			"    S            range select from last select to cursor",
			"    d            delete selected (or cursor item)",
			"    e            dry-run preview of selected items",
			"    tab          toggle trash / permanent delete mode",
			"    y / n        confirm / cancel deletion",
			"",
			"  HISTORY TAB",
			"    r            restore selected items to original location",
			"    d            permanently delete selected items from trash",
			"    s            toggle select",
		)
	}
	lines = append(lines,
		"",
		"  DISPLAY",
		"    f            open filter prompt (type to filter, enter to apply, esc to clear)",
		"    h            toggle hidden files",
		"    t            cycle sort mode: size / name / updated / created (BROWSE tab)",
		"    space        Quick Look preview (macOS)",
		"",
		"  OTHER",
		"    ?            show this help",
		"    q / ctrl+c   quit",
		"",
		"  FLAGS",
		"    -n N         max items in LARGEST tab (default: 1000)",
		"    --read-only  disable deletion",
		"    -y           skip delete confirmation",
		"    [paths...]   one or more directories to scan",
	)

	usedLines := 10 // logo(4) + path + tabbar + sep + header-placeholder + help + status
	visibleRows := m.height - usedLines
	if visibleRows < 1 {
		visibleRows = 1
	}

	for i, line := range lines {
		if i >= visibleRows {
			break
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad remaining
	rendered := len(lines)
	if rendered > visibleRows {
		rendered = visibleRows
	}
	for i := rendered; i < visibleRows; i++ {
		b.WriteString("\n")
	}

	// Separator
	b.WriteString(separatorStyle.Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Help line
	b.WriteString(helpDescStyle.Width(contentWidth).Render(fmt.Sprintf(" press any key to close")))
	b.WriteString("\n")

	// Status
	b.WriteString(statusBarStyle.Width(contentWidth).Render(" help"))

	return b.String()
}
