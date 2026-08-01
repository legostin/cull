package main

import (
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
