package main

// PathInterner deduplicates directory prefix strings by mapping them to uint32 IDs.
type PathInterner struct {
	pathToID map[string]uint32
	idToPath []string
}

// NewPathInterner creates a new PathInterner.
func NewPathInterner() *PathInterner {
	return &PathInterner{
		pathToID: make(map[string]uint32),
		// index 0 is reserved as "no interned path"
		idToPath: []string{""},
	}
}

// Intern returns a unique ID for the given path, creating one if needed.
func (pi *PathInterner) Intern(path string) uint32 {
	if id, ok := pi.pathToID[path]; ok {
		return id
	}
	id := uint32(len(pi.idToPath))
	pi.pathToID[path] = id
	pi.idToPath = append(pi.idToPath, path)
	return id
}

// Resolve returns the path string for the given ID.
func (pi *PathInterner) Resolve(id uint32) string {
	if int(id) >= len(pi.idToPath) {
		return ""
	}
	return pi.idToPath[id]
}
