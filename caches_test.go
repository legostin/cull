package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCachePaths(t *testing.T) {
	home := t.TempDir()
	// Existing plain dir
	mustMkdir(t, filepath.Join(home, ".npm", "_cacache"))
	// Glob targets: two profile dirs with a cache2 subdir each
	mustMkdir(t, filepath.Join(home, "profiles", "abc.default", "cache2"))
	mustMkdir(t, filepath.Join(home, "profiles", "xyz.dev", "cache2"))

	tests := []struct {
		name     string
		patterns []string
		want     int
	}{
		{"plain existing", []string{"~/.npm/_cacache"}, 1},
		{"plain missing", []string{"~/.cache/nonexistent"}, 0},
		{"glob matches two", []string{"~/profiles/*/cache2"}, 2},
		{"glob no match", []string{"~/profiles/*/nope"}, 0},
		{"mixed", []string{"~/.npm/_cacache", "~/profiles/*/cache2", "~/missing"}, 3},
		{"empty pattern skipped", []string{""}, 0},
	}
	for _, tt := range tests {
		got := resolveCachePaths(tt.patterns, home)
		if len(got) != tt.want {
			t.Errorf("%s: got %d paths %v, want %d", tt.name, len(got), got, tt.want)
		}
		for _, p := range got {
			if !filepath.IsAbs(p) {
				t.Errorf("%s: path %q is not absolute", tt.name, p)
			}
		}
	}
}

func TestScanCaches(t *testing.T) {
	home := t.TempDir()
	mustMkdir(t, filepath.Join(home, "a"))
	mustMkdir(t, filepath.Join(home, "b"))

	defs := []cacheDef{
		{Name: "both exist", Paths: []string{"~/a", "~/b"}},
		{Name: "one exists", Paths: []string{"~/a", "~/missing"}},
		{Name: "none exist", Paths: []string{"~/missing"}},
		{Name: "docker skipped", Paths: nil, Kind: cacheKindDocker},
	}
	hits := scanCaches(defs, home)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
	if hits[0].Def.Name != "both exist" || len(hits[0].Paths) != 2 {
		t.Errorf("hit 0 = %+v, want 'both exist' with 2 paths", hits[0])
	}
	if hits[1].Def.Name != "one exists" || len(hits[1].Paths) != 1 {
		t.Errorf("hit 1 = %+v, want 'one exists' with 1 path", hits[1])
	}
}

func TestSumPathsSize(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	mustWriteFile(t, filepath.Join(dir1, "f1"), 100)
	mustMkdir(t, filepath.Join(dir1, "sub"))
	mustWriteFile(t, filepath.Join(dir1, "sub", "f2"), 50)
	mustWriteFile(t, filepath.Join(dir2, "f3"), 7)

	if got := sumPathsSize([]string{dir1, dir2}); got != 157 {
		t.Errorf("sumPathsSize = %d, want 157", got)
	}
	if got := sumPathsSize([]string{filepath.Join(dir1, "nope")}); got != 0 {
		t.Errorf("missing path: sumPathsSize = %d, want 0", got)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}
