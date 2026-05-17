//go:build !windows

package scan

import "os"

// isReparsePoint is Windows-only. On other platforms there's no
// equivalent concept (junction/mount-point reparse tags), so the
// standard ModeSymlink check in walkOne handles everything.
func isReparsePoint(info os.FileInfo) bool { return false }

// fileUniqueID dedup is currently Windows-only. On other platforms we
// return ok=false so every file is counted as unique. POSIX has
// st_dev/st_ino which would work the same way — left for a follow-up.
func fileUniqueID(path string, info os.FileInfo) (string, bool) {
	return "", false
}
