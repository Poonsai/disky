package tree

import "testing"

func TestComputeSizes(t *testing.T) {
	root := &Node{Name: "root", IsDir: true}
	sub := &Node{Name: "sub", IsDir: true, Parent: root}
	sub.Children = []*Node{
		{Name: "a", Size: 10, Parent: sub},
		{Name: "b", Size: 20, Parent: sub},
	}
	root.Children = []*Node{
		sub,
		{Name: "c", Size: 5, Parent: root},
	}

	ComputeSizes(root)

	if sub.Size != 30 {
		t.Errorf("sub.Size: got %d, want 30", sub.Size)
	}
	if root.Size != 35 {
		t.Errorf("root.Size: got %d, want 35", root.Size)
	}
}

func TestRemoveAndRecompute(t *testing.T) {
	root := &Node{Name: "root", IsDir: true, Size: 35}
	sub := &Node{Name: "sub", IsDir: true, Parent: root, Size: 30}
	a := &Node{Name: "a", Size: 10, Parent: sub}
	b := &Node{Name: "b", Size: 20, Parent: sub}
	sub.Children = []*Node{a, b}
	c := &Node{Name: "c", Size: 5, Parent: root}
	root.Children = []*Node{sub, c}

	RemoveAndRecompute(b)

	if len(sub.Children) != 1 || sub.Children[0] != a {
		t.Errorf("sub.Children after remove: %v", sub.Children)
	}
	if sub.Size != 10 {
		t.Errorf("sub.Size after remove: got %d, want 10", sub.Size)
	}
	if root.Size != 15 {
		t.Errorf("root.Size after remove: got %d, want 15", root.Size)
	}
}

func TestRemoveRoot(t *testing.T) {
	root := &Node{Name: "root", IsDir: true, Size: 100}
	// Removing root must be a no-op (no parent to recompute).
	RemoveAndRecompute(root)
	if root.Size != 100 {
		t.Errorf("root.Size should be unchanged: got %d", root.Size)
	}
}
