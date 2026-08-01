package main

import (
	"strings"
	"time"
)

// artifactRule maps project marker files to build-artifact directory names
// expected in the same directory. Markers of the form "*.ext" match any
// entry name with that extension.
type artifactRule struct {
	Markers   []string
	Artifacts []string
}

// artifactRules is the curated v1 registry (see the PROJECTS tab spec).
var artifactRules = []artifactRule{
	{Markers: []string{"package.json"}, Artifacts: []string{"node_modules", ".next", ".nuxt", ".turbo", "dist", ".parcel-cache"}},
	{Markers: []string{"Cargo.toml"}, Artifacts: []string{"target"}},
	{Markers: []string{"go.mod"}, Artifacts: []string{"vendor"}},
	{Markers: []string{"pyproject.toml", "requirements.txt", "setup.py"}, Artifacts: []string{".venv", "venv", "__pycache__", ".tox", ".mypy_cache", ".pytest_cache", ".ruff_cache"}},
	{Markers: []string{"build.gradle", "build.gradle.kts"}, Artifacts: []string{"build", ".gradle"}},
	{Markers: []string{"pom.xml"}, Artifacts: []string{"target"}},
	{Markers: []string{"Podfile"}, Artifacts: []string{"Pods"}},
	{Markers: []string{"Package.swift"}, Artifacts: []string{".build"}},
	{Markers: []string{"CMakeLists.txt"}, Artifacts: []string{"build"}},
	{Markers: []string{"*.tf"}, Artifacts: []string{".terraform"}},
	{Markers: []string{"build.zig"}, Artifacts: []string{"zig-cache", "zig-out"}},
	{Markers: []string{"mix.exs"}, Artifacts: []string{"_build", "deps"}},
	{Markers: []string{"composer.json"}, Artifacts: []string{"vendor"}},
}

// cautionArtifacts marks artifact dirs whose deletion may break builds.
var cautionArtifacts = map[string]bool{
	"dist":   true,
	"vendor": true,
}

// idleSafeAfter is how long a project must be untouched before its
// artifacts are highlighted as safe to clean.
const idleSafeAfter = 180 * 24 * time.Hour

// isIdleSafe reports whether a PROJECTS entry belongs to a project idle
// long enough to highlight as safe to clean.
func isIdleSafe(e Entry) bool {
	return !e.ModTime.IsZero() && time.Since(e.ModTime) > idleSafeAfter
}

// matchArtifacts returns the union of artifact dir names whose marker files
// are present among the given directory entry names.
func matchArtifacts(names []string) map[string]bool {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	out := make(map[string]bool)
	for _, r := range artifactRules {
		matched := false
		for _, mk := range r.Markers {
			if strings.HasPrefix(mk, "*.") {
				suffix := mk[1:]
				for _, n := range names {
					if strings.HasSuffix(n, suffix) {
						matched = true
						break
					}
				}
			} else if nameSet[mk] {
				matched = true
			}
			if matched {
				break
			}
		}
		if matched {
			for _, a := range r.Artifacts {
				out[a] = true
			}
		}
	}
	return out
}
