package tree

import "testing"

func TestSortDescending(t *testing.T) {
	root := &Node{Name: "root", IsDir: true}
	root.Children = []*Node{
		{Name: "small", Size: 10, Parent: root},
		{Name: "big", Size: 1000, Parent: root},
		{Name: "medium", Size: 100, Parent: root},
	}

	Sort(root)

	want := []string{"big", "medium", "small"}
	for i, c := range root.Children {
		if c.Name != want[i] {
			t.Errorf("position %d: got %q, want %q", i, c.Name, want[i])
		}
	}
}

func TestSortRecursive(t *testing.T) {
	root := &Node{Name: "root", IsDir: true}
	sub := &Node{Name: "sub", IsDir: true, Parent: root, Size: 200}
	sub.Children = []*Node{
		{Name: "small", Size: 10, Parent: sub},
		{Name: "big", Size: 100, Parent: sub},
	}
	root.Children = []*Node{sub}

	Sort(root)

	if sub.Children[0].Name != "big" {
		t.Fatalf("sub.Children[0]: got %q, want %q", sub.Children[0].Name, "big")
	}
}

func TestSortAncestors(t *testing.T) {
	// Three-level tree, all initially sorted descending.
	// root (size 300)
	//   big-branch (size 200)
	//     leaf-1 (size 200)
	//   small-branch (size 100)
	//     leaf-2 (size 100)
	root := &Node{Name: "root", IsDir: true, Size: 300}
	big := &Node{Name: "big-branch", IsDir: true, Parent: root, Size: 200}
	leaf1 := &Node{Name: "leaf-1", Parent: big, Size: 200}
	big.Children = []*Node{leaf1}

	small := &Node{Name: "small-branch", IsDir: true, Parent: root, Size: 100}
	leaf2 := &Node{Name: "leaf-2", Parent: small, Size: 100}
	small.Children = []*Node{leaf2}

	root.Children = []*Node{big, small}

	// Shrink big-branch below small-branch and run SortAncestors on the
	// leaf whose ancestor sizes changed.
	big.Size = 50
	leaf1.Size = 50

	SortAncestors(leaf1)

	if root.Children[0].Name != "small-branch" {
		t.Errorf("root.Children[0]: got %q, want %q", root.Children[0].Name, "small-branch")
	}
	if root.Children[1].Name != "big-branch" {
		t.Errorf("root.Children[1]: got %q, want %q", root.Children[1].Name, "big-branch")
	}
}
