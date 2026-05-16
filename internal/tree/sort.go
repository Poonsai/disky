package tree

import "sort"

// Sort orders every directory's Children descending by Size, recursively.
func Sort(n *Node) {
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Size > n.Children[j].Size
	})
	for _, c := range n.Children {
		if c.IsDir {
			Sort(c)
		}
	}
}
