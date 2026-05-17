//go:build windows

// Package recycle moves filesystem paths into the Windows Recycle Bin.
//
// We use the IFileOperation COM API rather than the legacy
// SHFileOperationW for two reasons:
//
//  1. Long paths. SHFileOperationW does not honor the `\\?\` long-path
//     prefix and silently fails on paths over MAX_PATH (260 chars),
//     which are common inside nested node_modules trees.
//  2. Safety. SHFileOperationW with FOF_ALLOWUNDO silently downgrades to
//     a permanent delete when the file can't go to the Recycle Bin
//     (network share, bin quota exceeded). IFileOperation with
//     FOFX_RECYCLEONDELETE returns an HRESULT error instead, so callers
//     find out before the user loses unrecoverable data.
package recycle

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32                           = windows.NewLazySystemDLL("ole32.dll")
	shell32                         = windows.NewLazySystemDLL("shell32.dll")
	procCoInitializeEx              = ole32.NewProc("CoInitializeEx")
	procCoUninitialize              = ole32.NewProc("CoUninitialize")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
)

// COM class/interface IDs. Values come from <shobjidl_core.h> / <shlobj.h>.
var (
	clsidFileOperation = windows.GUID{
		Data1: 0x3AD05575, Data2: 0x8857, Data3: 0x4850,
		Data4: [8]byte{0x92, 0x77, 0x11, 0xB8, 0x5B, 0xDB, 0x8E, 0x09},
	}
	iidIFileOperation = windows.GUID{
		Data1: 0x947AAB5F, Data2: 0x0A5C, Data3: 0x4C13,
		Data4: [8]byte{0xB4, 0xD6, 0x4B, 0xF7, 0x83, 0x6F, 0xC9, 0xF8},
	}
	iidIShellItem = windows.GUID{
		Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE,
		Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE},
	}
)

const (
	coinitApartmentThreaded = 0x2
	coinitDisableOle1Dde    = 0x4

	clsctxInprocServer = 0x1

	// IFileOperation flags. We suppress UI but keep error reporting via
	// HRESULT — so a failed recycle returns an error rather than silently
	// turning into a permanent delete.
	fofSilent           uint32 = 0x0004
	fofNoConfirmation   uint32 = 0x0010
	fofNoErrorUI        uint32 = 0x0400
	fofNoConfirmMkDir   uint32 = 0x0200
	fofxRecycleOnDelete uint32 = 0x00080000
	fofxEarlyFailure    uint32 = 0x00100000

	sFalse          = uintptr(0x00000001) // CoInitializeEx: already initialized on this thread
	rpcEChangedMode = uintptr(0x80010106) // CoInitializeEx: thread already in a different apartment
)

// IFileOperation vtable indices (IUnknown's 3 methods, then IFileOperation's).
const (
	ifopSetOperationFlags = 5
	ifopDeleteItem        = 18
	ifopPerformOperations = 21
)

// IUnknown vtable index for Release. All COM interfaces share this layout.
const iunkRelease = 2

// comObj wraps a COM interface pointer. The pointer is stored as
// unsafe.Pointer (not uintptr) so that vtable dereferences are direct
// pointer→pointer conversions rather than the uintptr→unsafe.Pointer
// round-trip that `go vet` flags as "possible misuse of unsafe.Pointer".
// The underlying memory is OS-allocated and outside the Go heap, so
// Go's GC never moves or scans it.
type comObj struct {
	p unsafe.Pointer
}

func (c comObj) isNil() bool { return c.p == nil }

func (c comObj) vtable() *[64]uintptr {
	return *(**[64]uintptr)(c.p)
}

func (c comObj) call(index int, args ...uintptr) uintptr {
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(c.p))
	all = append(all, args...)
	hr, _, _ := syscall.SyscallN(c.vtable()[index], all...)
	return hr
}

func (c comObj) release() {
	if c.isNil() {
		return
	}
	syscall.SyscallN(c.vtable()[iunkRelease], uintptr(c.p))
}

// Send moves a single absolute path to the Recycle Bin. Returns an error
// if the operation fails — including the case where the OS would have
// silently permanent-deleted the file (e.g. a network share or a file
// too large for the bin's quota) because FOFX_RECYCLEONDELETE makes
// IFileOperation surface that condition as an HRESULT.
func Send(path string) error {
	// COM is per-thread. Bubble Tea callbacks can run on different
	// goroutines, which Go's scheduler can move between OS threads.
	// Pinning ensures CoInitializeEx, the COM calls, and CoUninitialize
	// all run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	needsUninit, err := initializeCOM()
	if err != nil {
		return err
	}
	if needsUninit {
		defer procCoUninitialize.Call()
	}

	fileOp, err := createFileOperation()
	if err != nil {
		return err
	}
	defer fileOp.release()

	if err := setOperationFlags(fileOp, fofSilent|fofNoConfirmation|fofNoErrorUI|fofNoConfirmMkDir|fofxRecycleOnDelete|fofxEarlyFailure); err != nil {
		return err
	}

	item, err := shellItemFromPath(path)
	if err != nil {
		return err
	}
	defer item.release()

	if err := deleteItem(fileOp, item); err != nil {
		return err
	}
	return performOperations(fileOp)
}

// initializeCOM returns (needsUninit, err). needsUninit is true only when
// we successfully initialized COM on this thread and therefore owe a
// matching CoUninitialize. If COM was already initialized in a different
// apartment we return needsUninit=false and let the existing init stand.
func initializeCOM() (bool, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded|coinitDisableOle1Dde)
	switch hr {
	case 0:
		return true, nil
	case sFalse:
		// COM was already initialized on this thread in a compatible
		// mode. Per MS docs we still owe a CoUninitialize for our
		// successful init call.
		return true, nil
	case rpcEChangedMode:
		// Thread is already in a different apartment. Use it as-is and
		// do NOT call CoUninitialize — that would unbalance the count
		// kept by whoever first initialized this thread.
		return false, nil
	default:
		return false, fmt.Errorf("CoInitializeEx: 0x%x", hr)
	}
}

func createFileOperation() (comObj, error) {
	var fileOp comObj
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOperation)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOperation)),
		uintptr(unsafe.Pointer(&fileOp.p)),
	)
	if hr != 0 {
		return comObj{}, fmt.Errorf("CoCreateInstance(IFileOperation): 0x%x", hr)
	}
	return fileOp, nil
}

func shellItemFromPath(path string) (comObj, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return comObj{}, fmt.Errorf("encode path: %w", err)
	}
	var item comObj
	hr, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&iidIShellItem)),
		uintptr(unsafe.Pointer(&item.p)),
	)
	if hr != 0 {
		return comObj{}, fmt.Errorf("resolve %q: 0x%x", path, hr)
	}
	return item, nil
}

func setOperationFlags(fileOp comObj, flags uint32) error {
	if hr := fileOp.call(ifopSetOperationFlags, uintptr(flags)); hr != 0 {
		return fmt.Errorf("SetOperationFlags: 0x%x", hr)
	}
	return nil
}

func deleteItem(fileOp, item comObj) error {
	// DeleteItem(IShellItem* psiItem, IFileOperationProgressSink* pfopsItem)
	if hr := fileOp.call(ifopDeleteItem, uintptr(item.p), 0); hr != 0 {
		return fmt.Errorf("DeleteItem: 0x%x", hr)
	}
	return nil
}

func performOperations(fileOp comObj) error {
	if hr := fileOp.call(ifopPerformOperations); hr != 0 {
		return fmt.Errorf("PerformOperations: 0x%x", hr)
	}
	return nil
}
