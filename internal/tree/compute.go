package tree

// ComputeSizes sets n.Size (and each descendant directory's Size) to the
// sum of its children's sizes. File nodes' sizes are assumed to already be set.
// Returns the total computed for n.
func ComputeSizes(n *Node) int64 {
	if !n.IsDir {
		return n.Size
	}
	var total int64
	for _, c := range n.Children {
		total += ComputeSizes(c)
	}
	n.Size = total
	return total
}

// RemoveAndRecompute removes n from its parent's Children and walks the parent
// chain subtracting n.Size from every ancestor. Calling on a root node is a no-op.
func RemoveAndRecompute(n *Node) {
	if n.Parent == nil {
		return
	}
	delta := n.Size
	siblings := n.Parent.Children
	for i, c := range siblings {
		if c == n {
			n.Parent.Children = append(siblings[:i], siblings[i+1:]...)
			break
		}
	}
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		cur.Size -= delta
	}
}
