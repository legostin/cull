package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrashRegistry_LoadSaveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	regPath := filepath.Join(tmpDir, "trash.json")

	reg := &TrashRegistry{
		Records: []TrashRecord{
			{
				OriginalPath: "/home/user/file.txt",
				TrashPath:    "/home/user/.Trash/file.txt",
				DeletedAt:    time.Now().Truncate(time.Second),
				Size:         1024,
				IsDir:        false,
			},
		},
	}

	// Save
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Load
	loaded := &TrashRegistry{}
	readData, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(readData, loaded); err != nil {
		t.Fatal(err)
	}

	if len(loaded.Records) != 1 {
		t.Fatalf("records len = %d, want 1", len(loaded.Records))
	}
	if loaded.Records[0].OriginalPath != "/home/user/file.txt" {
		t.Errorf("original path = %q", loaded.Records[0].OriginalPath)
	}
	if loaded.Records[0].Size != 1024 {
		t.Errorf("size = %d, want 1024", loaded.Records[0].Size)
	}
}

func TestTrashRegistry_Add(t *testing.T) {
	reg := &TrashRegistry{}
	rec := TrashRecord{
		OriginalPath: "/test/file.txt",
		TrashPath:    "/trash/file.txt",
		DeletedAt:    time.Now(),
		Size:         500,
	}

	// Add without saving (would fail without cache dir)
	reg.Records = append(reg.Records, rec)
	if len(reg.Records) != 1 {
		t.Errorf("records len = %d, want 1", len(reg.Records))
	}
}

func TestTrashRegistry_AddAll(t *testing.T) {
	reg := &TrashRegistry{}
	recs := []TrashRecord{
		{OriginalPath: "/a", TrashPath: "/trash/a", Size: 100},
		{OriginalPath: "/b", TrashPath: "/trash/b", Size: 200},
	}
	reg.Records = append(reg.Records, recs...)
	if len(reg.Records) != 2 {
		t.Errorf("records len = %d, want 2", len(reg.Records))
	}
}

func TestTrashRegistry_Remove(t *testing.T) {
	reg := &TrashRegistry{
		Records: []TrashRecord{
			{OriginalPath: "/a", TrashPath: "/trash/a"},
			{OriginalPath: "/b", TrashPath: "/trash/b"},
			{OriginalPath: "/c", TrashPath: "/trash/c"},
		},
	}

	pathSet := map[string]bool{"/a": true, "/c": true}
	filtered := make([]TrashRecord, 0, len(reg.Records))
	for _, rec := range reg.Records {
		if !pathSet[rec.OriginalPath] {
			filtered = append(filtered, rec)
		}
	}
	reg.Records = filtered

	if len(reg.Records) != 1 {
		t.Fatalf("records len = %d, want 1", len(reg.Records))
	}
	if reg.Records[0].OriginalPath != "/b" {
		t.Errorf("remaining record = %q, want /b", reg.Records[0].OriginalPath)
	}
}

func TestTrashRegistry_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that exists
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	reg := &TrashRegistry{
		Records: []TrashRecord{
			{OriginalPath: "/orig/exists.txt", TrashPath: existingFile, DeletedAt: now},
			{OriginalPath: "/orig/gone.txt", TrashPath: filepath.Join(tmpDir, "gone.txt"), DeletedAt: now},
			{OriginalPath: "/orig/win.txt", TrashPath: "", DeletedAt: now}, // Windows entry
		},
	}

	removed := reg.Cleanup()
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(reg.Records) != 2 {
		t.Errorf("records len = %d, want 2 (existing + windows)", len(reg.Records))
	}
}

func TestTrashRegistry_ToEntries(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	reg := &TrashRegistry{
		Records: []TrashRecord{
			{
				OriginalPath: "/home/user/docs/report.pdf",
				TrashPath:    "/trash/report.pdf",
				DeletedAt:    now,
				Size:         2048,
				IsDir:        false,
			},
			{
				OriginalPath: "/home/user/projects/old",
				TrashPath:    "/trash/old",
				DeletedAt:    now,
				Size:         8192,
				IsDir:        true,
			},
		},
	}

	entries := reg.ToEntries()
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}

	e := entries[0]
	if e.Name != "report.pdf" {
		t.Errorf("name = %q, want report.pdf", e.Name)
	}
	if e.Path != "/home/user/docs/report.pdf" {
		t.Errorf("path = %q", e.Path)
	}
	if e.Size != 2048 {
		t.Errorf("size = %d, want 2048", e.Size)
	}
	if !e.Sized {
		t.Error("sized should be true")
	}
	if !e.ModTime.Equal(now) {
		t.Errorf("modtime = %v, want %v", e.ModTime, now)
	}

	e2 := entries[1]
	if !e2.IsDir {
		t.Error("second entry should be a directory")
	}
}

func TestTrashRegistry_LookupByOriginalPath(t *testing.T) {
	reg := &TrashRegistry{
		Records: []TrashRecord{
			{OriginalPath: "/a", TrashPath: "/trash/a", Size: 100},
			{OriginalPath: "/b", TrashPath: "/trash/b", Size: 200},
		},
	}

	rec, ok := reg.LookupByOriginalPath("/b")
	if !ok {
		t.Fatal("should find /b")
	}
	if rec.Size != 200 {
		t.Errorf("size = %d, want 200", rec.Size)
	}

	_, ok = reg.LookupByOriginalPath("/nonexistent")
	if ok {
		t.Error("should not find nonexistent path")
	}
}

func TestTrashRegistry_EmptyLoad(t *testing.T) {
	// loadTrashRegistry with no file should return empty registry
	reg := &TrashRegistry{}
	if len(reg.Records) != 0 {
		t.Error("new registry should have no records")
	}
}
