package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// dockerAvailable reports whether the docker CLI is on PATH.
func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// parseDockerSize parses docker-style human sizes ("1.5GB (90%)", "500MB",
// "0B"). Docker uses decimal units. Returns 0 on any parse failure.
func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i >= 0 {
		s = s[:i] // strip trailing " (90%)"
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	mult := map[string]float64{"B": 1, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12}
	m, ok := mult[strings.ToUpper(s[i:])]
	if !ok {
		return 0
	}
	return int64(num * m)
}

// parseDockerDF sums the Reclaimable field over `docker system df --format
// '{{json .}}'` NDJSON output. Unparseable lines are skipped.
func parseDockerDF(out []byte) int64 {
	var total int64
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var row struct{ Reclaimable string }
		if err := json.Unmarshal(line, &row); err != nil {
			continue
		}
		total += parseDockerSize(row.Reclaimable)
	}
	return total
}

// dockerReclaimable queries docker for total reclaimable space.
func dockerReclaimable() (int64, error) {
	out, err := exec.Command("docker", "system", "df", "--format", "{{json .}}").Output()
	if err != nil {
		return 0, err
	}
	return parseDockerDF(out), nil
}

// parseDockerPruneOutput extracts the freed size from `docker system prune`
// output ("Total reclaimed space: 4.2GB"). Returns 0 if absent.
func parseDockerPruneOutput(out string) int64 {
	const marker = "Total reclaimed space:"
	i := strings.LastIndex(out, marker)
	if i < 0 {
		return 0
	}
	rest := out[i+len(marker):]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	return parseDockerSize(rest)
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
