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
