package main

import "testing"

func TestNewPathInterner(t *testing.T) {
	pi := NewPathInterner()
	if pi == nil {
		t.Fatal("NewPathInterner returned nil")
	}
	// Index 0 is reserved as "no interned path"
	if got := pi.Resolve(0); got != "" {
		t.Errorf("Resolve(0) = %q, want empty string", got)
	}
}

func TestIntern_SamePathSameID(t *testing.T) {
	pi := NewPathInterner()
	id1 := pi.Intern("/usr/local/bin")
	id2 := pi.Intern("/usr/local/bin")
	if id1 != id2 {
		t.Errorf("same path got different IDs: %d vs %d", id1, id2)
	}
}

func TestIntern_DifferentPathsDifferentIDs(t *testing.T) {
	pi := NewPathInterner()
	id1 := pi.Intern("/usr/local/bin")
	id2 := pi.Intern("/usr/local/lib")
	if id1 == id2 {
		t.Errorf("different paths got same ID: %d", id1)
	}
}

func TestResolve_ReturnsOriginalPath(t *testing.T) {
	pi := NewPathInterner()
	path := "/home/user/documents"
	id := pi.Intern(path)
	got := pi.Resolve(id)
	if got != path {
		t.Errorf("Resolve(%d) = %q, want %q", id, got, path)
	}
}

func TestResolve_OutOfRange(t *testing.T) {
	pi := NewPathInterner()
	if got := pi.Resolve(999); got != "" {
		t.Errorf("Resolve(999) = %q, want empty string", got)
	}
}

func TestIntern_MultiplePathsRoundTrip(t *testing.T) {
	pi := NewPathInterner()
	paths := []string{"/a", "/b", "/c/d", "/e/f/g"}
	ids := make([]uint32, len(paths))
	for i, p := range paths {
		ids[i] = pi.Intern(p)
	}
	for i, p := range paths {
		if got := pi.Resolve(ids[i]); got != p {
			t.Errorf("Resolve(%d) = %q, want %q", ids[i], got, p)
		}
	}
}
