//go:build !windows

package main

import (
	"os"
	"syscall"
)

// diskUsage returns the bytes a file actually occupies on disk (allocated
// blocks), not its logical length. Sparse files — Docker/OrbStack/UTM disk
// images, simulator disks — report logical sizes far beyond physical disk
// capacity; du semantics keep totals honest.
func diskUsage(info os.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks * 512
	}
	return info.Size()
}

// fileID identifies a file for hardlink deduplication. ok is false when the
// platform data is unavailable or the file has a single link (no dedup needed).
func fileID(info os.FileInfo) (dev, ino uint64, ok bool) {
	st, sok := info.Sys().(*syscall.Stat_t)
	if !sok || st.Nlink <= 1 {
		return 0, 0, false
	}
	return uint64(st.Dev), uint64(st.Ino), true
}
