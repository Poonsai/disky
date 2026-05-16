package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// buildTree creates a deterministic temp directory tree for tests:
//
//	root/
//	  a.txt          (10 bytes)
//	  sub/
//	    b.txt        (20 bytes)
//	    deep/
//	      c.txt      (40 bytes)
//
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

func TestScanProgress(t *testing.T) {
	root := buildTree(t)
	p := &Progress{}

	_, err := Scan(context.Background(), root, p)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if got := atomic.LoadInt64(&p.Items); got != 3 {
		t.Errorf("Items: got %d, want 3", got)
	}
	if got := atomic.LoadInt64(&p.Bytes); got != 70 {
		t.Errorf("Bytes: got %d, want 70", got)
	}
	if p.CurrentPath() == "" {
		t.Errorf("CurrentPath should be non-empty after scan")
	}
}

func TestScanCancellation(t *testing.T) {
	root := t.TempDir()
	// Create a wide tree: 200 dirs, each with 5 files
	for i := 0; i < 200; i++ {
		dir := filepath.Join(root, "d"+strconv.Itoa(i))
		os.Mkdir(dir, 0o755)
		for j := 0; j < 5; j++ {
			mustWrite(t, filepath.Join(dir, "f"+strconv.Itoa(j)), 1)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so workers see a cancelled context on first dequeue.
	cancel()

	start := time.Now()
	_, err := Scan(ctx, root, nil)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Scan took too long after cancel: %v", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err: got %v, want context.Canceled", err)
	}
}

func TestScanSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	os.Mkdir(target, 0o755)
	mustWrite(t, filepath.Join(target, "real.txt"), 50)

	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	got, err := Scan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var linkNode *struct{ s int64 }
	for _, c := range got.Children {
		if c.Name == "link" {
			linkNode = &struct{ s int64 }{c.Size}
			if c.IsDir {
				t.Errorf("link should not be treated as dir")
			}
			if len(c.Children) != 0 {
				t.Errorf("link should have no children: got %d", len(c.Children))
			}
		}
	}
	if linkNode == nil {
		t.Fatal("link not found in scan output")
	}
	if linkNode.s != 0 {
		t.Errorf("link size: got %d, want 0 (symlinks count as 0)", linkNode.s)
	}
	// Real file should still be counted via the real path.
	if got.Size != 50 {
		t.Errorf("root size: got %d, want 50 (link not double-counted)", got.Size)
	}
}
