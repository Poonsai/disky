package tree

import "testing"

func TestNodePathRoot(t *testing.T) {
	root := &Node{Name: `C:\`, IsDir: true}
	if got := root.Path(); got != `C:\` {
		t.Fatalf("root path: got %q, want %q", got, `C:\`)
	}
}

func TestNodePathNested(t *testing.T) {
	root := &Node{Name: `C:\`, IsDir: true}
	users := &Node{Name: "Users", Parent: root, IsDir: true}
	boozer := &Node{Name: "Boozer", Parent: users, IsDir: true}
	file := &Node{Name: "note.txt", Parent: boozer}

	if got, want := file.Path(), `C:\Users\Boozer\note.txt`; got != want {
		t.Fatalf("file path: got %q, want %q", got, want)
	}
}
