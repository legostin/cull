//go:build darwin

package main

import (
	"encoding/binary"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Not yet exported by x/sys: extended common attributes (bsd/sys/attr.h).
const (
	attrCmnextPrivatesize = 0x00000008 // ATTR_CMNEXT_PRIVATESIZE
	fsoptAttrCmnExtended  = 0x00000020 // FSOPT_ATTR_CMN_EXTENDED
)

// vnode object types (bsd/sys/vnode.h).
const (
	vreg = 1
	vdir = 2
	vlnk = 5
)

// readDirBulk lists a directory via getattrlistbulk: one syscall returns
// name, type, mtime, file id, link count and sizes for dozens of entries,
// replacing ReadDir + a stat per file. Files >= privateSizeThreshold get an
// exact freed-if-deleted size (APFS clone aware), so e.g. Chrome's
// code_sign_clone shows as ~0 instead of a full copy. Returns ok=false so
// callers can fall back to the portable path.
func readDirBulk(dir string) ([]bulkRec, bool) {
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, false
	}
	defer unix.Close(fd)

	al := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr: unix.ATTR_CMN_RETURNED_ATTRS | unix.ATTR_CMN_NAME |
			unix.ATTR_CMN_OBJTYPE | unix.ATTR_CMN_MODTIME | unix.ATTR_CMN_FILEID,
		Fileattr: unix.ATTR_FILE_LINKCOUNT | unix.ATTR_FILE_ALLOCSIZE,
		// PRIVATESIZE is intentionally NOT requested here: computing it per
		// file costs ~30% of scan time. Large files get it individually via
		// privateSizeOf below — small clones don't move the numbers.
	}

	var out []bulkRec
	buf := make([]byte, 128<<10)
	for {
		// x/sys exposes the syscall number but not a wrapper yet.
		rn, _, errno := syscall.Syscall6(unix.SYS_GETATTRLISTBULK,
			uintptr(fd), uintptr(unsafe.Pointer(&al)), uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)), uintptr(fsoptAttrCmnExtended), 0)
		if errno != 0 {
			return nil, false
		}
		n := int(rn)
		if n == 0 {
			return out, true
		}
		pos := 0
		for i := 0; i < n; i++ {
			reclen := int(binary.LittleEndian.Uint32(buf[pos:]))
			if reclen < 4 || pos+reclen > len(buf) {
				return nil, false
			}
			rec, ok := parseBulkRecord(buf[pos : pos+reclen])
			if !ok {
				return nil, false
			}
			// Clone-aware size for big files only (see privateSizeThreshold).
			if !rec.isDir && !rec.isSymlink && rec.size >= privateSizeThreshold {
				if ps, ok := privateSizeOf(dir + "/" + rec.name); ok {
					rec.size = ps
				}
			}
			out = append(out, rec)
			pos += reclen
		}
	}
}

// privateSizeThreshold: files at least this large get an exact
// freed-if-deleted size (APFS clone aware). Smaller clones are noise.
const privateSizeThreshold = 1 << 20

// privateSizeOf returns ATTR_CMNEXT_PRIVATESIZE for one path: the bytes a
// deletion would actually free, excluding blocks shared with clones.
func privateSizeOf(path string) (int64, bool) {
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return 0, false
	}
	al := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Forkattr:    attrCmnextPrivatesize,
	}
	var buf [12]byte // u32 length + i64 private size
	_, _, errno := syscall.Syscall6(unix.SYS_GETATTRLIST,
		uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&al)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(fsoptAttrCmnExtended|unix.FSOPT_NOFOLLOW), 0)
	if errno != 0 {
		return 0, false
	}
	if binary.LittleEndian.Uint32(buf[0:]) < 12 {
		return 0, false
	}
	return int64(binary.LittleEndian.Uint64(buf[4:])), true
}

// parseBulkRecord decodes one getattrlistbulk record. Attributes appear in
// canonical order (common, file, fork), each 4-byte aligned; presence is
// governed by the returned attribute_set_t.
func parseBulkRecord(rec []byte) (bulkRec, bool) {
	var r bulkRec
	p := 4 // skip the length field

	need := func(n int) bool { return p+n <= len(rec) }
	if !need(int(unsafe.Sizeof(unix.Attrlist{}))) && !need(20) {
		return r, false
	}
	// attribute_set_t: 5×uint32 (ATTR_CMN_RETURNED_ATTRS is always first)
	retCommon := binary.LittleEndian.Uint32(rec[p:])
	retFile := binary.LittleEndian.Uint32(rec[p+12:])
	retFork := binary.LittleEndian.Uint32(rec[p+16:])
	p += 20

	if retCommon&unix.ATTR_CMN_NAME != 0 {
		if !need(8) {
			return r, false
		}
		off := int(int32(binary.LittleEndian.Uint32(rec[p:])))
		ln := int(binary.LittleEndian.Uint32(rec[p+4:]))
		start := p + off
		if start < 0 || ln < 1 || start+ln > len(rec) {
			return r, false
		}
		r.name = string(rec[start : start+ln-1]) // NUL-terminated
		p += 8
	}
	if retCommon&unix.ATTR_CMN_OBJTYPE != 0 {
		if !need(4) {
			return r, false
		}
		switch binary.LittleEndian.Uint32(rec[p:]) {
		case vdir:
			r.isDir = true
		case vlnk:
			r.isSymlink = true
		}
		p += 4
	}
	if retCommon&unix.ATTR_CMN_MODTIME != 0 {
		if !need(16) {
			return r, false
		}
		sec := int64(binary.LittleEndian.Uint64(rec[p:]))
		nsec := int64(binary.LittleEndian.Uint64(rec[p+8:]))
		r.mod = time.Unix(sec, nsec)
		p += 16
	}
	if retCommon&unix.ATTR_CMN_FILEID != 0 {
		if !need(8) {
			return r, false
		}
		r.ino = binary.LittleEndian.Uint64(rec[p:])
		p += 8
	}
	var allocSize int64
	haveAlloc := false
	if retFile&unix.ATTR_FILE_LINKCOUNT != 0 {
		if !need(4) {
			return r, false
		}
		r.nlink = binary.LittleEndian.Uint32(rec[p:])
		p += 4
	}
	if retFile&unix.ATTR_FILE_ALLOCSIZE != 0 {
		if !need(8) {
			return r, false
		}
		allocSize = int64(binary.LittleEndian.Uint64(rec[p:]))
		haveAlloc = true
		p += 8
	}
	if retFork&attrCmnextPrivatesize != 0 {
		if !need(8) {
			return r, false
		}
		r.size = int64(binary.LittleEndian.Uint64(rec[p:]))
	} else if haveAlloc {
		r.size = allocSize
	}
	return r, true
}
