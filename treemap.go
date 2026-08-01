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
