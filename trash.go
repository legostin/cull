//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// moveToTrash moves a file or directory to the system trash.
// Returns the destination path in the trash and any error.
func moveToTrash(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	trashDir := filepath.Join(home, ".Trash")
	base := filepath.Base(path)
	dest := filepath.Join(trashDir, base)

	// Handle name collisions
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		ts := time.Now().Format("15-04-05")
		dest = filepath.Join(trashDir, fmt.Sprintf("%s %s%s", name, ts, ext))
	}

	return dest, os.Rename(path, dest)
}
