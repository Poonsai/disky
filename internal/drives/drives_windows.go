//go:build windows

// Package drives enumerates mounted Windows drives.
package drives

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Drive describes a mounted local drive.
type Drive struct {
	Letter string // e.g. "C:\\"
	Label  string // e.g. "Local Disk"
	Total  int64
	Free   int64
}

// UsedPercent returns the percentage of Total currently used (0-100).
// Returns 0 if Total is 0.
func (d Drive) UsedPercent() int {
	if d.Total == 0 {
		return 0
	}
	used := d.Total - d.Free
	return int(used * 100 / d.Total)
}

// List returns mounted fixed and removable drives. USB sticks count;
// network mounts and CD-ROMs are excluded.
func List() ([]Drive, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("GetLogicalDrives: %w", err)
	}
	var drives []Drive
	for i := uint32(0); i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		letter := fmt.Sprintf("%c:\\", 'A'+i)
		ptr, _ := windows.UTF16PtrFromString(letter)

		dt := windows.GetDriveType(ptr)
		if dt != windows.DRIVE_FIXED && dt != windows.DRIVE_REMOVABLE {
			continue
		}

		var free, total, totalFree uint64
		if err := windows.GetDiskFreeSpaceEx(ptr, &free, &total, &totalFree); err != nil {
			continue
		}

		var nameBuf [windows.MAX_PATH + 1]uint16
		var fsBuf [windows.MAX_PATH + 1]uint16
		var serial, maxLen, flags uint32
		_ = windows.GetVolumeInformation(
			ptr,
			&nameBuf[0], uint32(len(nameBuf)),
			&serial, &maxLen, &flags,
			&fsBuf[0], uint32(len(fsBuf)),
		)
		label := windows.UTF16ToString(nameBuf[:])
		if label == "" {
			label = "Local Disk"
		}

		drives = append(drives, Drive{
			Letter: letter,
			Label:  label,
			Total:  int64(total),
			Free:   int64(totalFree),
		})
	}
	return drives, nil
}
