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

// SortAncestors re-sorts the Children slice of each ancestor of n (its
// parent, grandparent, and so on up to the root). Use this after n's Size
// changes so that n's position among its siblings — and the position of n's
// parent among its siblings, etc. — stays consistent with the descending-by-
// size invariant. Unlike Sort, this does NOT recurse into the subtrees of
// each ancestor's other children, which is what makes it O(depth) instead of
// O(tree size).
func SortAncestors(n *Node) {
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		sort.Slice(cur.Children, func(i, j int) bool {
			return cur.Children[i].Size > cur.Children[j].Size
		})
	}
}
