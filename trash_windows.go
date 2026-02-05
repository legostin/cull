package main

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
)

const (
	foDelete          = 0x0003
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent         = 0x0004
	fofNoErrorUI      = 0x0400
)

// SHFILEOPSTRUCTW mirrors the Windows SHFILEOPSTRUCT.
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

// moveToTrash moves a file or directory to the Windows Recycle Bin.
// Returns empty string for the trash path since Windows doesn't expose it.
func moveToTrash(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// SHFileOperation requires double-null terminated string
	pathUTF16, err := syscall.UTF16FromString(absPath)
	if err != nil {
		return "", err
	}
	pathUTF16 = append(pathUTF16, 0) // extra null terminator

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &pathUTF16[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}

	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return "", syscall.Errno(ret)
	}
	return "", nil
}
