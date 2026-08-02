//go:build windows

package main

import "os"

// diskUsage returns the file's logical size on Windows; per-file allocation
// data is not exposed through os.FileInfo there.
func diskUsage(info os.FileInfo) int64 {
	return info.Size()
}

// fileID reports no identity on Windows — hardlink dedup is skipped.
func fileID(info os.FileInfo) (dev, ino uint64, ok bool) {
	return 0, 0, false
}
