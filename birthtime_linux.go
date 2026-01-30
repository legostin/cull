package main

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// extractBirthTime returns the file creation time on Linux via statx (kernel 4.11+).
// Falls back to zero time on older kernels or unsupported filesystems.
func extractBirthTime(path string, info os.FileInfo) time.Time {
	_ = info
	var stx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME, &stx)
	if err != nil {
		return time.Time{}
	}
	if stx.Mask&unix.STATX_BTIME == 0 {
		return time.Time{}
	}
	return time.Unix(int64(stx.Btime.Sec), int64(stx.Btime.Nsec))
}
