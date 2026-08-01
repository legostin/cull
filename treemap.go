package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// mapRect is one treemap rectangle in terminal cells.
type mapRect struct {
	X, Y, W, H int
	Index      int // index into the input entries; -1 for the "+N more" rect
	More       int // number of collapsed entries when Index == -1
}

// treemapMinArea is the estimated cell area below which entries are
// collapsed into a single "+N more" rectangle (spec: 8×3).
const treemapMinArea = 24

type treemapItem struct {
	index  int
	weight int64
	more   int
}

// layoutTreemap tiles w×h cells with rectangles proportional to entry sizes
// using recursive binary weighted split along the longer axis. Parent
// entries are skipped; unsized entries weigh 1 byte. Entries whose estimated
// area is under treemapMinArea are merged into one "+N more" item.
func layoutTreemap(entries []Entry, w, h int) []mapRect {
	if w <= 0 || h <= 0 {
		return nil
	}
	var items []treemapItem
	var total int64
	for i, e := range entries {
		if e.IsParent {
			continue
		}
		wt := e.Size
		if wt < 1 {
			wt = 1
		}
		items = append(items, treemapItem{index: i, weight: wt})
		total += wt
	}
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].weight > items[j].weight })

	// Collapse the tail of items whose estimated area is too small.
	area := int64(w * h)
	cut := len(items)
	for cut > 1 && items[cut-1].weight*area/total < treemapMinArea {
		cut--
	}
	if cut < len(items) {
		merged := treemapItem{index: -1}
		for _, it := range items[cut:] {
			merged.weight += it.weight
			merged.more++
		}
		items = append(items[:cut], merged)
		sort.SliceStable(items, func(i, j int) bool { return items[i].weight > items[j].weight })
	}

	var out []mapRect
	splitTreemap(items, w, h, 0, 0, &out)
	return out
}

// splitTreemap recursively divides the rect among items by weight.
func splitTreemap(items []treemapItem, w, h, x, y int, out *[]mapRect) {
	if len(items) == 0 || w <= 0 || h <= 0 {
		return
	}
	if len(items) == 1 {
		it := items[0]
		*out = append(*out, mapRect{X: x, Y: y, W: w, H: h, Index: it.index, More: it.more})
		return
	}
	var total int64
	for _, it := range items {
		total += it.weight
	}
	// Greedy prefix closest to half the weight (items are size-desc).
	var acc int64
	k := 1
	for i := 0; i < len(items)-1; i++ {
		acc += items[i].weight
		k = i + 1
		if acc*2 >= total {
			break
		}
	}
	var left int64
	for _, it := range items[:k] {
		left += it.weight
	}
	// Terminal cells are ~2× taller than wide; weight the axis choice so
	// rectangles come out visually square-ish rather than as thin columns.
	if w >= 2*h {
		lw := int(int64(w) * left / total)
		if lw < 1 {
			lw = 1
		}
		if lw >= w {
			lw = w - 1
		}
		splitTreemap(items[:k], lw, h, x, y, out)
		splitTreemap(items[k:], w-lw, h, x+lw, y, out)
	} else {
		lh := int(int64(h) * left / total)
		if lh < 1 {
			lh = 1
		}
		if lh >= h {
			lh = h - 1
		}
		splitTreemap(items[:k], w, lh, x, y, out)
		splitTreemap(items[k:], w, h-lh, x, y+lh, out)
	}
}

// nearestRect returns the index (into rects) of the rectangle one step in
// direction (dx,dy) from rects[cur], or cur when none exists. Because the
// layout is a perfect tiling, stepping one cell past the current rect's
// border from its center line hits exactly the visually adjacent rect —
// no diagonal jumps.
func nearestRect(rects []mapRect, cur, dx, dy int) int {
	if cur < 0 || cur >= len(rects) || (dx == 0 && dy == 0) {
		return cur
	}
	r := rects[cur]
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	var px, py int
	switch {
	case dx < 0:
		px, py = r.X-1, cy
	case dx > 0:
		px, py = r.X+r.W, cy
	case dy < 0:
		px, py = cx, r.Y-1
	default:
		px, py = cx, r.Y+r.H
	}
	for i, c := range rects {
		if i != cur && px >= c.X && px < c.X+c.W && py >= c.Y && py < c.Y+c.H {
			return i
		}
	}
	return cur
}

// browseMapLayout computes the treemap layout for the BROWSE tab at the
// current terminal size (same content area as the list rows).
func (m *model) browseMapLayout() []mapRect {
	w := m.width - 2
	h := m.height - 11
	if w < 1 || h < 1 {
		return nil
	}
	return layoutTreemap(m.tabs[tabBrowse].entries, w, h)
}

// mapCursorRect returns the index in rects of the cursor's rectangle: the
// rect with Index == cursor, else the "+N more" rect, else 0.
func (m *model) mapCursorRect(rects []mapRect) int {
	cur := m.tabs[tabBrowse].cursor
	moreIdx := 0
	for i, r := range rects {
		if r.Index == cur {
			return i
		}
		if r.Index == -1 {
			moreIdx = i
		}
	}
	return moreIdx
}

// Style classes for treemap grid cells.
const (
	mapClsBorder uint8 = iota
	mapClsLabel
	mapClsDirFill
	mapClsFileFill
	mapClsCursor
	mapClsSelected
	mapClsPending
	mapClsFill10MB
	mapClsFill100MB
	mapClsFill1GB
	mapClsFill10GB
)

// mapFillClass picks the fill style class for a sized entry: the same
// weight ladder as the list's size column (10 MB / 100 MB / 1 GB / 10 GB),
// falling back to the type color (dir blue, file gray) for small entries.
func mapFillClass(size int64, isDir bool) uint8 {
	const (
		MB = int64(1) << 20
		GB = int64(1) << 30
	)
	switch {
	case size >= 10*GB:
		return mapClsFill10GB
	case size >= GB:
		return mapClsFill1GB
	case size >= 100*MB:
		return mapClsFill100MB
	case size >= 10*MB:
		return mapClsFill10MB
	case isDir:
		return mapClsDirFill
	default:
		return mapClsFileFill
	}
}

// mapMessage renders a centered one-line message padded to the given rows.
func mapMessage(msg string, rows int) string {
	var b strings.Builder
	mid := rows / 2
	for i := 0; i < rows; i++ {
		if i == mid {
			b.WriteString(scanningStyle.Render("  " + msg))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderMap draws the BROWSE treemap as a block of exactly `rows` lines.
func (m model) renderMap(width, rows int) string {
	if width < 40 || rows < 10 {
		return mapMessage("terminal too small for map view", rows)
	}
	t := m.tabs[tabBrowse]
	rects := layoutTreemap(t.entries, width, rows)
	if len(rects) == 0 {
		return mapMessage("empty directory", rows)
	}

	grid := make([][]rune, rows)
	cls := make([][]uint8, rows)
	for y := 0; y < rows; y++ {
		grid[y] = make([]rune, width)
		cls[y] = make([]uint8, width)
		for x := 0; x < width; x++ {
			grid[y][x] = ' '
		}
	}

	curRect := m.mapCursorRect(rects)
	for ri, r := range rects {
		var e Entry
		isMore := r.Index == -1
		if !isMore {
			e = t.entries[r.Index]
		}
		selected := !isMore && t.selected[e.Path]

		frameCls := mapClsBorder
		if ri == curRect {
			frameCls = mapClsCursor
		} else if selected {
			frameCls = mapClsSelected
		}

		fillRune, fillCls := '▒', mapClsFileFill
		switch {
		case isMore:
			fillRune, fillCls = '░', mapClsFileFill
		case e.IsDir && !e.Sized:
			fillRune, fillCls = '·', mapClsPending
		case e.IsDir:
			fillRune, fillCls = '█', mapFillClass(e.Size, true)
		default:
			fillCls = mapFillClass(e.Size, false)
		}
		if ri == curRect {
			fillCls = mapClsCursor
		}

		// Too small for a border: solid fill only.
		if r.W < 3 || r.H < 3 {
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := r.X; x < r.X+r.W; x++ {
					grid[y][x], cls[y][x] = fillRune, fillCls
				}
			}
			continue
		}

		x1, y1 := r.X+r.W-1, r.Y+r.H-1
		for x := r.X; x <= x1; x++ {
			grid[r.Y][x], cls[r.Y][x] = '─', frameCls
			grid[y1][x], cls[y1][x] = '─', frameCls
		}
		for y := r.Y; y <= y1; y++ {
			grid[y][r.X], cls[y][r.X] = '│', frameCls
			grid[y][x1], cls[y][x1] = '│', frameCls
		}
		grid[r.Y][r.X], grid[r.Y][x1] = '┌', '┐'
		grid[y1][r.X], grid[y1][x1] = '└', '┘'

		for y := r.Y + 1; y < y1; y++ {
			for x := r.X + 1; x < x1; x++ {
				grid[y][x], cls[y][x] = fillRune, fillCls
			}
		}

		// Label on the first interior row.
		var label string
		if isMore {
			label = fmt.Sprintf("+%d more…", r.More)
		} else {
			label = m.entryDisplayName(e)
			if e.Sized {
				var totalSize int64
				for _, en := range t.entries {
					if !en.IsParent && en.Sized {
						totalSize += en.Size
					}
				}
				label += " · " + formatSize(e.Size) + " · " + strings.TrimSpace(formatPercent(e.Size, totalSize))
			} else {
				label += " · …"
			}
			if selected {
				label = "● " + label
			}
		}
		labelCls := mapClsLabel
		if ri == curRect {
			labelCls = mapClsCursor
		} else if selected {
			labelCls = mapClsSelected
		}
		maxLabel := r.W - 2
		lr := []rune(" " + label + " ")
		if len(lr) > maxLabel {
			lr = lr[:maxLabel]
		}
		for i, ch := range lr {
			grid[r.Y+1][r.X+1+i], cls[r.Y+1][r.X+1+i] = ch, labelCls
		}
	}

	styles := map[uint8]lipgloss.Style{
		mapClsBorder:    mapBorderStyle,
		mapClsLabel:     mapLabelStyle,
		mapClsDirFill:   mapDirFillStyle,
		mapClsFileFill:  mapFileFillStyle,
		mapClsCursor:    mapCursorStyle,
		mapClsSelected:  mapSelectedStyle,
		mapClsPending:   mapPendingStyle,
		mapClsFill10MB:  mapFill10MBStyle,
		mapClsFill100MB: mapFill100MBStyle,
		mapClsFill1GB:   mapFill1GBStyle,
		mapClsFill10GB:  mapFill10GBStyle,
	}
	var b strings.Builder
	b.Grow(rows * width * 2)
	for y := 0; y < rows; y++ {
		x := 0
		for x < width {
			c := cls[y][x]
			start := x
			for x < width && cls[y][x] == c {
				x++
			}
			b.WriteString(styles[c].Render(string(grid[y][start:x])))
		}
		b.WriteString("\n")
	}
	return b.String()
}
