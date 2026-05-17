//go:build windows && recycletest

package recycle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSendMovesFileToRecycleBin(t *testing.T) {
	// Build with: go test -tags=recycletest ./internal/recycle/
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.txt")
	if err := os.WriteFile(path, []byte("delete me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Send(path); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after Send: err=%v", err)
	}
}

func TestSendMissingPathReturnsError(t *testing.T) {
	// Build with: go test -tags=recycletest ./internal/recycle/
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.txt")

	// IFileOperation should refuse a non-existent path. The old
	// SHFileOperationW version returned a useless "code N" error here;
	// the new path should surface a real HRESULT.
	if err := Send(missing); err == nil {
		t.Fatalf("Send(missing path) succeeded; expected an error")
	}
}

func TestSendLongPathSucceeds(t *testing.T) {
	// Build with: go test -tags=recycletest ./internal/recycle/
	// Verifies that paths over MAX_PATH (260) can be recycled. The old
	// SHFileOperationW implementation failed silently on long paths.
	dir := t.TempDir()
	// Build a nested directory chain that takes the absolute path well
	// past 260 chars. Each segment is short on its own to avoid hitting
	// per-component limits.
	current := dir
	for i := 0; i < 12; i++ {
		current = filepath.Join(current, "long-segment-name-padding")
		if err := os.Mkdir(current, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", current, err)
		}
	}
	target := filepath.Join(current, "leaf.txt")
	if len(target) <= 260 {
		t.Skipf("temp dir too short to exceed MAX_PATH: %d chars", len(target))
	}
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write long-path file: %v", err)
	}

	if err := Send(target); err != nil {
		t.Fatalf("Send long path (%d chars): %v", len(target), err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("long-path file still exists after Send: err=%v", err)
	}
}
