package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
