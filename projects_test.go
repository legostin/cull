package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMatchArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []string
		none  []string // artifact names that must NOT be matched
	}{
		{"node", []string{"package.json", "index.js"},
			[]string{"node_modules", ".next", ".nuxt", ".turbo", "dist", ".parcel-cache"}, nil},
		{"rust", []string{"Cargo.toml"}, []string{"target"}, []string{"node_modules"}},
		{"go", []string{"go.mod"}, []string{"vendor"}, nil},
		{"python-pyproject", []string{"pyproject.toml"},
			[]string{".venv", "venv", "__pycache__", ".tox", ".mypy_cache", ".pytest_cache", ".ruff_cache"}, nil},
		{"python-requirements", []string{"requirements.txt"}, []string{".venv"}, nil},
		{"gradle", []string{"build.gradle.kts"}, []string{"build", ".gradle"}, nil},
		{"maven", []string{"pom.xml"}, []string{"target"}, nil},
		{"pods", []string{"Podfile"}, []string{"Pods"}, nil},
		{"swiftpm", []string{"Package.swift"}, []string{".build"}, nil},
		{"cmake", []string{"CMakeLists.txt"}, []string{"build"}, nil},
		{"terraform-glob", []string{"main.tf"}, []string{".terraform"}, nil},
		{"zig", []string{"build.zig"}, []string{"zig-cache", "zig-out"}, nil},
		{"elixir", []string{"mix.exs"}, []string{"_build", "deps"}, nil},
		{"composer", []string{"composer.json"}, []string{"vendor"}, nil},
		{"no-markers", []string{"readme.md", "src"}, nil, []string{"build", "target", "node_modules"}},
		{"union", []string{"package.json", "Cargo.toml"}, []string{"node_modules", "target"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchArtifacts(tc.files)
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("matchArtifacts(%v): missing %q", tc.files, w)
				}
			}
			for _, n := range tc.none {
				if got[n] {
					t.Errorf("matchArtifacts(%v): unexpected %q", tc.files, n)
				}
			}
		})
	}
}

func TestCautionArtifacts(t *testing.T) {
	if !cautionArtifacts["dist"] || !cautionArtifacts["vendor"] {
		t.Error("dist and vendor must be caution artifacts")
	}
	if cautionArtifacts["node_modules"] {
		t.Error("node_modules must not be a caution artifact")
	}
}

// mkProject creates dir with the given marker files and artifact dirs
// (each artifact dir gets one 10-byte file inside).
func mkProject(t *testing.T, dir string, markers []string, artifacts []string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, mk := range markers {
		if err := os.WriteFile(filepath.Join(dir, mk), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range artifacts {
		ad := filepath.Join(dir, a)
		if err := os.MkdirAll(ad, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ad, "blob"), []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func artifactByPath(arts []projectArtifact, path string) (projectArtifact, bool) {
	for _, a := range arts {
		if a.Path == path {
			return a, true
		}
	}
	return projectArtifact{}, false
}

func TestFindArtifacts(t *testing.T) {
	root := t.TempDir()
	mkProject(t, filepath.Join(root, "rustproj"), []string{"Cargo.toml"}, []string{"target"})
	mkProject(t, filepath.Join(root, "goproj"), []string{"go.mod"}, []string{"vendor"})
	mkProject(t, filepath.Join(root, "pyproj"), []string{"pyproject.toml"}, []string{".venv"})
	// monorepo: root project + nested package, each with its own node_modules
	mkProject(t, filepath.Join(root, "mono"), []string{"package.json"}, []string{"node_modules"})
	mkProject(t, filepath.Join(root, "mono", "packages", "a"), []string{"package.json"}, []string{"node_modules"})
	// build/ WITHOUT a marker must not be reported
	mkProject(t, filepath.Join(root, "plain"), nil, nil)
	if err := os.MkdirAll(filepath.Join(root, "plain", "build"), 0o755); err != nil {
		t.Fatal(err)
	}

	arts := findArtifacts(root)

	wantPaths := []string{
		filepath.Join(root, "rustproj", "target"),
		filepath.Join(root, "goproj", "vendor"),
		filepath.Join(root, "pyproj", ".venv"),
		filepath.Join(root, "mono", "node_modules"),
		filepath.Join(root, "mono", "packages", "a", "node_modules"),
	}
	if len(arts) != len(wantPaths) {
		t.Fatalf("got %d artifacts, want %d: %+v", len(arts), len(wantPaths), arts)
	}
	for _, p := range wantPaths {
		a, ok := artifactByPath(arts, p)
		if !ok {
			t.Errorf("missing artifact %s", p)
			continue
		}
		if a.Kind != filepath.Base(p) {
			t.Errorf("artifact %s: Kind = %q, want %q", p, a.Kind, filepath.Base(p))
		}
	}
	if _, ok := artifactByPath(arts, filepath.Join(root, "plain", "build")); ok {
		t.Error("build/ without marker must not be reported")
	}
	// nested node_modules attributed to nested project, counted once
	nested, _ := artifactByPath(arts, filepath.Join(root, "mono", "packages", "a", "node_modules"))
	if nested.ProjectPath != filepath.Join(root, "mono", "packages", "a") {
		t.Errorf("nested artifact project = %q, want nearest marker dir", nested.ProjectPath)
	}
	// caution flag
	goVendor, _ := artifactByPath(arts, filepath.Join(root, "goproj", "vendor"))
	if !goVendor.Caution {
		t.Error("go vendor/ must carry the caution flag")
	}
	rust, _ := artifactByPath(arts, filepath.Join(root, "rustproj", "target"))
	if rust.Caution {
		t.Error("rust target/ must not carry the caution flag")
	}
}

func TestFindArtifactsIdleTime(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "app")
	mkProject(t, proj, []string{"package.json"}, []string{"node_modules"})
	gitDir := filepath.Join(proj, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "index"), []byte("g"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-300 * 24 * time.Hour)
	// sources are old; artifact and .git contents keep fresh mtimes and
	// must NOT count toward idle time
	if err := os.Chtimes(filepath.Join(proj, "package.json"), old, old); err != nil {
		t.Fatal(err)
	}

	arts := findArtifacts(root)
	if len(arts) != 1 {
		t.Fatalf("got %d artifacts, want 1", len(arts))
	}
	if arts[0].LastTouched.After(old.Add(24 * time.Hour)) {
		t.Errorf("LastTouched = %v, want ~%v (artifact/.git mtimes excluded)", arts[0].LastTouched, old)
	}
}

func TestSortProjectEntries(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Path: "a", Size: 10, ModTime: now.Add(-1 * time.Hour)},
		{Path: "b", Size: 30, ModTime: now.Add(-3 * time.Hour)},
		{Path: "c", Size: 20, ModTime: now.Add(-2 * time.Hour)},
	}
	sortProjectEntries(entries, false)
	if entries[0].Path != "b" || entries[1].Path != "c" || entries[2].Path != "a" {
		t.Errorf("by size: got %s,%s,%s want b,c,a", entries[0].Path, entries[1].Path, entries[2].Path)
	}
	sortProjectEntries(entries, true)
	if entries[0].Path != "b" || entries[1].Path != "c" || entries[2].Path != "a" {
		t.Errorf("by idle (oldest first): got %s,%s,%s want b,c,a", entries[0].Path, entries[1].Path, entries[2].Path)
	}
}

func TestIsIdleSafe(t *testing.T) {
	old := Entry{ModTime: time.Now().Add(-200 * 24 * time.Hour)}
	fresh := Entry{ModTime: time.Now().Add(-10 * 24 * time.Hour)}
	zero := Entry{}
	if !isIdleSafe(old) {
		t.Error("200-day-old project must be idle-safe")
	}
	if isIdleSafe(fresh) {
		t.Error("10-day-old project must not be idle-safe")
	}
	if isIdleSafe(zero) {
		t.Error("zero ModTime must not be idle-safe")
	}
}
