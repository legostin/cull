package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// moveToTrash moves a file or directory to the system trash.
func moveToTrash(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return trashDarwin(path)
	case "linux":
		return trashLinux(path)
	case "windows":
		return trashWindows(path)
	default:
		return fmt.Errorf("trash not supported on %s", runtime.GOOS)
	}
}

// trashDarwin moves an item to ~/.Trash/ on macOS.
func trashDarwin(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
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

	return os.Rename(path, dest)
}

// trashLinux moves an item to the XDG trash on Linux.
func trashLinux(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	trashFiles := filepath.Join(home, ".local", "share", "Trash", "files")
	trashInfo := filepath.Join(home, ".local", "share", "Trash", "info")

	if err := os.MkdirAll(trashFiles, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(trashInfo, 0o700); err != nil {
		return err
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
		return err
	}
	infoContent := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		absPath, time.Now().Format("2006-01-02T15:04:05"))
	infoPath := filepath.Join(trashInfo, base+".trashinfo")
	if err := os.WriteFile(infoPath, []byte(infoContent), 0o600); err != nil {
		return err
	}

	return os.Rename(path, dest)
}

// trashWindows moves an item to the Recycle Bin on Windows using SHFileOperation.
func trashWindows(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return shellMoveToRecycleBin(absPath)
}
