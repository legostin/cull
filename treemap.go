package main

import "sort"

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
	if w >= h {
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

// nearestRect returns the index (into rects) of the closest rectangle whose
// center lies in direction (dx,dy) from rects[cur], or cur when none exists.
// Off-axis distance is doubled so movement feels axis-aligned.
func nearestRect(rects []mapRect, cur, dx, dy int) int {
	if cur < 0 || cur >= len(rects) {
		return cur
	}
	cx := rects[cur].X*2 + rects[cur].W // centers ×2 to stay integral
	cy := rects[cur].Y*2 + rects[cur].H
	best, bestDist := cur, 1<<62
	for i, r := range rects {
		if i == cur {
			continue
		}
		rx := r.X*2 + r.W
		ry := r.Y*2 + r.H
		if dx > 0 && rx <= cx || dx < 0 && rx >= cx || dy > 0 && ry <= cy || dy < 0 && ry >= cy {
			continue
		}
		major := (rx-cx)*dx + (ry-cy)*dy
		var minor int
		if dx != 0 {
			minor = ry - cy
		} else {
			minor = rx - cx
		}
		if minor < 0 {
			minor = -minor
		}
		d := major + 2*minor
		if d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
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
