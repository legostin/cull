package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// moveToTrash moves a file or directory to the XDG trash on Linux.
// Returns the destination path in the trash and any error.
func moveToTrash(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	trashFiles := filepath.Join(home, ".local", "share", "Trash", "files")
	trashInfo := filepath.Join(home, ".local", "share", "Trash", "info")

	if err := os.MkdirAll(trashFiles, 0o700); err != nil {
		return "", err
	}
	if err := os.MkdirAll(trashInfo, 0o700); err != nil {
		return "", err
	}

	base := filepath.Base(path)
	dest := filepath.Join(trashFiles, base)

	// Handle name collisions
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		ts := time.Now().Format("15-04-05")
		dest = filepath.Join(trashFiles, fmt.Sprintf("%s %s%s", name, ts, ext))
		base = filepath.Base(dest)
	}

	// Write .trashinfo file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	infoContent := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		absPath, time.Now().Format("2006-01-02T15:04:05"))
	infoPath := filepath.Join(trashInfo, base+".trashinfo")
	if err := os.WriteFile(infoPath, []byte(infoContent), 0o600); err != nil {
		return "", err
	}

	return dest, os.Rename(path, dest)
}
