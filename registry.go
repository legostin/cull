package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrashRecord represents a single item that was moved to system trash via cull.
type TrashRecord struct {
	OriginalPath string    `json:"original_path"`
	TrashPath    string    `json:"trash_path"`
	DeletedAt    time.Time `json:"deleted_at"`
	Size         int64     `json:"size"`
	IsDir        bool      `json:"is_dir"`
}

// TrashRegistry manages the trash.json file that tracks cull trash operations.
type TrashRegistry struct {
	Records []TrashRecord `json:"records"`
	mu      sync.Mutex
}

// trashRegistryPath returns the path to the trash registry JSON file.
func trashRegistryPath() string {
	cd := cacheDir()
	if cd == "" {
		return ""
	}
	return filepath.Join(cd, "trash.json")
}

// loadTrashRegistry reads the trash registry from disk.
// Returns an empty registry if the file doesn't exist or can't be read.
func loadTrashRegistry() (*TrashRegistry, error) {
	reg := &TrashRegistry{}
	p := trashRegistryPath()
	if p == "" {
		return reg, nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return reg, err
	}

	if err := json.Unmarshal(data, reg); err != nil {
		return &TrashRegistry{}, nil
	}
	return reg, nil
}

// Save writes the registry to disk atomically (tmp + rename).
func (r *TrashRegistry) Save() error {
	p := trashRegistryPath()
	if p == "" {
		return nil
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Add appends a record and saves.
func (r *TrashRegistry) Add(rec TrashRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Records = append(r.Records, rec)
	return r.Save()
}

// AddAll appends multiple records and saves once.
func (r *TrashRegistry) AddAll(recs []TrashRecord) error {
	if len(recs) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Records = append(r.Records, recs...)
	return r.Save()
}

// Remove deletes records matching the given original paths and saves.
func (r *TrashRegistry) Remove(originalPaths []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pathSet := make(map[string]bool, len(originalPaths))
	for _, p := range originalPaths {
		pathSet[p] = true
	}

	filtered := make([]TrashRecord, 0, len(r.Records))
	for _, rec := range r.Records {
		if !pathSet[rec.OriginalPath] {
			filtered = append(filtered, rec)
		}
	}
	r.Records = filtered
	return r.Save()
}

// Cleanup removes stale entries whose TrashPath no longer exists on disk.
// Returns the number of entries removed. Does not save; caller should call Save() if needed.
func (r *TrashRegistry) Cleanup() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	filtered := make([]TrashRecord, 0, len(r.Records))
	for _, rec := range r.Records {
		// Remove clearly invalid records (zero timestamp or empty original path)
		if rec.DeletedAt.IsZero() || rec.OriginalPath == "" {
			removed++
			continue
		}
		if rec.TrashPath == "" {
			// Windows entries without trash path — keep if we can't verify
			filtered = append(filtered, rec)
			continue
		}
		if _, err := os.Stat(rec.TrashPath); err == nil {
			filtered = append(filtered, rec)
		} else {
			removed++
		}
	}
	r.Records = filtered
	return removed
}

// ToEntries converts the registry records to []Entry for display in the history tab.
// Entries whose TrashPath no longer exists on disk are marked Stale.
func (r *TrashRegistry) ToEntries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := make([]Entry, 0, len(r.Records))
	for _, rec := range r.Records {
		stale := false
		if rec.TrashPath != "" {
			if _, err := os.Stat(rec.TrashPath); err != nil {
				stale = true
			}
		}
		entries = append(entries, Entry{
			Name:    filepath.Base(rec.OriginalPath),
			Path:    rec.OriginalPath,
			Size:    rec.Size,
			IsDir:   rec.IsDir,
			Sized:   true,
			ModTime: rec.DeletedAt,
			Stale:   stale,
		})
	}
	return entries
}

// LookupByOriginalPath returns a TrashRecord matching the given original path, if any.
func (r *TrashRegistry) LookupByOriginalPath(origPath string) (TrashRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rec := range r.Records {
		if rec.OriginalPath == origPath {
			return rec, true
		}
	}
	return TrashRecord{}, false
}
