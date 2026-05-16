// Package scan walks a directory tree and produces a tree.Node hierarchy
// suitable for browsing in the disky TUI.
package scan

import (
	"context"
	"os"
	"path/filepath"

	"github.com/boozercab/disky/internal/tree"
)

// Progress is updated atomically during a scan so the UI can render counters.
// Pass nil if progress is not needed.
type Progress struct{} // expanded in Task 8

// Scan walks root and returns a fully-populated *tree.Node, with Sizes
// computed bottom-up and Children sorted descending by Size.
// If ctx is cancelled the scan returns what it has so far along with ctx.Err().
func Scan(ctx context.Context, root string, _ *Progress) (*tree.Node, error) {
	rootNode := &tree.Node{Name: filepath.Clean(root), IsDir: true}
	if err := walkDir(ctx, rootNode.Name, rootNode); err != nil {
		return rootNode, err
	}
	tree.ComputeSizes(rootNode)
	tree.Sort(rootNode)
	return rootNode, nil
}

func walkDir(ctx context.Context, path string, n *tree.Node) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		n.Err = err
		return nil
	}
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		info, err := os.Lstat(childPath)
		if err != nil {
			continue
		}
		child := &tree.Node{
			Name:   entry.Name(),
			Parent: n,
			IsDir:  info.IsDir(),
		}
		switch {
		case info.IsDir():
			_ = walkDir(ctx, childPath, child)
		default:
			child.Size = info.Size()
		}
		n.Children = append(n.Children, child)
	}
	return nil
}
