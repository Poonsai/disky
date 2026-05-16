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
