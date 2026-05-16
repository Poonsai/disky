// Package scan walks a directory tree and produces a tree.Node hierarchy
// suitable for browsing in the disky TUI.
package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/boozercab/disky/internal/tree"
)

// Progress is updated atomically during a scan so the UI can render counters.
// Use Progress.CurrentPath() to read the latest path string safely.
type Progress struct {
	Items       int64
	Bytes       int64
	currentPath atomic.Value // string
}

func (p *Progress) CurrentPath() string {
	if p == nil {
		return ""
	}
	v := p.currentPath.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func Scan(ctx context.Context, root string, p *Progress) (*tree.Node, error) {
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

		if p != nil {
			p.currentPath.Store(path)
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
				if p != nil {
					atomic.AddInt64(&p.Items, 1)
					atomic.AddInt64(&p.Bytes, info.Size())
				}
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
