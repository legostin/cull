//go:build !darwin

package main

// readDirBulk is only implemented on macOS (getattrlistbulk); other
// platforms use the portable ReadDir + per-file stat path.
func readDirBulk(dir string) ([]bulkRec, bool) {
	return nil, false
}
