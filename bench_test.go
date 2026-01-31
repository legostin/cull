package main

import (
	"bytes"
	"container/heap"
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Bench tree parameters.
const (
	benchDirs         = 100
	benchFilesPerDir  = 1000
	benchTotalFiles   = benchDirs * benchFilesPerDir
)

var (
	benchRoot string
	benchOnce sync.Once
)

// generateBenchTree creates a directory tree with benchDirs subdirectories,
// each containing benchFilesPerDir 1-byte files. The tree is created once
// and shared across all benchmarks in the test binary run.
func generateBenchTree(tb testing.TB) string {
	tb.Helper()
	benchOnce.Do(func() {
		root, err := os.MkdirTemp("", "bench-spacefree-*")
		if err != nil {
			tb.Fatal(err)
		}
		for d := 0; d < benchDirs; d++ {
			dir := filepath.Join(root, fmt.Sprintf("dir_%03d", d))
			if err := os.Mkdir(dir, 0o755); err != nil {
				tb.Fatal(err)
			}
			for f := 0; f < benchFilesPerDir; f++ {
				name := filepath.Join(dir, fmt.Sprintf("file_%04d.dat", f))
				if err := os.WriteFile(name, []byte{0}, 0o644); err != nil {
					tb.Fatal(err)
				}
			}
		}
		benchRoot = root
	})
	if benchRoot == "" {
		tb.Fatal("bench tree was not created")
	}
	return benchRoot
}

func TestMain(m *testing.M) {
	code := m.Run()
	if benchRoot != "" {
		os.RemoveAll(benchRoot)
	}
	os.Exit(code)
}

// makeEntries builds a slice of n synthetic Entry values.
func makeEntries(n int) []Entry {
	rng := rand.New(rand.NewSource(42))
	entries := make([]Entry, n)
	now := time.Now()
	for i := range entries {
		entries[i] = Entry{
			Name:       fmt.Sprintf("file_%06d.dat", i),
			Path:       fmt.Sprintf("/tmp/bench/dir_%03d/file_%06d.dat", i%benchDirs, i),
			Size:       rng.Int63n(1 << 30),
			IsDir:      false,
			Sized:      true,
			ModTime:    now.Add(-time.Duration(rng.Intn(86400)) * time.Second),
			CreateTime: now.Add(-time.Duration(rng.Intn(86400*30)) * time.Second),
		}
	}
	return entries
}

// --- Benchmarks ---

func BenchmarkQuickScanDir(b *testing.B) {
	root := generateBenchTree(b)
	dir := filepath.Join(root, "dir_000")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := quickScanDir(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQuickScanDir_Large(b *testing.B) {
	root := generateBenchTree(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := quickScanDir(root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeepScan(b *testing.B) {
	root := generateBenchTree(b)
	// Collect first-level dirs for the scan.
	dirEntries, err := os.ReadDir(root)
	if err != nil {
		b.Fatal(err)
	}
	firstLevel := make([]string, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() {
			firstLevel = append(firstLevel, filepath.Join(root, de.Name()))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := startDeepScan(root, 1000, firstLevel)
		for msg := range ch {
			if msg.done {
				break
			}
		}
	}
}

func BenchmarkSortEntries_Size(b *testing.B) {
	base := makeEntries(benchTotalFiles)
	entries := make([]Entry, len(base))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(entries, base)
		sortEntries(entries, sortSizeDesc)
	}
}

func BenchmarkSortEntries_Name(b *testing.B) {
	base := makeEntries(benchTotalFiles)
	entries := make([]Entry, len(base))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(entries, base)
		sortEntries(entries, sortNameAsc)
	}
}

func BenchmarkSnapshotHeap(b *testing.B) {
	entries := makeEntries(1000)
	h := &entryHeap{}
	heap.Init(h)
	for _, e := range entries {
		heap.Push(h, e)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = snapshotHeap(h, false)
	}
}

func BenchmarkSaveDirCache(b *testing.B) {
	dir := b.TempDir()
	entries := makeEntries(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		saveDirCache(dir, entries)
	}
}

func BenchmarkLoadDirCache(b *testing.B) {
	dir := b.TempDir()
	entries := makeEntries(1000)
	saveDirCache(dir, entries)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = loadDirCache(dir)
	}
}

func BenchmarkPathInterner(b *testing.B) {
	paths := make([]string, benchTotalFiles)
	for i := range paths {
		paths[i] = fmt.Sprintf("/tmp/bench/dir_%03d/file_%06d.dat", i%benchDirs, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pi := NewPathInterner()
		for _, p := range paths {
			pi.Intern(p)
		}
	}
}

// BenchmarkGobEncode measures raw gob encoding of 1000 entries (no disk I/O).
func BenchmarkGobEncode(b *testing.B) {
	entries := makeEntries(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&entries); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGobDecode measures raw gob decoding of 1000 entries (no disk I/O).
func BenchmarkGobDecode(b *testing.B) {
	entries := makeEntries(1000)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&entries); err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out []Entry
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&out); err != nil {
			b.Fatal(err)
		}
	}
}
