//go:build windows

package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScanDedupsHardLinks(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original.bin")
	mustWrite(t, original, 1000)

	link := filepath.Join(root, "linked.bin")
	if err := os.Link(original, link); err != nil {
		t.Skipf("hard link unsupported in this environment: %v", err)
	}

	got, err := Scan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Without dedup the root would total 2000 bytes (one full count per
	// hard link). With dedup the second link counts as 0, so the root
	// should match the underlying data size of 1000.
	if got.Size != 1000 {
		t.Errorf("root size with hardlink: got %d, want 1000 (dedup expected)", got.Size)
	}
	if len(got.Children) != 2 {
		t.Fatalf("root children: got %d, want 2", len(got.Children))
	}

	// Exactly one of the two children should keep its 1000-byte weight.
	// Which one wins is order-dependent on the worker pool, but the sum
	// must be 1000 and one must be 0.
	a, b := got.Children[0].Size, got.Children[1].Size
	if a+b != 1000 || (a != 0 && b != 0) {
		t.Errorf("expected one child sized 1000 and one sized 0; got %d and %d", a, b)
	}
}

func TestScanSkipsDirectoryJunctions(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(target, "inside.bin"), 500)

	junction := filepath.Join(root, "junction")
	// mklink /J requires no admin rights on local NTFS volumes, unlike
	// mklink /D which usually does. Use cmd because Go's stdlib has no
	// junction-creation API.
	cmd := exec.Command("cmd", "/c", "mklink", "/J", junction, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("mklink /J unsupported here: %v: %s", err, string(out))
	}

	got, err := Scan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// The walker must NOT descend into the junction. Doing so would
	// double-count target's 500 bytes via the junction path.
	if got.Size != 500 {
		t.Errorf("root size: got %d, want 500 (junction should not be followed)", got.Size)
	}

	var junctionNode *struct {
		size     int64
		isDir    bool
		children int
	}
	for _, c := range got.Children {
		if c.Name == "junction" {
			junctionNode = &struct {
				size     int64
				isDir    bool
				children int
			}{c.Size, c.IsDir, len(c.Children)}
		}
	}
	if junctionNode == nil {
		t.Fatal("junction not found in scan output")
	}
	if junctionNode.size != 0 {
		t.Errorf("junction size: got %d, want 0 (reparse points count as 0)", junctionNode.size)
	}
	if junctionNode.isDir {
		t.Errorf("junction should not be treated as a directory in the tree")
	}
	if junctionNode.children != 0 {
		t.Errorf("junction should have no children: got %d", junctionNode.children)
	}
}
