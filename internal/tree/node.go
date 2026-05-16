// Package tree defines the in-memory directory tree used throughout disky.
package tree

import "path/filepath"

type Node struct {
	Name     string
	Size     int64
	Children []*Node
	Parent   *Node
	IsDir    bool
	Err      error
}

func (n *Node) Path() string {
	if n.Parent == nil {
		return n.Name
	}
	return filepath.Join(n.Parent.Path(), n.Name)
}
