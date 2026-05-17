package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boozercab/disky/internal/tree"
)

func sampleTree() *tree.Node {
	root := &tree.Node{Name: `C:\`, IsDir: true, Size: 130}
	sub := &tree.Node{Name: "sub", IsDir: true, Parent: root, Size: 100}
	sub.Children = []*tree.Node{
		{Name: "big.bin", Size: 80, Parent: sub},
		{Name: "small.txt", Size: 20, Parent: sub},
	}
	root.Children = []*tree.Node{
		sub,
		{Name: "a.txt", Size: 30, Parent: root},
	}
	return root
}

func TestBrowserInitial(t *testing.T) {
	m := NewBrowser(sampleTree())
	if m.Cursor != 0 {
		t.Errorf("Cursor: got %d, want 0", m.Cursor)
	}
	if m.Current.Name != `C:\` {
		t.Errorf("Current.Name: got %q, want %q", m.Current.Name, `C:\`)
	}
}

func TestBrowserEnterFolder(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Cursor at row 0 = "sub" (largest first).
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bm := next.(BrowserModel)
	if bm.Current.Name != "sub" {
		t.Errorf("Current.Name: got %q, want %q", bm.Current.Name, "sub")
	}
	if bm.Cursor != 0 {
		t.Errorf("Cursor after enter: got %d, want 0", bm.Cursor)
	}
}

func TestBrowserGoBack(t *testing.T) {
	m := NewBrowser(sampleTree())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	back, _ := next.(BrowserModel).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if back.(BrowserModel).Current.Name != `C:\` {
		t.Errorf("after back: %q", back.(BrowserModel).Current.Name)
	}
}

func TestBrowserBackAtRootIsNoop(t *testing.T) {
	m := NewBrowser(sampleTree())
	back, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if back.(BrowserModel).Current.Name != `C:\` {
		t.Errorf("back at root should be no-op")
	}
}

func TestBrowserEnterFileIsNoop(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Move cursor to "a.txt" (row 1).
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	enter, _ := down.(BrowserModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if enter.(BrowserModel).Current.Name != `C:\` {
		t.Errorf("enter on file should not change Current")
	}
}

func TestBrowserDeleteRequest(t *testing.T) {
	m := NewBrowser(sampleTree())
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("d"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if bm.PendingDelete == nil {
		t.Fatal("PendingDelete should be set after 'd'")
	}
	if bm.PendingDelete.Name != "sub" {
		t.Errorf("PendingDelete.Name: got %q, want %q", bm.PendingDelete.Name, "sub")
	}
}

func TestBrowserApplyDelete(t *testing.T) {
	m := NewBrowser(sampleTree())
	target := m.Current.Children[0] // "sub"
	m = m.ApplyDelete(target)
	// "sub" should be gone from root.Children, leaving only "a.txt".
	if len(m.Current.Children) != 1 || m.Current.Children[0].Name != "a.txt" {
		t.Errorf("Current.Children after delete: %+v", m.Current.Children)
	}
	if m.Root.Size != 30 {
		t.Errorf("Root.Size: got %d, want 30", m.Root.Size)
	}
	// Cursor must remain within bounds.
	if m.Cursor >= len(m.Current.Children) {
		t.Errorf("Cursor out of bounds: %d (len=%d)", m.Cursor, len(m.Current.Children))
	}
}

func TestBrowserCancelDelete(t *testing.T) {
	m := NewBrowser(sampleTree())
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("d"), Type: tea.KeyRunes})
	m = next.(BrowserModel).CancelDelete()
	if m.PendingDelete != nil {
		t.Error("PendingDelete should be cleared after Cancel")
	}
}

func TestBrowserRescanRequest(t *testing.T) {
	m := NewBrowser(sampleTree())
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("r"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if !bm.PendingRescan {
		t.Error("PendingRescan should be true after 'r'")
	}
}

func TestBrowserApplyRescan(t *testing.T) {
	m := NewBrowser(sampleTree())
	m.PendingRescan = true

	// Build a replacement subtree for the current folder.
	newSub := &tree.Node{Name: `C:\`, IsDir: true, Size: 50}
	newSub.Children = []*tree.Node{
		{Name: "fresh.txt", Size: 50, Parent: newSub},
	}

	m = m.ApplyRescan(newSub)

	if m.PendingRescan {
		t.Error("PendingRescan should be cleared after Apply")
	}
	if m.Current.Size != 50 {
		t.Errorf("Current.Size after rescan: got %d, want 50", m.Current.Size)
	}
	if len(m.Current.Children) != 1 || m.Current.Children[0].Name != "fresh.txt" {
		t.Errorf("Current.Children after rescan: %+v", m.Current.Children)
	}
}

func TestBrowserCancelRescan(t *testing.T) {
	m := NewBrowser(sampleTree())
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("r"), Type: tea.KeyRunes})
	m = next.(BrowserModel).CancelRescan()
	if m.PendingRescan {
		t.Error("PendingRescan should be cleared after Cancel")
	}
}

func TestBrowserApplyDeleteResortsAncestors(t *testing.T) {
	// Build a 3-level tree where deleting inside "big-sub" makes it
	// smaller than its sibling "small-sub", so root.Children should
	// resort.
	root := &tree.Node{Name: `C:\`, IsDir: true, Size: 250}
	bigSub := &tree.Node{Name: "big-sub", IsDir: true, Parent: root, Size: 200}
	target := &tree.Node{Name: "fat.bin", Size: 200, Parent: bigSub}
	bigSub.Children = []*tree.Node{target}

	smallSub := &tree.Node{Name: "small-sub", IsDir: true, Parent: root, Size: 50}
	smallSub.Children = []*tree.Node{{Name: "tiny.txt", Size: 50, Parent: smallSub}}

	root.Children = []*tree.Node{bigSub, smallSub}

	// Browse into big-sub and delete its only child.
	m := NewBrowser(root)
	m.Current = bigSub
	m = m.ApplyDelete(target)

	// root.Children must now have small-sub first (50 > 0 after the delete).
	if root.Children[0].Name != "small-sub" {
		t.Errorf("root.Children[0] after delete: got %q, want %q",
			root.Children[0].Name, "small-sub")
	}
}

func TestBrowserApplyRescanResortsAncestors(t *testing.T) {
	// Same setup, but trigger via rescan that shrinks big-sub.
	root := &tree.Node{Name: `C:\`, IsDir: true, Size: 250}
	bigSub := &tree.Node{Name: "big-sub", IsDir: true, Parent: root, Size: 200}
	bigSub.Children = []*tree.Node{{Name: "fat.bin", Size: 200, Parent: bigSub}}

	smallSub := &tree.Node{Name: "small-sub", IsDir: true, Parent: root, Size: 50}
	smallSub.Children = []*tree.Node{{Name: "tiny.txt", Size: 50, Parent: smallSub}}

	root.Children = []*tree.Node{bigSub, smallSub}

	m := NewBrowser(root)
	m.Current = bigSub
	m.PendingRescan = true

	// Replacement subtree: big-sub is now only 10 bytes.
	newCurrent := &tree.Node{Name: "big-sub", IsDir: true, Size: 10}
	newCurrent.Children = []*tree.Node{{Name: "trimmed.txt", Size: 10}}

	m = m.ApplyRescan(newCurrent)

	if root.Children[0].Name != "small-sub" {
		t.Errorf("root.Children[0] after rescan: got %q, want %q",
			root.Children[0].Name, "small-sub")
	}
}
