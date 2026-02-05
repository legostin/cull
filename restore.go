package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// restoreFromTrash moves a file from system trash back to its original location.
func restoreFromTrash(rec TrashRecord) error {
	if rec.TrashPath == "" {
		return fmt.Errorf("cannot restore: trash path unknown (Windows limitation)")
	}

	// Verify the trash file still exists
	if _, err := os.Stat(rec.TrashPath); err != nil {
		return fmt.Errorf("trash file no longer exists: %s", rec.TrashPath)
	}

	// Create parent directories if needed
	parentDir := filepath.Dir(rec.OriginalPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("cannot create parent directory: %w", err)
	}

	if err := os.Rename(rec.TrashPath, rec.OriginalPath); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	// On Linux, also remove the .trashinfo file
	if runtime.GOOS == "linux" {
		removeTrashInfo(rec.TrashPath)
	}

	return nil
}

// purgeFromTrash permanently deletes a file from system trash.
func purgeFromTrash(rec TrashRecord) error {
	if rec.TrashPath == "" {
		return fmt.Errorf("cannot purge: trash path unknown (Windows limitation)")
	}

	if err := os.RemoveAll(rec.TrashPath); err != nil {
		return fmt.Errorf("purge failed: %w", err)
	}

	// On Linux, also remove the .trashinfo file
	if runtime.GOOS == "linux" {
		removeTrashInfo(rec.TrashPath)
	}

	return nil
}

// removeTrashInfo removes the .trashinfo metadata file for a Linux XDG trash entry.
func removeTrashInfo(trashPath string) {
	// trashPath is like ~/.local/share/Trash/files/somefile
	// trashinfo is at ~/.local/share/Trash/info/somefile.trashinfo
	dir := filepath.Dir(trashPath)
	base := filepath.Base(trashPath)
	trashRoot := filepath.Dir(dir) // ~/.local/share/Trash

	// Only handle standard XDG trash layout
	if !strings.HasSuffix(dir, filepath.Join("Trash", "files")) {
		return
	}

	infoPath := filepath.Join(trashRoot, "info", base+".trashinfo")
	_ = os.Remove(infoPath)
}
