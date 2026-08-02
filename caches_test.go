package main

import (
	"os"
	"path/filepath"
	"strings"
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

	// Sizes are allocated blocks (du semantics): at least the 157 payload
	// bytes, at most three files rounded up to whole blocks.
	got := sumPathsSize([]string{dir1, dir2})
	if got < 157 || got > 3*1<<20 {
		t.Errorf("sumPathsSize = %d, want between 157 and 3 blocks", got)
	}
	if got := sumPathsSize([]string{filepath.Join(dir1, "nope")}); got != 0 {
		t.Errorf("missing path: sumPathsSize = %d, want 0", got)
	}
}

func TestPlatformCacheDefs(t *testing.T) {
	defs := platformCacheDefs()
	if len(defs) == 0 {
		t.Fatal("platformCacheDefs returned no defs")
	}
	seen := make(map[string]bool)
	for _, d := range defs {
		if d.Name == "" {
			t.Errorf("def with empty Name: %+v", d)
		}
		if seen[d.Name] {
			t.Errorf("duplicate def name %q", d.Name)
		}
		seen[d.Name] = true
		if d.Kind == cacheKindDir && len(d.Paths) == 0 {
			t.Errorf("%s: cacheKindDir def has no paths", d.Name)
		}
		for _, p := range d.Paths {
			if strings.Contains(p, "**") {
				t.Errorf("%s: pattern %q uses ** which filepath.Glob does not support", d.Name, p)
			}
			if p == "" {
				t.Errorf("%s: empty path pattern", d.Name)
			}
		}
	}
}

func TestParseDockerSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"0B", 0},
		{"500B", 500},
		{"1.5kB", 1500},
		{"500MB", 500_000_000},
		{"1.5GB (90%)", 1_500_000_000},
		{"2TB", 2_000_000_000_000},
		{"  1.234GB  ", 1_234_000_000},
		{"garbage", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseDockerSize(tt.in); got != tt.want {
			t.Errorf("parseDockerSize(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseDockerDF(t *testing.T) {
	out := []byte(`{"Type":"Images","Reclaimable":"1.5GB (90%)"}
{"Type":"Containers","Reclaimable":"0B"}
{"Type":"Local Volumes","Reclaimable":"500MB (100%)"}
{"Type":"Build Cache","Reclaimable":"2GB"}
not json line
`)
	want := int64(1_500_000_000 + 0 + 500_000_000 + 2_000_000_000)
	if got := parseDockerDF(out); got != want {
		t.Errorf("parseDockerDF = %d, want %d", got, want)
	}
	if got := parseDockerDF(nil); got != 0 {
		t.Errorf("parseDockerDF(nil) = %d, want 0", got)
	}
}

func TestParseDockerPruneOutput(t *testing.T) {
	out := "Deleted Containers:\nabc123\n\nDeleted Images:\ndef456\n\nTotal reclaimed space: 4.2GB\n"
	if got := parseDockerPruneOutput(out); got != 4_200_000_000 {
		t.Errorf("parseDockerPruneOutput = %d, want 4200000000", got)
	}
	if got := parseDockerPruneOutput("no totals here"); got != 0 {
		t.Errorf("no-total case = %d, want 0", got)
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

func TestParseTMSnapshotDates(t *testing.T) {
	out := `Snapshots for volume group containing disk /:
com.apple.os.update-5514FF97DEE9C60C7FBF462B06A418D5FC4A882D5AF41D2BAF0A1419FB9B9F86
com.apple.TimeMachine.2026-08-01-101112.local
com.apple.TimeMachine.2026-08-02-093000.local
com.apple.os.update-MSUPrepareUpdate
`
	dates := parseTMSnapshotDates([]byte(out))
	want := []string{"2026-08-01-101112", "2026-08-02-093000"}
	if len(dates) != 2 || dates[0] != want[0] || dates[1] != want[1] {
		t.Errorf("parseTMSnapshotDates = %v, want %v", dates, want)
	}
	if got := parseTMSnapshotDates([]byte("Snapshots for volume group containing disk /:\ncom.apple.os.update-X\n")); len(got) != 0 {
		t.Errorf("os.update-only output must yield no deletable snapshots, got %v", got)
	}
}
