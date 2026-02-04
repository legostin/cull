package main

import (
	"container/heap"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
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
	IsSymlink  bool      // true if the entry is a symbolic link
	ModTime    time.Time // last modification time
	CreateTime time.Time // birth / creation time (macOS)

	// Interned directory prefix ID (session-local, not persisted in cache)
	DirID uint32
}

// --- Cross-run disk cache ---

// dirCache holds cached scan results for a directory.
type dirCache struct {
	Path     string
	ModTime  time.Time
	Entries  []Entry
	CachedAt time.Time
}

// cacheDir returns the cache directory path, creating it if needed.
func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".cache", "cull")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// cachePath returns the gob cache file path for a given directory.
func cachePath(dirPath string) string {
	cd := cacheDir()
	if cd == "" {
		return ""
	}
	h := crc32.ChecksumIEEE([]byte(dirPath))
	return filepath.Join(cd, fmt.Sprintf("%08x.gob", h))
}

// loadDirCache attempts to load cached entries for the given directory.
// Returns entries and true on cache hit, nil and false on miss.
func loadDirCache(dirPath string) ([]Entry, bool) {
	cp := cachePath(dirPath)
	if cp == "" {
		return nil, false
	}

	// Get current directory mtime
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, false
	}
	dirMtime := info.ModTime()

	f, err := os.Open(cp)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var dc dirCache
	if err := gob.NewDecoder(f).Decode(&dc); err != nil {
		return nil, false
	}

	// Validate: path must match and directory must not have been modified
	if dc.Path != dirPath || !dc.ModTime.Equal(dirMtime) {
		return nil, false
	}

	return dc.Entries, true
}

// saveDirCache persists scanned entries to the disk cache.
func saveDirCache(dirPath string, entries []Entry) {
	cp := cachePath(dirPath)
	if cp == "" {
		return
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		return
	}

	dc := dirCache{
		Path:     dirPath,
		ModTime:  info.ModTime(),
		Entries:  entries,
		CachedAt: time.Now(),
	}

	tmp := cp + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	if err := gob.NewEncoder(f).Encode(&dc); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	f.Close()
	_ = os.Rename(tmp, cp)
}

// quickScanDoneMsg is sent when the quick (non-recursive) scan completes.
type quickScanDoneMsg struct {
	entries []Entry
	path    string
}

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
// Uses disk cache when available.
func quickScanDir(path string) ([]Entry, error) {
	if cached, ok := loadDirCache(path); ok {
		return cached, nil
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(dirEntries)+1)
	for _, de := range dirEntries {
		fullPath := filepath.Join(path, de.Name())
		isSymlink := de.Type()&os.ModeSymlink != 0

		entry := Entry{
			Name:      de.Name(),
			Path:      fullPath,
			IsSymlink: isSymlink,
		}

		if isSymlink {
			// Resolve symlink target to determine if it's a directory
			targetInfo, err := os.Stat(fullPath)
			if err == nil {
				entry.IsDir = targetInfo.IsDir()
				entry.ModTime = targetInfo.ModTime()
				if eagerBirthTime {
					entry.CreateTime = extractBirthTime(fullPath, targetInfo)
				}
			}
			// Symlinks contribute zero size
			entry.Size = 0
			entry.Sized = true
		} else {
			entry.IsDir = de.IsDir()
			info, infoErr := de.Info()
			if infoErr == nil {
				entry.ModTime = info.ModTime()
				if !de.IsDir() {
					entry.Size = info.Size()
				}
				if eagerBirthTime {
					entry.CreateTime = extractBirthTime(fullPath, info)
				}
			}
			if !de.IsDir() {
				entry.Sized = true
			}
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

	saveDirCache(path, entries)

	return entries, nil
}

// fillMissingBirthTimes populates CreateTime for entries that have a zero value.
// Used on platforms where birth time is not gathered eagerly during quickScanDir.
func fillMissingBirthTimes(entries []Entry) {
	for i := range entries {
		if entries[i].IsParent || !entries[i].CreateTime.IsZero() {
			continue
		}
		info, err := os.Stat(entries[i].Path)
		if err != nil {
			continue
		}
		entries[i].CreateTime = extractBirthTime(entries[i].Path, info)
	}
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

// --- Deep scanner: progressive top-N largest ---

// deepScanMsg delivers a streaming snapshot of the top-N largest files
// and accumulated first-level directory sizes.
type deepScanMsg struct {
	rootPath     string
	entries      []Entry          // current top-N snapshot
	dirSizes     map[string]int64 // first-level dir -> accumulated size
	scanningDirs map[string]bool  // set of first-level dir paths currently being scanned
	done         bool             // true when walk is complete
}

// entryHeap is a min-heap of Entry by Size, for top-N largest.
type entryHeap []Entry

func (h entryHeap) Len() int            { return len(h) }
func (h entryHeap) Less(i, j int) bool   { return h[i].Size < h[j].Size }
func (h entryHeap) Swap(i, j int)        { h[i], h[j] = h[j], h[i] }
func (h *entryHeap) Push(x interface{})  { *h = append(*h, x.(Entry)) }
func (h *entryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// snapshotHeap extracts a sorted (size desc) copy of the heap contents.
// If fillCreateTime is true, extractBirthTime is called for each entry
// (deferred from the hot loop to avoid unnecessary syscalls).
func snapshotHeap(h *entryHeap, fillCreateTime bool) []Entry {
	entries := make([]Entry, h.Len())
	copy(entries, *h)
	if fillCreateTime {
		for i := range entries {
			info, err := os.Stat(entries[i].Path)
			if err == nil {
				entries[i].CreateTime = extractBirthTime(entries[i].Path, info)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})
	return entries
}

// startDeepScan spawns a worker pool that walks first-level directories in
// parallel and sends periodic top-N snapshots through the returned channel.
// It also accumulates sizes for first-level subdirectories (for the BROWSE tab).
func startDeepScan(root string, topN int, firstLevelDirs []string) chan deepScanMsg {
	if topN <= 0 {
		topN = 1000
	}

	ch := make(chan deepScanMsg, 2)

	// Get the device ID of the root to skip cross-device mounts (e.g. /proc, /sys).
	rootDev, hasRootDev := deviceID(root)

	go func() {
		defer close(ch)

		// Shared state protected by mu.
		var mu sync.Mutex
		h := &entryHeap{}
		heap.Init(h)
		dirSizes := make(map[string]int64, len(firstLevelDirs))
		scanningDirs := make(map[string]bool, len(firstLevelDirs))

		// --- Phase 1: scan root-level files (not inside any subdir) ---
		rootEntries, _ := os.ReadDir(root)
		for _, de := range rootEntries {
			if de.IsDir() || de.Type()&os.ModeSymlink != 0 {
				continue
			}
			path := filepath.Join(root, de.Name())
			info, err := de.Info()
			if err != nil {
				continue
			}
			size := info.Size()
			if h.Len() < topN {
				heap.Push(h, Entry{
					Name:    de.Name(),
					Path:    path,
					Size:    size,
					Sized:   true,
					ModTime: info.ModTime(),
				})
			} else if size > (*h)[0].Size {
				heap.Pop(h)
				heap.Push(h, Entry{
					Name:    de.Name(),
					Path:    path,
					Size:    size,
					Sized:   true,
					ModTime: info.ModTime(),
				})
			}
		}

		// --- Phase 2: parallel walk of first-level directories ---
		numWorkers := runtime.NumCPU()
		if numWorkers > len(firstLevelDirs) {
			numWorkers = len(firstLevelDirs)
		}
		if numWorkers < 1 {
			numWorkers = 1
		}

		work := make(chan string, len(firstLevelDirs))
		for _, d := range firstLevelDirs {
			work <- d
		}
		close(work)

		// Dedicated snapshot ticker goroutine.
		const snapshotInterval = 200 * time.Millisecond
		tickerDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(snapshotInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					mu.Lock()
					snap := make(map[string]int64, len(dirSizes))
					for k, v := range dirSizes {
						snap[k] = v
					}
					sdSnap := make(map[string]bool, len(scanningDirs))
					for k := range scanningDirs {
						sdSnap[k] = true
					}
					entries := snapshotHeap(h, false)
					mu.Unlock()
					ch <- deepScanMsg{
						rootPath:     root,
						entries:      entries,
						dirSizes:     snap,
						scanningDirs: sdSnap,
					}
				case <-tickerDone:
					return
				}
			}
		}()

		var wg sync.WaitGroup
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go func() {
				defer wg.Done()
				for dirPath := range work {
					mu.Lock()
					scanningDirs[dirPath] = true
					mu.Unlock()

					_ = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
						if err != nil {
							return nil
						}

						// Skip symlinks entirely — they don't contribute to real disk usage.
						if d.Type()&os.ModeSymlink != 0 {
							return nil
						}

						if d.IsDir() {
							// Skip directories on different filesystems.
							if hasRootDev && path != dirPath {
								if dev, ok := deviceID(path); ok && dev != rootDev {
									return filepath.SkipDir
								}
							}
							return nil
						}

						info, err := d.Info()
						if err != nil {
							return nil
						}
						size := info.Size()

						mu.Lock()
						dirSizes[dirPath] += size

						if h.Len() < topN {
							heap.Push(h, Entry{
								Name:    d.Name(),
								Path:    path,
								Size:    size,
								Sized:   true,
								ModTime: info.ModTime(),
							})
						} else if size > (*h)[0].Size {
							heap.Pop(h)
							heap.Push(h, Entry{
								Name:    d.Name(),
								Path:    path,
								Size:    size,
								Sized:   true,
								ModTime: info.ModTime(),
							})
						}
						mu.Unlock()

						return nil
					})

					mu.Lock()
					delete(scanningDirs, dirPath)
					mu.Unlock()
				}
			}()
		}

		wg.Wait()
		close(tickerDone)

		// Final snapshot — fill CreateTime only for the final top-N entries.
		ch <- deepScanMsg{
			rootPath:     root,
			entries:      snapshotHeap(h, true),
			dirSizes:     dirSizes,
			scanningDirs: nil,
			done:         true,
		}
	}()

	return ch
}

// pollDeepScanCmd returns a tea.Cmd that reads the next message from the deep scan channel.
func pollDeepScanCmd(root string, ch chan deepScanMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return deepScanMsg{rootPath: root, done: true}
		}
		return msg
	}
}
