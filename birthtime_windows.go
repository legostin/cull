package main

import (
	"os"
	"syscall"
	"time"
)

// extractBirthTime returns the file creation time on Windows.
func extractBirthTime(path string, info os.FileInfo) time.Time {
	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, sys.CreationTime.Nanoseconds())
	}
	return time.Time{}
}
