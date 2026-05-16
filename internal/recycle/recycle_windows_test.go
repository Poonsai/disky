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
