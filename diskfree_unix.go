//go:build darwin || linux

package main

import "syscall"

// deviceID returns the filesystem device ID for the given path.
func deviceID(path string) (uint64, bool) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0, false
	}
	return uint64(stat.Dev), true
}

// diskFreeSpace returns available bytes on the filesystem containing path.
func diskFreeSpace(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}
