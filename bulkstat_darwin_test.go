//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReadDirBulkMatchesReadDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.bin"), make([]byte, 100_000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file.bin", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	recs, ok := readDirBulk(dir)
	if !ok {
		t.Skip("getattrlistbulk unavailable on this filesystem")
	}
	got := map[string]bulkRec{}
	for _, r := range recs {
		got[r.name] = r
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(got), got)
	}
	if !got["subdir"].isDir || got["subdir"].isSymlink {
		t.Errorf("subdir flags wrong: %+v", got["subdir"])
	}
	if !got["link"].isSymlink {
		t.Errorf("link flags wrong: %+v", got["link"])
	}
	f := got["file.bin"]
	if f.isDir || f.isSymlink {
		t.Errorf("file flags wrong: %+v", f)
	}
	if f.size < 100_000 || f.size > 100_000+1<<20 {
		t.Errorf("file size = %d, want ≈100000 (allocated)", f.size)
	}
	if f.ino == 0 {
		t.Error("file id must be set")
	}
	if f.mod.IsZero() {
		t.Error("mod time must be set")
	}
}

func TestReadDirBulkSparse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sparse.img")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 30); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if info, _ := os.Stat(p); diskUsage(info) == info.Size() {
		t.Skip("no sparse support")
	}

	recs, ok := readDirBulk(dir)
	if !ok {
		t.Skip("getattrlistbulk unavailable")
	}
	if len(recs) != 1 || recs[0].size > 10<<20 {
		t.Errorf("sparse via bulk = %+v, want tiny size", recs)
	}
}

func TestReadDirBulkCloneReportsPrivateSize(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "orig.bin")
	// Above privateSizeThreshold so the clone-aware size kicks in.
	if err := os.WriteFile(orig, make([]byte, 8<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	// APFS clone (cp -c); skip on filesystems without clone support.
	if out, err := exec.Command("/bin/cp", "-c", orig, filepath.Join(dir, "clone.bin")).CombinedOutput(); err != nil {
		t.Skipf("cp -c failed: %v %s", err, out)
	}

	recs, ok := readDirBulk(dir)
	if !ok {
		t.Skip("getattrlistbulk unavailable")
	}
	var clone bulkRec
	for _, r := range recs {
		if r.name == "clone.bin" {
			clone = r
		}
	}
	// The clone shares all blocks with the original: deleting it frees ~0.
	if clone.size > 128<<10 {
		t.Errorf("clone size = %d, want ~0 (private size, blocks shared with orig)", clone.size)
	}
}
