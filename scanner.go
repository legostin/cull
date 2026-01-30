package main

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Entry represents a file or directory with its computed size.
type Entry struct {
	Name       string
	Path       string
	Size       int64
	IsDir      bool
	IsParent   bool      // true for the ".." entry
	Sized      bool      // true when the size has been fully computed
	ModTime    time.Time // last modification time
	CreateTime time.Time // birth / creation time (macOS)
}

// quickScanDoneMsg is sent when the quick (non-recursive) scan completes.
type quickScanDoneMsg struct {
	entries []Entry
	path    string
}

// dirSizeStartMsg signals that we're about to compute a directory's size.
type dirSizeStartMsg struct {
	path string
}

// dirSizeResultMsg delivers a computed directory size and a pre-scanned listing of that directory.
type dirSizeResultMsg struct {
	path       string
	size       int64
	subEntries []Entry // quick scan of the directory (for caching)
}

// scanCompleteMsg signals that all directory sizes have been computed.
type scanCompleteMsg struct{}

// scanErrMsg is sent when scanning fails.
type scanErrMsg struct {
	err error
}

// sortEntries sorts entries in place according to the given sort mode.
// The parent entry (..) is always kept first.
func sortEntries(entries []Entry, mode sortMode) {
	start := 0
	if len(entries) > 0 && entries[0].IsParent {
		start = 1
	}
	sub := entries[start:]
	switch mode {
	case sortSizeDesc:
		sort.Slice(sub, func(i, j int) bool {
			return sub[i].Size > sub[j].Size
		})
	case sortNameAsc:
		sort.Slice(sub, func(i, j int) bool {
			return sub[i].Name < sub[j].Name
		})
	case sortUpdatedDesc:
		sort.Slice(sub, func(i, j int) bool {
			return sub[i].ModTime.After(sub[j].ModTime)
		})
	case sortCreatedDesc:
		sort.Slice(sub, func(i, j int) bool {
			return sub[i].CreateTime.After(sub[j].CreateTime)
		})
	}
}

// quickScanDir reads a directory without recursion.
// Files get their real size; directories get Size = 0.
func quickScanDir(path string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, de := range dirEntries {
		fullPath := filepath.Join(path, de.Name())
		entry := Entry{
			Name:  de.Name(),
			Path:  fullPath,
			IsDir: de.IsDir(),
		}

		info, infoErr := de.Info()
		if infoErr == nil {
			entry.ModTime = info.ModTime()
			if !de.IsDir() {
				entry.Size = info.Size()
			}
			entry.CreateTime = extractBirthTime(fullPath, info)
		}
		if !de.IsDir() {
			entry.Sized = true
		}

		entries = append(entries, entry)
	}

	// Default sort: size descending
	sortEntries(entries, sortSizeDesc)

	// Prepend ".." entry if not at filesystem root
	parent := filepath.Dir(path)
	if parent != path {
		parentEntry := Entry{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			IsParent: true,
		}
		entries = append([]Entry{parentEntry}, entries...)
	}

	return entries, nil
}

// quickScanCmd returns a command that does a quick (non-recursive) directory listing.
func quickScanCmd(path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := quickScanDir(path)
		if err != nil {
			return scanErrMsg{err: err}
		}
		return quickScanDoneMsg{entries: entries, path: path}
	}
}

// computeDirSizeCmd returns a command that computes a single directory's size
// and also quick-scans its contents for caching.
func computeDirSizeCmd(path string) tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { return dirSizeStartMsg{path: path} },
		func() tea.Msg {
			size := dirSize(path)
			subEntries, _ := quickScanDir(path)
			return dirSizeResultMsg{path: path, size: size, subEntries: subEntries}
		},
	)
}

// dirSize recursively computes the total size of a directory.
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors, keep walking
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// diskFreeSpace returns available bytes on the filesystem containing path.
func diskFreeSpace(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}
