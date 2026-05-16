package scan

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// buildTree creates a deterministic temp directory tree for tests:
//   root/
//     a.txt          (10 bytes)
//     sub/
//       b.txt        (20 bytes)
//       deep/
//         c.txt      (40 bytes)
// Returns the root path. Cleanup is handled by t.TempDir.
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), 10)
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), 20)
	if err := os.Mkdir(filepath.Join(root, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "sub", "deep", "c.txt"), 40)
	return root
}

func mustWrite(t *testing.T, path string, size int64) {
	t.Helper()
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanBasic(t *testing.T) {
	root := buildTree(t)

	got, err := Scan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if got.Size != 70 {
		t.Errorf("root size: got %d, want 70", got.Size)
	}
	if len(got.Children) != 2 {
		t.Fatalf("root children: got %d, want 2", len(got.Children))
	}
	// Sorted descending by size: "sub" (60 bytes) before "a.txt" (10 bytes).
	if got.Children[0].Name != "sub" {
		t.Errorf("first child: got %q, want %q", got.Children[0].Name, "sub")
	}
	sub := got.Children[0]
	if sub.Size != 60 {
		t.Errorf("sub size: got %d, want 60", sub.Size)
	}
}

func TestScanParallelCorrectness(t *testing.T) {
	root := t.TempDir()
	// 50 sibling dirs each with 10 files of 1 byte = 500 bytes total
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dir := filepath.Join(root, "d"+strconv.Itoa(i))
			os.Mkdir(dir, 0o755)
			for j := 0; j < 10; j++ {
				mustWrite(t, filepath.Join(dir, "f"+strconv.Itoa(j)), 1)
			}
		}(i)
	}
	wg.Wait()

	got, err := Scan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.Size != 500 {
		t.Errorf("root size: got %d, want 500", got.Size)
	}
	if len(got.Children) != 50 {
		t.Errorf("root children: got %d, want 50", len(got.Children))
	}
}
