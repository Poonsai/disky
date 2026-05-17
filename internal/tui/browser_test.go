package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Poonsai/disky/internal/tree"
)

// TestMain forces a color profile so lipgloss emits ANSI SGR sequences
// during tests. Without this, every Render() returns plain text and the
// styling tests can't distinguish styled from unstyled output.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

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

// wideTree builds a root with `n` children, sized so they sort 0..n-1.
func wideTree(n int) *tree.Node {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	for i := 0; i < n; i++ {
		root.Children = append(root.Children, &tree.Node{
			Name:   fmt.Sprintf("child-%02d", i),
			Size:   int64(n - i),
			Parent: root,
		})
	}
	return root
}

// browserAt builds a browser model sized so vp == viewportSize children fit.
// viewportSize > 0.
func browserAt(viewportSize int, childCount int) BrowserModel {
	m := NewBrowser(wideTree(childCount))
	// Layout: 4 chrome rows + viewportSize child rows.
	m.Height = viewportSize + 4
	m.Width = 100
	return m.adjustOffset()
}

func TestBrowserWindowSizeSetsHeight(t *testing.T) {
	m := NewBrowser(wideTree(5))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	bm := next.(BrowserModel)
	if bm.Height != 30 || bm.Width != 80 {
		t.Errorf("Width/Height: got %d×%d, want 80×30", bm.Width, bm.Height)
	}
}

func TestBrowserViewportScrollsDownPastBottom(t *testing.T) {
	// 30 children, viewport shows 10.
	m := browserAt(10, 30)
	// Move cursor to row 15.
	for i := 0; i < 15; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(BrowserModel)
	}
	if m.Cursor != 15 {
		t.Fatalf("Cursor: got %d, want 15", m.Cursor)
	}
	// Cursor must be within [Offset, Offset+vp).
	if m.Cursor < m.Offset || m.Cursor >= m.Offset+10 {
		t.Errorf("cursor %d outside viewport [%d, %d)", m.Cursor, m.Offset, m.Offset+10)
	}
}

func TestBrowserViewportScrollsUpPastTop(t *testing.T) {
	m := browserAt(10, 30)
	// Jump to bottom, then walk back up to row 2.
	gNext, _ := m.Update(tea.KeyMsg{Runes: []rune("G"), Type: tea.KeyRunes})
	m = gNext.(BrowserModel)
	for m.Cursor > 2 {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = next.(BrowserModel)
	}
	if m.Cursor != 2 {
		t.Fatalf("Cursor: got %d, want 2", m.Cursor)
	}
	if m.Cursor < m.Offset || m.Cursor >= m.Offset+10 {
		t.Errorf("cursor %d outside viewport [%d, %d)", m.Cursor, m.Offset, m.Offset+10)
	}
}

func TestBrowserGJumpAdjustsOffset(t *testing.T) {
	m := browserAt(10, 30)
	gNext, _ := m.Update(tea.KeyMsg{Runes: []rune("G"), Type: tea.KeyRunes})
	m = gNext.(BrowserModel)
	if m.Cursor != 29 {
		t.Fatalf("Cursor after G: got %d, want 29", m.Cursor)
	}
	if m.Offset != 20 {
		t.Errorf("Offset after G on 30 children with vp 10: got %d, want 20", m.Offset)
	}
	gtNext, _ := m.Update(tea.KeyMsg{Runes: []rune("g"), Type: tea.KeyRunes})
	m = gtNext.(BrowserModel)
	if m.Cursor != 0 || m.Offset != 0 {
		t.Errorf("after g: Cursor=%d, Offset=%d, want 0/0", m.Cursor, m.Offset)
	}
}

func TestBrowserNoScrollWhenListFits(t *testing.T) {
	// Viewport 20, only 5 children. Offset should stay 0 regardless of cursor.
	m := browserAt(20, 5)
	gNext, _ := m.Update(tea.KeyMsg{Runes: []rune("G"), Type: tea.KeyRunes})
	m = gNext.(BrowserModel)
	if m.Offset != 0 {
		t.Errorf("Offset on small list: got %d, want 0", m.Offset)
	}
}

func TestBrowserHeaderHandlesNonASCIIPath(t *testing.T) {
	// Path with multibyte runes; byte slicing in the middle would corrupt
	// UTF-8. The rendered View must still be valid UTF-8.
	root := &tree.Node{Name: `C:\`, IsDir: true}
	users := &tree.Node{Name: "Users", IsDir: true, Parent: root}
	cafe := &tree.Node{Name: "Café", IsDir: true, Parent: users, Size: 1024}
	docs := &tree.Node{Name: "Документы", IsDir: true, Parent: cafe, Size: 1024}
	docs.Children = []*tree.Node{{Name: "файл.txt", Size: 1024, Parent: docs}}
	cafe.Children = []*tree.Node{docs}
	users.Children = []*tree.Node{cafe}
	root.Children = []*tree.Node{users}

	m := NewBrowser(root)
	m.Current = docs
	m.Width = 40 // force truncation
	m.Height = 24

	out := m.View()
	if !utf8.ValidString(out) {
		t.Error("View output is not valid UTF-8 after path truncation")
	}
}

func TestBrowserRowTruncatesLongName(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	longName := "amd64_microsoft-windows-this-is-a-very-long-package-name-that-would-wrap"
	root.Children = []*tree.Node{{Name: longName, Size: 100, Parent: root}}

	m := NewBrowser(root)
	m.Width = 80
	m.Height = 24

	out := m.View()
	// Output must NOT contain the full long name (it has to be truncated
	// to fit on one terminal line; otherwise the viewport math breaks).
	if strings.Contains(out, longName) {
		t.Errorf("View should truncate long name; full name still present")
	}
	// Every line of the View must be <= width.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		// Strip ANSI escape sequences for the length check.
		stripped := stripANSI(line)
		if utf8.RuneCountInString(stripped) > 80 {
			t.Errorf("line %d exceeds width 80: %q (%d runes)", i, stripped, utf8.RuneCountInString(stripped))
		}
	}
}

func TestBrowserToastClearsOnKey(t *testing.T) {
	m := NewBrowser(sampleTree())
	m.Toast = "scary error"
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if next.(BrowserModel).Toast != "" {
		t.Errorf("Toast should clear on key press; got %q", next.(BrowserModel).Toast)
	}
}

func TestBrowserCursorRowHasNoInnerResetBeforeName(t *testing.T) {
	// Regression test for the "selected-row reverse-video breaks at the
	// Bar boundary" bug. The cursor row's payload must not contain an
	// inner SGR reset (\x1b[0m) that would terminate StyleSelected's
	// reverse-video early.
	root := &tree.Node{Name: `C:\`, IsDir: true}
	root.Children = []*tree.Node{
		{Name: "first", Size: 100, Parent: root},
		{Name: "second", Size: 50, Parent: root},
	}
	m := NewBrowser(root)
	m.Width = 80
	m.Height = 24
	// Cursor on row 0 by default.
	out := m.View()

	// Find the line that contains "first" (the cursor row).
	var cursorLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "first") && !strings.Contains(line, "second") {
			cursorLine = line
			break
		}
	}
	if cursorLine == "" {
		t.Fatal("did not find cursor row in View output")
	}

	// The cursor row should start with a reverse-on SGR (\x1b[7m), then
	// have NO further \x1b[0m until the very end of the row.
	if !strings.Contains(cursorLine, "\x1b[7m") {
		t.Errorf("cursor row missing reverse SGR: %q", cursorLine)
	}
	resetIdx := strings.Index(cursorLine, "\x1b[0m")
	// If there's a reset, it must be at or near the end (after "first").
	// Specifically, no reset should appear before the name text.
	if resetIdx >= 0 {
		nameIdx := strings.Index(cursorLine, "first")
		if resetIdx < nameIdx {
			t.Errorf("inner SGR reset before name breaks reverse-video; row=%q", cursorLine)
		}
	}
}

func TestBrowserToastRendersInBottomSlot(t *testing.T) {
	m := NewBrowser(sampleTree())
	m.Width = 80
	m.Height = 24
	m.Toast = "could not delete foo: locked"
	out := m.View()
	if !strings.Contains(out, "could not delete foo: locked") {
		t.Errorf("View should render toast text; got:\n%s", out)
	}
	// Normal help line should NOT appear while toast is up.
	if strings.Contains(out, "d delete   r rescan") {
		t.Error("help line should be replaced by toast")
	}
}

// stripANSI removes ANSI CSI escape sequences for plain-text length checks.
func stripANSI(s string) string {
	out := make([]rune, 0, len(s))
	in := []rune(s)
	for i := 0; i < len(in); i++ {
		if in[i] == 0x1b && i+1 < len(in) && in[i+1] == '[' {
			// skip until letter
			i += 2
			for i < len(in) && !((in[i] >= 'A' && in[i] <= 'Z') || (in[i] >= 'a' && in[i] <= 'z')) {
				i++
			}
			continue
		}
		out = append(out, in[i])
	}
	return string(out)
}
