package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSystemPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	sys := []string{
		"/private/var/folders/ty/8t1xxn/T/x",
		"/var/folders/ty/8t1xxn/X/com.google.Chrome.code_sign_clone",
		"/private/tmp/foo",
		"/tmp/foo",
		"/System/Library",
		"/usr/lib/dyld",
		filepath.Join(home, "Library", "Containers", "com.apple.mail"),
	}
	for _, p := range sys {
		if !isSystemPath(p) {
			t.Errorf("isSystemPath(%q) = false, want true", p)
		}
	}
	notSys := []string{
		filepath.Join(home, "projects", "app"),
		filepath.Join(home, "Library", "Caches", "Google"), // caches are fair game
		"/usr/local/bin/tool",                              // user-managed prefix
		filepath.Join(home, "Downloads", "tmp"),
	}
	for _, p := range notSys {
		if isSystemPath(p) {
			t.Errorf("isSystemPath(%q) = true, want false", p)
		}
	}
}

func TestSystemPathDeleteAlwaysConfirms(t *testing.T) {
	m := newTestModel()
	m.skipConfirm = true // -y
	tab := m.tab()
	tab.allEntries = []Entry{{Name: "x", Path: "/private/var/folders/ty/abc/T/x", Size: 5, Sized: true}}
	tab.entries = append([]Entry{}, tab.allEntries...)
	tab.cursor = 0

	result, _ := m.Update(keyMsg("d"))
	rm := result.(model)
	if rm.mode != modeConfirm {
		t.Error("deleting a system-managed path must confirm even with -y")
	}
}
