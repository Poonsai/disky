//go:build windows

// Package recycle moves filesystem paths into the Windows Recycle Bin.
package recycle

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	foDelete          = 0x0003
	fofSilent         = 0x0004
	fofNoConfirmation = 0x0010
	fofAllowUndo      = 0x0040
	fofNoErrorUI      = 0x0400
)

// shFileOpStruct mirrors SHFILEOPSTRUCTW from shellapi.h.
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

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

// Send moves a single absolute path to the Recycle Bin.
// path must be absolute and exist; otherwise SHFileOperationW returns an error.
func Send(path string) error {
	utf16, err := syscall.UTF16FromString(path)
	if err != nil {
		return fmt.Errorf("encode path: %w", err)
	}
	// SHFileOperationW expects pFrom to be a double-null-terminated string list.
	// syscall.UTF16FromString already appends one null; we append another.
	utf16 = append(utf16, 0)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &utf16[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofNoErrorUI | fofSilent,
	}

	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperationW failed: code %d", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("operation aborted")
	}
	return nil
}
