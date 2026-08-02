package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

// sumPathsSize walks all paths and sums the disk usage of regular files,
// counting each hardlinked inode once. Errors are skipped.
func sumPathsSize(paths []string) int64 {
	var total int64
	seen := make(map[[2]uint64]bool)
	for _, p := range paths {
		_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.Type().IsRegular() {
				if info, err := d.Info(); err == nil {
					if dev, ino, ok := fileID(info); ok {
						if seen[[2]uint64{dev, ino}] {
							return nil
						}
						seen[[2]uint64{dev, ino}] = true
					}
					total += diskUsage(info)
				}
			}
			return nil
		})
	}
	return total
}

// cachesLoadedMsg is sent when the CACHES tab scan completes.
type cachesLoadedMsg struct {
	entries    []Entry
	pathGroups map[string][]string
}

// cacheSizeMsg carries the computed size of one cache row.
// ok=false means the row must be dropped (e.g. docker df failed).
type cacheSizeMsg struct {
	path string
	size int64
	ok   bool
}

// dockerPruneDoneMsg is sent after docker system prune succeeds.
type dockerPruneDoneMsg struct {
	reclaimed int64
}

// dockerPruneErrMsg is sent when docker system prune fails.
type dockerPruneErrMsg struct {
	err error
}

// loadCachesCmd scans the platform cache registry and the docker CLI.
func loadCachesCmd() tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return cachesLoadedMsg{}
		}
		hits := scanCaches(platformCacheDefs(), home)
		entries := make([]Entry, 0, len(hits)+1)
		groups := make(map[string][]string, len(hits))
		for _, h := range hits {
			primary := h.Paths[0]
			entries = append(entries, Entry{Name: h.Def.Name, Path: primary, IsDir: true})
			groups[primary] = h.Paths
		}
		if dockerAvailable() {
			entries = append(entries, Entry{Name: "Docker (system prune -a)", Path: dockerEntryPath})
		}
		if n := len(tmSnapshotDates()); n > 0 {
			entries = append(entries, Entry{
				Name:  fmt.Sprintf("Time Machine local snapshots (%d)", n),
				Path:  tmSnapEntryPath,
				Sized: true,
			})
		}
		return cachesLoadedMsg{entries: entries, pathGroups: groups}
	}
}

// cacheSizeCmd computes the total size of one cache row in the background.
func cacheSizeCmd(primary string, paths []string) tea.Cmd {
	return func() tea.Msg {
		return cacheSizeMsg{path: primary, size: sumPathsSize(paths), ok: true}
	}
}

// dockerSizeCmd queries docker reclaimable space; on failure the row is dropped.
func dockerSizeCmd() tea.Cmd {
	return func() tea.Msg {
		size, err := dockerReclaimable()
		if err != nil {
			return cacheSizeMsg{path: dockerEntryPath, ok: false}
		}
		return cacheSizeMsg{path: dockerEntryPath, size: size, ok: true}
	}
}

// dockerPruneCmd runs docker system prune -a -f.
func dockerPruneCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("docker", "system", "prune", "-a", "-f").CombinedOutput()
		if err != nil {
			return dockerPruneErrMsg{err: fmt.Errorf("docker prune: %v: %s", err, bytes.TrimSpace(out))}
		}
		return dockerPruneDoneMsg{reclaimed: parseDockerPruneOutput(string(out))}
	}
}

// tmSnapEntryPath is the sentinel Entry.Path for the Time Machine local
// snapshots row on the CACHES tab (darwin only).
const tmSnapEntryPath = "tmsnapshots://delete"

// parseTMSnapshotDates extracts deletable Time Machine snapshot dates from
// `tmutil listlocalsnapshots /` output. OS-update snapshots
// (com.apple.os.update-*) are system-managed and excluded.
func parseTMSnapshotDates(out []byte) []string {
	var dates []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "com.apple.TimeMachine."
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		d := strings.TrimPrefix(line, prefix)
		d = strings.TrimSuffix(d, ".local")
		if d != "" {
			dates = append(dates, d)
		}
	}
	return dates
}

// tmSnapshotDates lists deletable local Time Machine snapshots.
func tmSnapshotDates() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if _, err := exec.LookPath("tmutil"); err != nil {
		return nil
	}
	out, err := exec.Command("tmutil", "listlocalsnapshots", "/").Output()
	if err != nil {
		return nil
	}
	return parseTMSnapshotDates(out)
}

// tmSnapDoneMsg is sent after snapshot deletion completes.
type tmSnapDoneMsg struct {
	deleted int
}

// tmSnapErrMsg is sent when snapshot deletion fails.
type tmSnapErrMsg struct {
	err error
}

// tmSnapDeleteCmd deletes all local Time Machine snapshots one by one.
func tmSnapDeleteCmd(dates []string) tea.Cmd {
	return func() tea.Msg {
		deleted := 0
		for _, d := range dates {
			out, err := exec.Command("tmutil", "deletelocalsnapshots", d).CombinedOutput()
			if err != nil {
				return tmSnapErrMsg{err: fmt.Errorf(
					"tmutil deletelocalsnapshots %s: %v: %s (try: sudo cull)",
					d, err, bytes.TrimSpace(out))}
			}
			deleted++
		}
		return tmSnapDoneMsg{deleted: deleted}
	}
}
