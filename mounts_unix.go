//go:build linux

package main

import (
	"os"
	"strings"
)

// mountPointsUnder returns mount point paths strictly below root. Used to
// detect filesystem boundaries during the deep scan without a stat call per
// directory.
func mountPointsUnder(root string) map[string]bool {
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return nil
	}
	out := make(map[string]bool)
	prefix := strings.TrimSuffix(root, "/") + "/"
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		mp := unescapeMount(fields[1])
		if strings.HasPrefix(mp, prefix) {
			out[mp] = true
		}
	}
	return out
}

// unescapeMount decodes the octal escapes /proc/mounts uses (\040 = space).
func unescapeMount(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			b.WriteByte((s[i+1]-'0')*64 + (s[i+2]-'0')*8 + (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
