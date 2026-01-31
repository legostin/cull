package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

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

	var parts []string
	for _, ti := range tabs {
		label := ti.name
		// Add scanning indicator for LARGEST tab while deep scan runs
		if ti.id == tabLargest && ti.id != m.activeTab && m.deepScanning {
			label += " ◐"
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
	if m.deleteType == deletePermanent {
		logoStyle = titleStyleDanger
	}
	for i, line := range logo {
		styled := logoStyle.Render(line)
		if i == 1 && freeText != "" {
			pad := contentWidth - lipgloss.Width(styled) - len(freeText)
			if pad < 1 {
				pad = 1
			}
			styled += strings.Repeat(" ", pad) + lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render(freeText)
		}
		b.WriteString(styled)
		b.WriteString("\n")
	}

	// Path
	var pathPrefix string
	if m.deleteType == deletePermanent {
		pathPrefix = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("⚠ PERMANENT DELETE") + " "
	}
	pathLine := pathStyle.Width(contentWidth).Render(" " + pathPrefix + m.path)
	b.WriteString(pathLine)
	b.WriteString("\n")

	// Tab bar
	b.WriteString(m.renderTabBar(contentWidth))
	b.WriteString("\n")

	// Separator
	b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", contentWidth)))
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

	// Compute max size for proportion bar
	var maxSize int64
	for _, e := range t.entries {
		if !e.IsParent && e.Sized && e.Size > maxSize {
			maxSize = e.Size
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
		header := headerStyle.Render(fmt.Sprintf("  %*s %9s  %-*s  %-10s  %-10s",
			barWidth, "",
			sizeHeaderLabel(m.sortBy),
			nameWidth, nameHeaderLabel(m.sortBy),
			createdHeaderLabel(m.sortBy),
			updatedHeaderLabel(m.sortBy)))
		b.WriteString(header)
	case tabLargest:
		header := headerStyle.Render(fmt.Sprintf("  %*s %9s  %-*s  %-10s  %-10s",
			barWidth, "",
			"SIZE▼",
			nameWidth, "NAME",
			"CREATED",
			"UPDATED"))
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
				plain := fmt.Sprintf("  %s %9s  %-*s  %10s  %10s", pad, "", nameWidth, "..", "", "")
				row := cursorStyle.Width(contentWidth).Render(plain)
				b.WriteString(row)
			} else {
				b.WriteString(fmt.Sprintf("  %s %9s  %-*s  %10s  %10s", pad, "", nameWidth, dirStyle.Render(".."), "", ""))
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
		if e.IsDir {
			name += "/"
		}

		isActiveScanning := m.deepScanning && !e.IsParent && e.IsDir && e.Name == m.deepScanDir
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
			plain := fmt.Sprintf("%s%s %9s  %-*s  %-10s  %-10s", markerText, plainBar, sizeFormatted, nameWidth, dn, createdStr, updatedStr)
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

			created := dateStyle.Render(createdStr)
			updated := dateStyle.Render(updatedStr)

			// Truncate name for non-cursor rows
			truncName := truncateName(displayName, nameWidth)

			if e.IsDir {
				nameStyleLocal := dirStyle
				if isActiveScanning {
					nameStyleLocal = scanningNameStyle
				} else if isUnsized {
					nameStyleLocal = dirUnsizedStyle
				}
				styledName := nameStyleLocal.Render(truncName)
				namePad := nameWidth - lipgloss.Width(styledName)
				if namePad < 0 {
					namePad = 0
				}
				row = fmt.Sprintf("%s%s %s  %s%s  %s  %s", marker, bar, size, styledName, strings.Repeat(" ", namePad), created, updated)
			} else {
				row = fmt.Sprintf("%s%s %s  %-*s  %s  %s", marker, bar, size, nameWidth, truncName, created, updated)
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
		for p := range t.selected {
			for _, e := range t.entries {
				if e.Path == p {
					totalSize += e.Size
					count++
					break
				}
			}
		}
		var confirmText string
		if m.deleteType == deleteTrash {
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
		help := fmt.Sprintf(" %s select %s delete %s filter %s sort %s preview %s del mode %s tabs %s help %s quit",
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
		b.WriteString(helpDescStyle.Width(contentWidth).Render(help))
		b.WriteString("\n")
	}

	// Status line
	statusParts := []string{}

	if m.deepScanning {
		scanDir := m.deepScanDir
		if scanDir == "" {
			scanDir = "…"
		}
		statusParts = append(statusParts, fmt.Sprintf("scanning: %s/ ◐", scanDir))
	}

	if len(statusParts) > 0 {
		status := " " + strings.Join(statusParts, " · ") + fmt.Sprintf(" · %d items", len(t.entries))
		b.WriteString(scanningStyle.Width(contentWidth).Render(status))
	} else {
		var totalSelected int64
		for p := range t.selected {
			for _, e := range t.entries {
				if e.Path == p {
					totalSelected += e.Size
					break
				}
			}
		}
		status := fmt.Sprintf(" %d items", len(t.entries))
		if t.filterText != "" && m.mode != modeFilter {
			status = fmt.Sprintf(" %d of %d items · filter: \"%s\"", len(t.entries), len(t.allEntries), t.filterText)
		}
		if len(t.selected) > 0 {
			status += fmt.Sprintf(" · %d selected · %s", len(t.selected), formatSize(totalSelected))
		}
		// Mode indicators
		if !m.showHidden {
			status += " · hidden:off"
		}
		if m.deleteType == deletePermanent {
			status += " · PERM DELETE"
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
	for p := range t.selected {
		for _, e := range t.allEntries {
			if e.Path == p {
				if count < visibleRows {
					line := fmt.Sprintf("  %9s  %s", formatSize(e.Size), e.Path)
					b.WriteString(line)
					b.WriteString("\n")
				}
				totalSize += e.Size
				count++
				break
			}
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
	b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", contentWidth)))
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
		"    shift-tab    switch tab (BROWSE / LARGEST)",
		"",
		"  SELECTION & DELETION",
		"    s            toggle select on cursor item",
		"    S            range select from last select to cursor",
		"    d            delete selected (or cursor item)",
		"    e            dry-run preview of selected items",
		"    tab          toggle trash / permanent delete mode",
		"    y / n        confirm / cancel deletion",
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
		"    [paths...]   one or more directories to scan",
	}

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
	b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Help line
	b.WriteString(helpDescStyle.Width(contentWidth).Render(fmt.Sprintf(" press any key to close")))
	b.WriteString("\n")

	// Status
	b.WriteString(statusBarStyle.Width(contentWidth).Render(" help"))

	return b.String()
}
