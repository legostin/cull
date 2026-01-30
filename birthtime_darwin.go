package main

import (
	"os"
	"syscall"
	"time"
)

// extractBirthTime returns the file creation time on macOS.
func extractBirthTime(info os.FileInfo) time.Time {
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(sys.Birthtimespec.Sec, sys.Birthtimespec.Nsec)
	}
	return time.Time{}
}
