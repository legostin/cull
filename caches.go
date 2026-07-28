package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// cacheKind distinguishes plain directory caches from special entries.
type cacheKind int

const (
	cacheKindDir    cacheKind = iota // directory cache, cleared by file deletion
	cacheKindDocker                  // special entry, cleared via docker system prune
)

// dockerEntryPath is the sentinel Entry.Path for the Docker row on the CACHES tab.
const dockerEntryPath = "docker://prune"

// cacheDef describes one known application cache location.
// Paths may contain "~" (expanded to home) and glob metacharacters
// (expanded via filepath.Glob; single-star segments only — Glob has no **).
type cacheDef struct {
	Name  string
	Paths []string
	Kind  cacheKind
}

// cacheHit is a cacheDef whose paths (at least one) exist on disk.
type cacheHit struct {
	Def   cacheDef
	Paths []string
}

// resolveCachePaths expands ~ and glob patterns, keeping only existing paths.
func resolveCachePaths(patterns []string, home string) []string {
	var out []string
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "~") {
			p = filepath.Join(home, p[1:])
		}
		if strings.ContainsAny(p, "*?[") {
			matches, err := filepath.Glob(p)
			if err != nil {
				continue
			}
			for _, match := range matches {
				if _, err := os.Stat(match); err == nil {
					out = append(out, match)
				}
			}
			continue
		}
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// scanCaches resolves every cacheDir def and returns those with existing paths.
func scanCaches(defs []cacheDef, home string) []cacheHit {
	var hits []cacheHit
	for _, d := range defs {
		if d.Kind != cacheKindDir {
			continue
		}
		paths := resolveCachePaths(d.Paths, home)
		if len(paths) > 0 {
			hits = append(hits, cacheHit{Def: d, Paths: paths})
		}
	}
	return hits
}

// sumPathsSize walks all paths and sums regular-file sizes. Errors are skipped.
func sumPathsSize(paths []string) int64 {
	var total int64
	for _, p := range paths {
		_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.Type().IsRegular() {
				if info, err := d.Info(); err == nil {
					total += info.Size()
				}
			}
			return nil
		})
	}
	return total
}
