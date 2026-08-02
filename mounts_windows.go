//go:build windows

package main

// mountPointsUnder is a no-op on Windows: the deep scan performs no
// filesystem-boundary checks there (same as before).
func mountPointsUnder(root string) map[string]bool {
	return nil
}
