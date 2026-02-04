package main

import "golang.org/x/sys/windows"

// deviceID returns the filesystem device ID for the given path.
// On Windows, volume serial numbers serve a similar role; for the purpose of
// cross-device skip logic we use GetVolumeInformationByHandleW.
func deviceID(path string) (uint64, bool) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	h, err := windows.CreateFile(pathPtr, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return 0, false
	}
	return uint64(info.VolumeSerialNumber), true
}

// diskFreeSpace returns available bytes on the filesystem containing path.
func diskFreeSpace(path string) uint64 {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return 0
	}
	return freeBytesAvailable
}
