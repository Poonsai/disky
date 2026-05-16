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
