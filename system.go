package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// systemPathPrefixes returns OS-managed locations whose contents belong to
// the system or to running applications (temp clones, containers, runtime
// state). cull marks them with a badge and always asks before deleting,
// even with -y: files there vanish on their own or are in active use.
func systemPathPrefixes(home string) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/private/var/folders", "/var/folders",
			"/private/tmp", "/tmp",
			"/private/var/db", "/var/db",
			"/System", "/usr/lib", "/usr/bin", "/usr/sbin", "/usr/libexec",
			"/bin", "/sbin", "/cores",
			filepath.Join(home, "Library", "Containers"),
			filepath.Join(home, "Library", "Group Containers"),
		}
	case "linux":
		return []string{
			"/proc", "/sys", "/dev", "/run", "/boot",
			"/tmp", "/var/run", "/var/lib/dpkg", "/var/lib/rpm",
			"/usr/lib", "/usr/bin", "/usr/sbin", "/bin", "/sbin", "/lib",
		}
	case "windows":
		return []string{
			`C:\Windows`, `C:\Program Files`, `C:\Program Files (x86)`,
		}
	}
	return nil
}

var (
	systemPrefixesOnce sync.Once
	systemPrefixes     []string
)

// isSystemPath reports whether p lives under an OS-managed location.
func isSystemPath(p string) bool {
	systemPrefixesOnce.Do(func() {
		home, _ := os.UserHomeDir()
		for _, pref := range systemPathPrefixes(home) {
			systemPrefixes = append(systemPrefixes, strings.TrimSuffix(pref, string(filepath.Separator))+string(filepath.Separator))
		}
	})
	for _, pref := range systemPrefixes {
		if strings.HasPrefix(p, pref) || p+string(filepath.Separator) == pref {
			return true
		}
	}
	return false
}

// anySystemPath reports whether any of the given paths is system-managed.
func anySystemPath(paths map[string]bool) bool {
	for p := range paths {
		if isSystemPath(p) {
			return true
		}
	}
	return false
}
