package main

import (
	"os"
	"time"
)

// extractBirthTime returns zero time on Linux (birth time not reliably available).
func extractBirthTime(info os.FileInfo) time.Time {
	_ = info
	return time.Time{}
}
