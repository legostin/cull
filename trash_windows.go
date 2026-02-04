package main

import (
	"syscall"
	"unsafe"
)

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
)

const (
	foDelete = 0x0003
	fofAllowUndo = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent = 0x0004
	fofNoErrorUI = 0x0400
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

// shellMoveToRecycleBin sends a file/directory to the Windows Recycle Bin.
func shellMoveToRecycleBin(path string) error {
	// SHFileOperation requires double-null terminated string
	pathUTF16, err := syscall.UTF16FromString(path)
	if err != nil {
		return err
	}
	pathUTF16 = append(pathUTF16, 0) // extra null terminator

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &pathUTF16[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}

	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}
