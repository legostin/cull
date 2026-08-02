package main

import "time"

// bulkRec is one directory entry as returned by readDirBulk.
type bulkRec struct {
	name      string
	isDir     bool
	isSymlink bool
	size      int64 // bytes freed if deleted (private size; excludes blocks shared with clones)
	mod       time.Time
	ino       uint64
	nlink     uint32
}
