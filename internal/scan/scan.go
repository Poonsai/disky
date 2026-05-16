// Package scan walks a directory tree and produces a tree.Node hierarchy
// suitable for browsing in the disky TUI.
package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/boozercab/disky/internal/tree"
)

// Progress is updated atomically during a scan so the UI can render counters.
// Pass nil if progress is not needed.
type Progress struct{} // expanded in Task 8

func Scan(ctx context.Context, root string, _ *Progress) (*tree.Node, error) {
	rootNode := &tree.Node{Name: filepath.Clean(root), IsDir: true}

	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	var mu sync.Mutex // protects appends to n.Children

	var walk func(path string, n *tree.Node)
	walk = func(path string, n *tree.Node) {
		defer wg.Done()
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			mu.Lock()
			n.Err = err
			mu.Unlock()
			return
		}
		var local []*tree.Node
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
			if info.IsDir() {
				wg.Add(1)
				go walk(childPath, child)
			} else {
				child.Size = info.Size()
			}
			local = append(local, child)
		}
		mu.Lock()
		n.Children = local
		mu.Unlock()
	}

	wg.Add(1)
	go walk(rootNode.Name, rootNode)
	wg.Wait()

	tree.ComputeSizes(rootNode)
	tree.Sort(rootNode)

	if err := ctx.Err(); err != nil {
		return rootNode, err
	}
	return rootNode, nil
}
