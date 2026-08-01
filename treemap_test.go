package main

import "testing"

func rectsArea(rs []mapRect) int {
	a := 0
	for _, r := range rs {
		a += r.W * r.H
	}
	return a
}

func TestLayoutTreemapTiling(t *testing.T) {
	entries := []Entry{
		{Name: "a", Path: "/a", Size: 1 << 30, Sized: true},
		{Name: "b", Path: "/b", Size: 500 << 20, Sized: true},
		{Name: "c", Path: "/c", Size: 200 << 20, Sized: true},
		{Name: "d", Path: "/d", Size: 10 << 20, Sized: true},
	}
	const w, h = 100, 30
	rects := layoutTreemap(entries, w, h)
	if rectsArea(rects) != w*h {
		t.Fatalf("area = %d, want %d (gaps or overlaps)", rectsArea(rects), w*h)
	}
	occ := make([][]bool, h)
	for i := range occ {
		occ[i] = make([]bool, w)
	}
	for _, r := range rects {
		if r.X < 0 || r.Y < 0 || r.X+r.W > w || r.Y+r.H > h || r.W <= 0 || r.H <= 0 {
			t.Fatalf("rect out of bounds: %+v", r)
		}
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				if occ[y][x] {
					t.Fatalf("overlap at %d,%d (%+v)", x, y, r)
				}
				occ[y][x] = true
			}
		}
	}
	seen := map[int]int{}
	for _, r := range rects {
		if r.Index >= 0 {
			seen[r.Index]++
		}
	}
	for i := range entries {
		if seen[i] > 1 {
			t.Errorf("entry %d appears %d times", i, seen[i])
		}
	}
	if seen[0] != 1 {
		t.Error("largest entry must have its own rect")
	}
}

func TestLayoutTreemapCollapsesSmall(t *testing.T) {
	entries := []Entry{{Name: "big", Path: "/big", Size: 1 << 30, Sized: true}}
	for i := 0; i < 50; i++ {
		entries = append(entries, Entry{Name: "tiny", Path: "/t", Size: 1, Sized: true})
	}
	rects := layoutTreemap(entries, 60, 20)
	var more *mapRect
	for i := range rects {
		if rects[i].Index == -1 {
			more = &rects[i]
		}
	}
	if more == nil || more.More != 50 {
		t.Fatalf("want +50 more rect, got %+v", rects)
	}
}

func TestLayoutTreemapEdgeCases(t *testing.T) {
	if got := layoutTreemap(nil, 80, 24); got != nil {
		t.Errorf("empty input: got %+v", got)
	}
	one := layoutTreemap([]Entry{{Name: "a", Size: 5, Sized: true}}, 80, 24)
	if len(one) != 1 || one[0].W != 80 || one[0].H != 24 {
		t.Errorf("single entry must fill the area: %+v", one)
	}
	u := layoutTreemap([]Entry{{Name: "u"}, {Name: "v"}}, 80, 24)
	if len(u) != 2 {
		t.Errorf("unsized entries must still be laid out: %+v", u)
	}
	p := layoutTreemap([]Entry{{Name: "..", IsParent: true}, {Name: "a", Size: 5, Sized: true}}, 80, 24)
	if len(p) != 1 {
		t.Errorf("parent must be skipped: %+v", p)
	}
}

func TestNearestRect(t *testing.T) {
	// 2×2 grid of equal rects
	rs := []mapRect{
		{X: 0, Y: 0, W: 10, H: 5, Index: 0},
		{X: 10, Y: 0, W: 10, H: 5, Index: 1},
		{X: 0, Y: 5, W: 10, H: 5, Index: 2},
		{X: 10, Y: 5, W: 10, H: 5, Index: 3},
	}
	cases := []struct{ from, dx, dy, want int }{
		{0, 1, 0, 1}, {0, 0, 1, 2}, {3, -1, 0, 2}, {3, 0, -1, 1},
		{0, -1, 0, 0}, // no neighbor left: stay
		{1, 0, -1, 1}, // no neighbor up: stay
	}
	for _, c := range cases {
		if got := nearestRect(rs, c.from, c.dx, c.dy); got != c.want {
			t.Errorf("from %d dir(%d,%d): got %d, want %d", c.from, c.dx, c.dy, got, c.want)
		}
	}
}

func TestNearestRectAsymmetric(t *testing.T) {
	// A spans the top; B and C sit under it side by side.
	// ┌──── A ────┐
	// ├─ B ─┬─ C ─┤
	// └─────┴─────┘
	rs := []mapRect{
		{X: 0, Y: 0, W: 20, H: 4, Index: 0},  // A
		{X: 0, Y: 4, W: 10, H: 4, Index: 1},  // B
		{X: 10, Y: 4, W: 10, H: 4, Index: 2}, // C
	}
	if got := nearestRect(rs, 2, -1, 0); got != 1 {
		t.Errorf("left from C must go to B (same band), got %d", got)
	}
	if got := nearestRect(rs, 2, 0, -1); got != 0 {
		t.Errorf("up from C must go to A, got %d", got)
	}
	if got := nearestRect(rs, 1, -1, 0); got != 1 {
		t.Errorf("left from B must stay (edge), got %d", got)
	}
	if got := nearestRect(rs, 0, 0, 1); got != 2 {
		t.Errorf("down from A (center over C) must go to C, got %d", got)
	}
	if got := nearestRect(rs, 0, 0, -1); got != 0 {
		t.Errorf("up from A must stay (edge), got %d", got)
	}
}

func TestMapFillClass(t *testing.T) {
	const MB, GB = int64(1) << 20, int64(1) << 30
	cases := []struct {
		size  int64
		isDir bool
		want  uint8
	}{
		{5 * MB, true, mapClsDirFill},
		{5 * MB, false, mapClsFileFill},
		{50 * MB, true, mapClsFill10MB},
		{500 * MB, false, mapClsFill100MB},
		{2 * GB, true, mapClsFill1GB},
		{20 * GB, false, mapClsFill10GB},
	}
	for _, c := range cases {
		if got := mapFillClass(c.size, c.isDir); got != c.want {
			t.Errorf("mapFillClass(%d, %v) = %d, want %d", c.size, c.isDir, got, c.want)
		}
	}
}
