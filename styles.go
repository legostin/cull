package main

import "github.com/charmbracelet/lipgloss"

var (
	// Colors — k9s-inspired dark theme
	colorFg       = lipgloss.Color("#e0e0e0")
	colorDim      = lipgloss.Color("#666677")
	colorBlue     = lipgloss.Color("#5599dd")
	colorYellow   = lipgloss.Color("#e8c547")
	colorGreen    = lipgloss.Color("#44cc88")
	colorRed      = lipgloss.Color("#ee5566")
	colorCursorBg = lipgloss.Color("#264f78")
	colorBorder   = lipgloss.Color("#444466")

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorYellow).
			Padding(0, 1)

	// Current path
	pathStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Padding(0, 1)

	// Column header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDim).
			Padding(0, 1)

	// Normal file row
	fileStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	// Directory name
	dirStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	// Cursor row
	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(colorCursorBg)

	// Selected marker
	selectedMarkerStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)

	// Size column — default (< 10 MB)
	sizeStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Width(9).
			Align(lipgloss.Right)

	// Size column — 10 MB+
	sizeStyle10MB = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88aa44")).
			Width(9).
			Align(lipgloss.Right)

	// Size column — 100 MB+
	sizeStyle100MB = lipgloss.NewStyle().
			Foreground(colorYellow).
			Width(9).
			Align(lipgloss.Right)

	// Size column — 1 GB+
	sizeStyle1GB = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ee8833")).
			Bold(true).
			Width(9).
			Align(lipgloss.Right)

	// Size column — 10 GB+
	sizeStyle10GB = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Width(9).
			Align(lipgloss.Right)

	// Size column — pending (not yet scanned)
	sizePendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				Width(9).
				Align(lipgloss.Right)

	// Date column (created / updated)
	dateStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Width(10)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)

	// Keybindings help
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	// Confirm dialog
	confirmStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(0, 1)

	// Symlink name (light cyan)
	symlinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66ccdd"))

	// Unsized directory name (dim, not yet computed)
	dirUnsizedStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	// Stale history entry (trash emptied externally, cannot restore)
	staleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	// Long-idle project row on the PROJECTS tab (safe to clean)
	idleRowStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	// Currently scanning directory name (dark yellow)
	scanningNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#b8962e"))

	// Scanning indicator
	scanningStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true).
			Padding(0, 1)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(0, 1)

	// Filter input line
	filterStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true).
			Padding(0, 1)

	// Proportion bar (filled portion)
	barFilledStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	// Dry-run preview header
	dryRunHeaderStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true).
				Padding(0, 1)

	// Title bar — danger (permanent delete mode)
	titleStyleDanger = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorRed).
				Padding(0, 1)

	// Title bar — read-only mode
	titleStyleReadOnly = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorGreen).
				Padding(0, 1)

	// Tab bar — active tab
	tabActiveStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	// Tab bar — inactive tab
	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	// Dupe group header
	dupeHeaderStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Bold(true)

	// Scanning row gradient wave palette (muted olive → soft gold → muted olive)
	scanGradient = []lipgloss.Color{
		"#6a6040",
		"#7d7248",
		"#908450",
		"#a39658",
		"#b5a860",
		"#c4b568",
		"#d0c070",
		"#c4b568",
		"#b5a860",
		"#a39658",
		"#908450",
		"#7d7248",
	}

	// Pre-built gradient styles (one per scanGradient color)
	scanGradientStyles []lipgloss.Style

	// Hoisted inline styles used in View()
	freeTextStyle       = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	readOnlyBadgeStyle  = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	permDeleteBadgeStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	separatorStyle      = lipgloss.NewStyle().Foreground(colorBorder)
	readOnlyStatusStyle = lipgloss.NewStyle().Foreground(colorGreen)
)

func init() {
	scanGradientStyles = make([]lipgloss.Style, len(scanGradient))
	for i, c := range scanGradient {
		scanGradientStyles[i] = lipgloss.NewStyle().Foreground(c)
	}
}
