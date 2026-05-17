//go:build windows

package scan

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// isReparsePoint reports whether the given file is a Windows reparse
// point (symlink, junction, mount point, OneDrive placeholder, etc.).
// Go's os.FileInfo.Mode() only marks reparse-tag-symlink files as
// os.ModeSymlink; other reparse types — including the very common
// directory junctions — slip through unflagged. Walking into them
// causes double-counting (e.g. C:\Documents and Settings → C:\Users).
func isReparsePoint(info os.FileInfo) bool {
	sys, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	return sys.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// fileUniqueID returns a stable identifier for the underlying file (not
// the path). On NTFS, this is the volume serial number plus the file
// index — two hard links to the same data produce the same ID.
//
// The lookup requires opening the file. We use FILE_READ_ATTRIBUTES (no
// data read), share everything to avoid colliding with in-use files, and
// FILE_FLAG_BACKUP_SEMANTICS so directories can also be opened if ever
// needed. If the open fails (locked, missing perms) we return ok=false
// so the caller falls back to counting the file as unique.
func fileUniqueID(path string, info os.FileInfo) (string, bool) {
	if info.IsDir() {
		return "", false
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	handle, err := windows.CreateFile(
		pathPtr,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(handle)

	var bhfi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &bhfi); err != nil {
		return "", false
	}
	// A file with only one link cannot collide with any other path, so
	// dedup is unnecessary. Returning ok=false short-circuits the map
	// lookup and (correctly) counts the file as unique.
	if bhfi.NumberOfLinks <= 1 {
		return "", false
	}
	return fmt.Sprintf("%d:%d:%d", bhfi.VolumeSerialNumber, bhfi.FileIndexHigh, bhfi.FileIndexLow), true
}
