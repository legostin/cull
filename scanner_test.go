package main

import (
	"container/heap"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSortEntries_SizeDesc(t *testing.T) {
	entries := []Entry{
		{Name: "..", IsParent: true},
		{Name: "small", Size: 10},
		{Name: "big", Size: 1000},
		{Name: "medium", Size: 100},
	}
	sortEntries(entries, sortSizeDesc)

	if entries[0].Name != ".." {
		t.Error("parent should always be first")
	}
	if entries[1].Size < entries[2].Size || entries[2].Size < entries[3].Size {
		t.Errorf("not sorted by size desc: %d, %d, %d",
			entries[1].Size, entries[2].Size, entries[3].Size)
	}
}

func TestSortEntries_NameAsc(t *testing.T) {
	entries := []Entry{
		{Name: "..", IsParent: true},
		{Name: "zebra"},
		{Name: "apple"},
		{Name: "mango"},
	}
	sortEntries(entries, sortNameAsc)

	if entries[0].Name != ".." {
		t.Error("parent should always be first")
	}
	if entries[1].Name != "apple" || entries[2].Name != "mango" || entries[3].Name != "zebra" {
		t.Errorf("not sorted by name: %s, %s, %s",
			entries[1].Name, entries[2].Name, entries[3].Name)
	}
}

func TestSortEntries_UpdatedDesc(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Name: "..", IsParent: true},
		{Name: "old", ModTime: now.Add(-time.Hour)},
		{Name: "new", ModTime: now},
		{Name: "mid", ModTime: now.Add(-30 * time.Minute)},
	}
	sortEntries(entries, sortUpdatedDesc)

	if entries[0].Name != ".." {
		t.Error("parent should always be first")
	}
	if entries[1].Name != "new" {
		t.Errorf("first non-parent should be 'new', got %q", entries[1].Name)
	}
}

func TestSortEntries_CreatedDesc(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Name: "old", CreateTime: now.Add(-time.Hour)},
		{Name: "new", CreateTime: now},
	}
	sortEntries(entries, sortCreatedDesc)

	if entries[0].Name != "new" {
		t.Errorf("first should be 'new', got %q", entries[0].Name)
	}
}

func TestSortEntries_NoParent(t *testing.T) {
	entries := []Entry{
		{Name: "b", Size: 10},
		{Name: "a", Size: 20},
	}
	sortEntries(entries, sortSizeDesc)
	if entries[0].Name != "a" {
		t.Error("without parent, sorting should still work")
	}
}

func TestSnapshotHeap(t *testing.T) {
	h := &entryHeap{}
	heap.Init(h)
	heap.Push(h, Entry{Name: "small", Size: 10})
	heap.Push(h, Entry{Name: "big", Size: 1000})
	heap.Push(h, Entry{Name: "medium", Size: 100})

	origLen := h.Len()
	snap := snapshotHeap(h)

	// Should not modify original heap
	if h.Len() != origLen {
		t.Errorf("heap length changed: %d -> %d", origLen, h.Len())
	}

	// Snapshot should be sorted size desc
	if len(snap) != 3 {
		t.Fatalf("snapshot len = %d, want 3", len(snap))
	}
	if snap[0].Size < snap[1].Size || snap[1].Size < snap[2].Size {
		t.Errorf("snapshot not sorted desc: %d, %d, %d",
			snap[0].Size, snap[1].Size, snap[2].Size)
	}
}

func TestSnapshotHeap_Empty(t *testing.T) {
	h := &entryHeap{}
	heap.Init(h)
	snap := snapshotHeap(h)
	if len(snap) != 0 {
		t.Errorf("empty heap snapshot len = %d, want 0", len(snap))
	}
}

func TestEntryHeap(t *testing.T) {
	h := &entryHeap{}
	heap.Init(h)
	heap.Push(h, Entry{Name: "a", Size: 100})
	heap.Push(h, Entry{Name: "b", Size: 10})
	heap.Push(h, Entry{Name: "c", Size: 50})

	// Min-heap: smallest should come out first
	e := heap.Pop(h).(Entry)
	if e.Size != 10 {
		t.Errorf("first pop size = %d, want 10", e.Size)
	}
}

func TestLoadSaveDirCache_RoundTrip(t *testing.T) {
	// Create a real temporary directory
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file so the dir has content
	if err := os.WriteFile(filepath.Join(subDir, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{Name: "test.txt", Path: filepath.Join(subDir, "test.txt"), Size: 5, Sized: true},
	}

	saveDirCache(subDir, entries)

	loaded, ok := loadDirCache(subDir)
	if !ok {
		t.Fatal("loadDirCache returned false after save")
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d entries, want 1", len(loaded))
	}
	if loaded[0].Name != "test.txt" || loaded[0].Size != 5 {
		t.Errorf("loaded entry: name=%q size=%d", loaded[0].Name, loaded[0].Size)
	}
}

func TestLoadDirCache_Stale(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "staledir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{{Name: "old.txt"}}
	saveDirCache(subDir, entries)

	// Modify the directory (add a file) to make cache stale
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(subDir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, ok := loadDirCache(subDir)
	if ok {
		t.Error("stale cache should return false")
	}
}

func TestLoadDirCache_NonexistentDir(t *testing.T) {
	_, ok := loadDirCache("/this/path/does/not/exist")
	if ok {
		t.Error("nonexistent dir should return false")
	}
}

func TestCachePath(t *testing.T) {
	p := cachePath("/some/dir")
	if p == "" {
		t.Skip("cache directory not available")
	}
	// Should end with .gob
	if filepath.Ext(p) != ".gob" {
		t.Errorf("cache path %q should end with .gob", p)
	}
}

func TestSortEntries_StableParentPosition(t *testing.T) {
	// Verify parent stays first regardless of its Size field
	entries := []Entry{
		{Name: "..", IsParent: true, Size: 999999},
		{Name: "z", Size: 1},
		{Name: "a", Size: 100},
	}
	sortEntries(entries, sortNameAsc)
	if !entries[0].IsParent {
		t.Error("parent with large size should still be first after name sort")
	}
}
