//go:build darwin || linux

package main

import "syscall"

// diskFreeSpace returns available bytes on the filesystem containing path.
func diskFreeSpace(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}
