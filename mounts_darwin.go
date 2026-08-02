//go:build darwin

package main

import (
	"strings"
	"syscall"
)

// mountPointsUnder returns mount point paths strictly below root. Used to
// detect filesystem boundaries during the deep scan without a stat call per
// directory.
func mountPointsUnder(root string) map[string]bool {
	n, err := syscall.Getfsstat(nil, 2 /* MNT_NOWAIT */)
	if err != nil || n <= 0 {
		return nil
	}
	buf := make([]syscall.Statfs_t, n)
	n, err = syscall.Getfsstat(buf, 2)
	if err != nil {
		return nil
	}
	out := make(map[string]bool)
	prefix := strings.TrimSuffix(root, "/") + "/"
	for _, fs := range buf[:n] {
		mp := cstr(fs.Mntonname[:])
		if strings.HasPrefix(mp, prefix) {
			out[mp] = true
		}
	}
	return out
}

func cstr(b []int8) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		out = append(out, byte(c))
	}
	return string(out)
}
