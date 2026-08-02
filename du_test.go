package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiskUsageSparseFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sparse.img")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 30); err != nil { // 1 GiB logical, no data
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	got := diskUsage(info)
	if got == info.Size() {
		t.Skip("filesystem does not create sparse holes")
	}
	if got > 10<<20 {
		t.Errorf("diskUsage(sparse 1GiB) = %d, want far below logical size", got)
	}
}

func TestDiskUsageRegularFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.bin")
	payload := make([]byte, 100_000)
	if err := os.WriteFile(p, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	got := diskUsage(info)
	// Allocated size is the payload rounded up to whole blocks.
	if got < int64(len(payload)) || got > int64(len(payload))+1<<20 {
		t.Errorf("diskUsage(100KB file) = %d, want ≈ %d", got, len(payload))
	}
}

func TestSumPathsSizeDedupesHardlinks(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "orig")
	payload := make([]byte, 50_000)
	if err := os.WriteFile(orig, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(orig, filepath.Join(dir, "twin")); err != nil {
		t.Skip("hardlinks not supported here")
	}
	got := sumPathsSize([]string{dir})
	if got > int64(len(payload))+1<<20 {
		t.Errorf("sumPathsSize with a hardlink twin = %d, want the payload counted once (≈%d)", got, len(payload))
	}
}

func TestDeepScanUsesAllocatedSize(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	sp := filepath.Join(sub, "sparse.img")
	f, err := os.Create(sp)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 30); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if info, _ := os.Stat(sp); diskUsage(info) == info.Size() {
		t.Skip("filesystem does not create sparse holes")
	}

	ch := startDeepScan(root, 10, []string{sub})
	var final deepScanMsg
	for msg := range ch {
		if msg.done {
			final = msg
		}
	}
	if len(final.entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(final.entries))
	}
	if final.entries[0].Size >= 1<<30 {
		t.Errorf("deep scan Size = %d, want allocated size far below the 1GiB logical length", final.entries[0].Size)
	}
	if final.dirSizes[sub] >= 1<<30 {
		t.Errorf("dirSizes = %d, want allocated size", final.dirSizes[sub])
	}
}
