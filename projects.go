package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// projectArtifact is one build-artifact directory found under a project.
type projectArtifact struct {
	ProjectName string // base name of the project directory
	ProjectPath string
	Path        string // absolute path of the artifact dir
	Kind        string // artifact dir name, e.g. "node_modules"
	Caution     bool   // deletion may break builds (dist, vendor)
	LastTouched time.Time
}

// findArtifacts walks root and returns all project build artifacts.
// It never descends into artifact dirs, .git, or other filesystems
// (network mounts can hang and wake sleeping VMs).
func findArtifacts(root string) []projectArtifact {
	var out []projectArtifact
	walkProjects(root, mountPointsUnderFn(root), &out)
	// Project names are paths relative to the scan root, so nested marker
	// dirs with generic names (myapp/src-tauri, packages/core) stay
	// distinguishable in the list.
	for i := range out {
		if rel, err := filepath.Rel(root, out[i].ProjectPath); err == nil && rel != "." {
			out[i].ProjectName = rel
		}
	}
	return out
}

// walkProjects recursively scans dir, appends found artifacts to out and
// returns the max mtime of files in the subtree, excluding artifact dirs
// and .git. Mount points and unreadable dirs are skipped silently.
func walkProjects(dir string, mounts map[string]bool, out *[]projectArtifact) time.Time {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}
	}

	names := make([]string, len(entries))
	for i, de := range entries {
		names[i] = de.Name()
	}
	wanted := matchArtifacts(names)

	var own []projectArtifact
	var maxMtime time.Time
	for _, de := range entries {
		name := de.Name()
		if de.IsDir() {
			if name == ".git" {
				continue
			}
			if wanted[name] {
				own = append(own, projectArtifact{
					ProjectName: filepath.Base(dir),
					ProjectPath: dir,
					Path:        filepath.Join(dir, name),
					Kind:        name,
					Caution:     cautionArtifacts[name],
				})
				continue // sized separately; excluded from idle mtime
			}
			sub := filepath.Join(dir, name)
			if mounts[sub] {
				continue // different filesystem
			}
			if m := walkProjects(sub, mounts, out); m.After(maxMtime) {
				maxMtime = m
			}
			continue
		}
		if info, err := de.Info(); err == nil && info.ModTime().After(maxMtime) {
			maxMtime = info.ModTime()
		}
	}

	for i := range own {
		own[i].LastTouched = maxMtime
	}
	*out = append(*out, own...)
	return maxMtime
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

// projectsLoadedMsg is sent when the PROJECTS walk completes.
type projectsLoadedMsg struct {
	root      string // joined roots the walk ran from; stale results are discarded
	entries   []Entry
	artifacts map[string]projectArtifact // keyed by artifact path
}

// joinRoots builds the stale-detection key for a set of scan roots.
func joinRoots(roots []string) string {
	return strings.Join(roots, "\x00")
}

// projectSizeMsg carries the computed size of one artifact row.
type projectSizeMsg struct {
	path string
	size int64
}

// loadProjectsCmd walks the launch roots for project artifacts.
func loadProjectsCmd(roots []string) tea.Cmd {
	return func() tea.Msg {
		var all []projectArtifact
		for _, r := range roots {
			all = append(all, findArtifacts(r)...)
		}
		entries := make([]Entry, 0, len(all))
		meta := make(map[string]projectArtifact, len(all))
		for _, a := range all {
			entries = append(entries, Entry{
				Name:    a.ProjectName,
				Path:    a.Path,
				IsDir:   true,
				ModTime: a.LastTouched,
			})
			meta[a.Path] = a
		}
		return projectsLoadedMsg{root: joinRoots(roots), entries: entries, artifacts: meta}
	}
}

// projectSizeCmd computes the size of one artifact dir in the background.
func projectSizeCmd(path string) tea.Cmd {
	return func() tea.Msg {
		return projectSizeMsg{path: path, size: sumPathsSize([]string{path})}
	}
}

// sortProjectEntries orders PROJECTS rows by size desc, or by idle time
// (oldest LastTouched first) when byIdle is set.
func sortProjectEntries(entries []Entry, byIdle bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		if byIdle {
			return entries[i].ModTime.Before(entries[j].ModTime)
		}
		return entries[i].Size > entries[j].Size
	})
}
