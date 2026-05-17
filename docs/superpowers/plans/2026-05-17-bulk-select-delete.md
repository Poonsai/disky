# Bulk Select for Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-folder multi-select to the browser so users can recycle several files/folders in one batch.

**Architecture:** New `Selected map[*tree.Node]struct{}` on `BrowserModel`. Space/Shift-arrow/J/K/a/A drive the selection. `d` checks for a non-empty selection and falls back to the cursor row when empty. `main.handleDelete` iterates the batch best-effort and reports a summary toast.

**Tech Stack:** Go 1.22+, Bubble Tea v1.3.10, Lipgloss v1.1.0. Windows-only (no other platforms in this app).

**Spec:** [docs/superpowers/specs/2026-05-17-bulk-select-delete-design.md](../specs/2026-05-17-bulk-select-delete-design.md)

---

## File Structure

| File | Role | Change |
|---|---|---|
| `internal/tui/confirm.go` | Confirm dialog | Multi-item API (`Items []ConfirmItem`), bulleted view with truncation |
| `internal/tui/confirm_test.go` | Confirm tests | New tests for single/multi/many-item display |
| `internal/tui/browser.go` | Browser model + view | `Selected` field, new key handlers, visual marker, `ApplyBatchDelete`, `ApplyRescan` clears selection, help line |
| `internal/tui/browser_test.go` | Browser tests | New tests for every selection behavior + regression |
| `cmd/disky/main.go` | Orchestration | `handleDelete` iterates the batch, builds summary toast |

---

## Task 1: Multi-item ConfirmModel API

**Files:**
- Modify: `internal/tui/confirm.go`
- Modify: `internal/tui/confirm_test.go`
- Modify: `cmd/disky/main.go`

- [ ] **Step 1: Add regression test for single-item view**

Open `internal/tui/confirm_test.go`. The existing `TestConfirmPluralizesItemCount` already covers the single-item path. Add a new test that asserts the bullet-style multi-item view doesn't apply when there's only one item:

```go
func TestConfirmSingleItemViewUnchanged(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\foo", Size: 1024, ItemCount: 0}})
	out := m.View()
	// Single-item path renders the bare path + size; no bullet, no count summary.
	if strings.Contains(out, "•") {
		t.Errorf("single item should not show bullets; got:\n%s", out)
	}
	if !strings.Contains(out, "C:\\foo") {
		t.Errorf("path missing from single-item view:\n%s", out)
	}
	if !strings.Contains(out, "1.0 KB") {
		t.Errorf("size missing from single-item view:\n%s", out)
	}
}
```

Also update the existing tests to use the new `[]ConfirmItem` signature. The complete replacement file body:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmEnterConfirms(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\big.bin", Size: 1024}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	c := next.(ConfirmModel)
	if c.Result != ConfirmYes {
		t.Errorf("Result: got %v, want ConfirmYes", c.Result)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\big.bin", Size: 1024}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	c := next.(ConfirmModel)
	if c.Result != ConfirmNo {
		t.Errorf("Result: got %v, want ConfirmNo", c.Result)
	}
}

func TestConfirmInitialPending(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\big.bin", Size: 1024}})
	if m.Result != ConfirmPending {
		t.Errorf("initial Result: got %v, want ConfirmPending", m.Result)
	}
}

func TestConfirmPluralizesItemCount(t *testing.T) {
	cases := []struct {
		count    int
		wantSub  string
		negative string
	}{
		{1, "1 item, ", "1 items"},
		{3, "3 items, ", "3 item,"},
		{0, "1.0 KB", "items"}, // file path: no item count rendered at all
	}
	for _, c := range cases {
		m := NewConfirm([]ConfirmItem{{Path: "C:\\thing", Size: 1024, ItemCount: c.count}})
		out := m.View()
		if !strings.Contains(out, c.wantSub) {
			t.Errorf("count=%d: view missing %q\n%s", c.count, c.wantSub, out)
		}
		if strings.Contains(out, c.negative) {
			t.Errorf("count=%d: view should not contain %q\n%s", c.count, c.negative, out)
		}
	}
}

func TestConfirmSingleItemViewUnchanged(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\foo", Size: 1024, ItemCount: 0}})
	out := m.View()
	if strings.Contains(out, "•") {
		t.Errorf("single item should not show bullets; got:\n%s", out)
	}
	if !strings.Contains(out, "C:\\foo") {
		t.Errorf("path missing from single-item view:\n%s", out)
	}
	if !strings.Contains(out, "1.0 KB") {
		t.Errorf("size missing from single-item view:\n%s", out)
	}
}

func TestConfirmMultiItemSummary(t *testing.T) {
	items := []ConfirmItem{
		{Path: "C:\\a", Size: 500},
		{Path: "C:\\b", Size: 1500},
		{Path: "C:\\c", Size: 2048},
	}
	m := NewConfirm(items)
	out := m.View()
	// Header summary: count + total size (500 + 1500 + 2048 = 4048 bytes = 3.95 KB rounded).
	if !strings.Contains(out, "3 items") {
		t.Errorf("multi-item view missing count; got:\n%s", out)
	}
	// Total size rendered via tree.FormatSize (4048 bytes → "4.0 KB" with the
	// 1024 unit divisor used elsewhere).
	if !strings.Contains(out, "4.0 KB") {
		t.Errorf("multi-item view missing total size; got:\n%s", out)
	}
	for _, want := range []string{"• C:\\a", "• C:\\b", "• C:\\c"} {
		if !strings.Contains(out, want) {
			t.Errorf("multi-item view missing bullet %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "and 0 more") {
		t.Errorf("no overflow expected; got:\n%s", out)
	}
}

func TestConfirmMultiItemTruncatesPaths(t *testing.T) {
	var items []ConfirmItem
	for i := 0; i < 12; i++ {
		items = append(items, ConfirmItem{Path: "C:\\item-" + string(rune('a'+i)), Size: 100})
	}
	m := NewConfirm(items)
	out := m.View()
	if !strings.Contains(out, "12 items") {
		t.Errorf("count missing; got:\n%s", out)
	}
	if !strings.Contains(out, "• C:\\item-a") {
		t.Errorf("first bullet missing; got:\n%s", out)
	}
	if !strings.Contains(out, "• C:\\item-e") {
		t.Errorf("fifth bullet (item-e) missing; got:\n%s", out)
	}
	if strings.Contains(out, "• C:\\item-f") {
		t.Errorf("sixth bullet should be truncated; got:\n%s", out)
	}
	if !strings.Contains(out, "... and 7 more") {
		t.Errorf("overflow line missing; got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail with the old API**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run TestConfirm -v`

Expected: build error citing `NewConfirm` signature mismatch.

- [ ] **Step 3: Rewrite `internal/tui/confirm.go`**

Replace the whole file with:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Poonsai/disky/internal/tree"
)

type ConfirmResult int

const (
	ConfirmPending ConfirmResult = iota
	ConfirmYes
	ConfirmNo
)

// ConfirmItem is one entry in a (possibly multi-item) recycle confirmation.
// ItemCount is the count of descendants for directories, or 0 for files.
type ConfirmItem struct {
	Path      string
	Size      int64
	ItemCount int
}

type ConfirmModel struct {
	Items  []ConfirmItem // length >= 1
	Result ConfirmResult
}

// maxBullets is the upper bound on per-path bullets in the multi-item view.
// Items beyond this are summarized as "... and N more".
const maxBullets = 5

func NewConfirm(items []ConfirmItem) ConfirmModel {
	return ConfirmModel{Items: items}
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "enter":
			m.Result = ConfirmYes
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.Result = ConfirmNo
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	var b strings.Builder
	b.WriteString(StyleTitle.Render("Move to Recycle Bin?") + "\n\n")
	if len(m.Items) == 1 {
		it := m.Items[0]
		b.WriteString("  " + it.Path + "\n")
		if it.ItemCount > 0 {
			noun := "items"
			if it.ItemCount == 1 {
				noun = "item"
			}
			b.WriteString(fmt.Sprintf("  %d %s, %s\n", it.ItemCount, noun, tree.FormatSize(it.Size)))
		} else {
			b.WriteString("  " + tree.FormatSize(it.Size) + "\n")
		}
	} else {
		var total int64
		for _, it := range m.Items {
			total += it.Size
		}
		b.WriteString(fmt.Sprintf("  %d items (%s)\n\n", len(m.Items), tree.FormatSize(total)))
		shown := len(m.Items)
		if shown > maxBullets {
			shown = maxBullets
		}
		for i := 0; i < shown; i++ {
			b.WriteString("  • " + m.Items[i].Path + "\n")
		}
		if len(m.Items) > maxBullets {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(m.Items)-maxBullets))
		}
	}
	b.WriteString("\n" + StyleHelp.Render("[Enter] confirm    [Esc] cancel") + "\n")
	return b.String()
}
```

- [ ] **Step 4: Update `cmd/disky/main.go` to use the new API**

The relevant section is in `handleDelete` (around lines 108-130). Replace the construction of `cm` with:

```go
items := []tui.ConfirmItem{{Path: target.Path(), Size: target.Size, ItemCount: itemCount}}
cm := tui.NewConfirm(items)
```

- [ ] **Step 5: Run the full test suite**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./... -count=1`

Expected: all pass.

- [ ] **Step 6: Commit**

```powershell
git add internal/tui/confirm.go internal/tui/confirm_test.go cmd/disky/main.go
git commit -m "refactor(tui): generalize ConfirmModel to a list of items

Single-item rendering is byte-for-byte unchanged. Multi-item view
will be used by the upcoming bulk-delete flow.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Selection state + Space toggle + visual marker

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/browser_test.go`:

```go
func TestBrowserSpaceTogglesCursorItem(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Toggle on (cursor at row 0 = "sub").
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	bm := next.(BrowserModel)
	if _, ok := bm.Selected[bm.Current.Children[0]]; !ok {
		t.Errorf("Selected should include the cursor row after Space")
	}
	// Toggle off — same key on the same row.
	next, _ = bm.Update(tea.KeyMsg{Type: tea.KeySpace})
	bm = next.(BrowserModel)
	if _, ok := bm.Selected[bm.Current.Children[0]]; ok {
		t.Errorf("Selected should be empty after second Space on same row")
	}
}

func TestBrowserSelectedRowRendersStarMarker(t *testing.T) {
	m := NewBrowser(sampleTree())
	m.Width = 80
	m.Height = 24
	// Move cursor off row 0 so the marker isn't masked by cursor styling.
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = down.(BrowserModel)
	space, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = space.(BrowserModel)
	// Move cursor back to row 0 so the selection on row 1 ("a.txt") is the
	// only star marker in the view.
	up, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = up.(BrowserModel)

	out := m.View()
	// The selected row (a.txt) should carry a "* " marker.
	if !strings.Contains(out, "*   a.txt") && !strings.Contains(out, "* a.txt") {
		t.Errorf("selected row missing star marker; got:\n%s", out)
	}
}

func TestBrowserUnselectedRowsHaveBlankMarkerSlot(t *testing.T) {
	// Sanity: when nothing is selected, no rows should carry a '*' anywhere.
	m := NewBrowser(sampleTree())
	m.Width = 80
	m.Height = 24
	out := m.View()
	if strings.Contains(out, "* ") && !strings.Contains(out, "Space select") {
		// help line could legitimately contain "Space select" but no "* ".
		t.Errorf("no row should show a star marker; got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run "TestBrowserSpaceTogglesCursorItem|TestBrowserSelectedRowRendersStarMarker|TestBrowserUnselectedRowsHaveBlankMarkerSlot" -v`

Expected: FAIL — `Selected` field undefined.

- [ ] **Step 3: Add `Selected` field on BrowserModel**

Modify the struct declaration in `internal/tui/browser.go` (around lines 12-21):

```go
type BrowserModel struct {
	Root          *tree.Node
	Current       *tree.Node
	Cursor        int
	Offset        int                       // first visible index in Current.Children
	Width, Height int                       // terminal size (set by tea.WindowSizeMsg)
	Selected      map[*tree.Node]struct{}   // multi-select set; nil/empty = no selection
	PendingDelete *tree.Node                // set when the user pressed 'd'; cleared on Apply/Cancel
	PendingRescan bool
	Toast         string // transient error message; rendered in help slot, cleared on any key
}
```

- [ ] **Step 4: Wire the Space handler**

In the `Update` method's key switch (around lines 80-127), add a `case " ":` branch before the `case "q", "esc", "ctrl+c":` line:

```go
		case " ":
			if m.Cursor < len(m.Current.Children) {
				node := m.Current.Children[m.Cursor]
				if m.Selected == nil {
					m.Selected = map[*tree.Node]struct{}{}
				}
				if _, ok := m.Selected[node]; ok {
					delete(m.Selected, node)
				} else {
					m.Selected[node] = struct{}{}
				}
			}
```

- [ ] **Step 5: Add the 2-cell selection slot to the row layout**

Locate the `View()` method's row-rendering block (around lines 178-222). Change `const fixedCells = 16 + BarWidth` to `const fixedCells = 18 + BarWidth` and update the format strings to include a `selMarker` cell.

Replace the row-rendering loop body. For the cursor row (around line 195-208):

```go
				if i == m.Cursor {
					plainMarker := ""
					if c.Err != nil {
						plainMarker = "[!] "
					}
					selMarker := "  "
					if _, ok := m.Selected[c]; ok {
						selMarker = "* "
					}
					row := fmt.Sprintf("  %10s  %s  %s%s%s",
						tree.FormatSize(c.Size),
						barPlain(float64(c.Size)/float64(maxSize)),
						selMarker,
						plainMarker,
						name,
					)
					b.WriteString(StyleSelected.Render(row) + "\n")
					continue
				}
```

For the non-cursor row (around lines 210-221):

```go
				marker := ""
				if c.Err != nil {
					marker = StyleError.Render("[!] ")
				}
				selMarker := "  "
				if _, ok := m.Selected[c]; ok {
					selMarker = "* "
				}
				row := fmt.Sprintf("  %10s  %s  %s%s%s",
					tree.FormatSize(c.Size),
					Bar(float64(c.Size)/float64(maxSize)),
					selMarker,
					marker,
					name,
				)
				b.WriteString(row + "\n")
```

- [ ] **Step 6: Run the tests again**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -count=1`

Expected: all pass, including the three new tests and all existing ones (the row truncation/long-name test still passes because the marker is only 2 extra cells).

- [ ] **Step 7: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go
git commit -m "feat(tui): selection state + Space toggle + star marker

Adds a Selected map on BrowserModel, Space-to-toggle handler, and a
2-cell selection marker slot in each row. Marker only visible on
selected rows.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Select-all (`a`) and clear-all (`A`)

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/browser_test.go`:

```go
func TestBrowserSelectAllPopulatesSelected(t *testing.T) {
	m := NewBrowser(sampleTree()) // 2 children at root
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if len(bm.Selected) != len(bm.Current.Children) {
		t.Errorf("Selected size: got %d, want %d", len(bm.Selected), len(bm.Current.Children))
	}
	for _, c := range bm.Current.Children {
		if _, ok := bm.Selected[c]; !ok {
			t.Errorf("child %q not in Selected", c.Name)
		}
	}
}

func TestBrowserSelectAllOnEmptyFolderIsNoop(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	m := NewBrowser(root)
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if len(bm.Selected) != 0 {
		t.Errorf("Selected should remain empty in empty folder; got %d", len(bm.Selected))
	}
}

func TestBrowserShiftAClearsSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Pre-populate via 'a'.
	aNext, _ := m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m = aNext.(BrowserModel)
	if len(m.Selected) == 0 {
		t.Fatal("precondition: select-all should have populated Selected")
	}
	// Clear via 'A'.
	bigA, _ := m.Update(tea.KeyMsg{Runes: []rune("A"), Type: tea.KeyRunes})
	m = bigA.(BrowserModel)
	if len(m.Selected) != 0 {
		t.Errorf("Selected should be empty after 'A'; got %d", len(m.Selected))
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run "TestBrowserSelectAllPopulatesSelected|TestBrowserSelectAllOnEmptyFolderIsNoop|TestBrowserShiftAClearsSelection" -v`

Expected: FAIL — `a` and `A` not handled.

- [ ] **Step 3: Add `a` and `A` handlers**

In `internal/tui/browser.go`'s `Update` switch, immediately after the `case " ":` block, add:

```go
		case "a":
			if len(m.Current.Children) > 0 {
				if m.Selected == nil {
					m.Selected = map[*tree.Node]struct{}{}
				}
				for _, c := range m.Current.Children {
					m.Selected[c] = struct{}{}
				}
			}
		case "A":
			m.Selected = nil
```

- [ ] **Step 4: Run the tests**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -count=1`

Expected: all pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go
git commit -m "feat(tui): 'a' select-all and 'A' clear-all

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Navigation clears the selection

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/browser_test.go`:

```go
func TestBrowserEnterFolderClearsSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Select the row that will become the new Current (sub) — the selection
	// must clear when we navigate INTO it.
	sp, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = sp.(BrowserModel)
	if len(m.Selected) != 1 {
		t.Fatal("precondition: Selected should have 1 item")
	}
	en, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bm := en.(BrowserModel)
	if len(bm.Selected) != 0 {
		t.Errorf("Selected should be cleared after entering folder; got %d", len(bm.Selected))
	}
}

func TestBrowserBackClearsSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Navigate into sub, select something, navigate back.
	en, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = en.(BrowserModel)
	sp, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = sp.(BrowserModel)
	if len(m.Selected) != 1 {
		t.Fatal("precondition: Selected should have 1 item in sub")
	}
	bk, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	bm := bk.(BrowserModel)
	if len(bm.Selected) != 0 {
		t.Errorf("Selected should be cleared after going back; got %d", len(bm.Selected))
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run "TestBrowserEnterFolderClearsSelection|TestBrowserBackClearsSelection" -v`

Expected: FAIL — selection survives navigation.

- [ ] **Step 3: Clear selection in navigation handlers**

In `internal/tui/browser.go`'s `Update`, modify the existing `enter, right` and `backspace, left, h` handlers. Replace the existing blocks with:

```go
		case "enter", "right":
			if m.Cursor < len(m.Current.Children) {
				sel := m.Current.Children[m.Cursor]
				if sel.IsDir {
					m.Current = sel
					m.Cursor = 0
					m.Selected = nil
				}
			}
		case "backspace", "left", "h":
			if m.Current.Parent != nil && m.Current != m.Root.Parent {
				// Find our index in parent.Children to restore cursor.
				parent := m.Current.Parent
				idx := 0
				for i, c := range parent.Children {
					if c == m.Current {
						idx = i
						break
					}
				}
				m.Current = parent
				m.Cursor = idx
				m.Selected = nil
			}
```

- [ ] **Step 4: Run the tests**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -count=1`

Expected: all pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go
git commit -m "feat(tui): clear selection when navigating between folders

Selection scope is per-folder by design (see spec). Entering a
subfolder or going back to the parent clears the Selected set so
stale pointers can't cause surprises.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Range-select via Shift+↑/↓ and J/K

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/browser_test.go`:

```go
func TestBrowserShiftDownExtendsSelection(t *testing.T) {
	m := NewBrowser(wideTree(5))
	// Cursor at row 0. Shift+Down should select row 0, advance to row 1, and
	// select row 1.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	bm := next.(BrowserModel)
	if bm.Cursor != 1 {
		t.Errorf("Cursor after Shift+Down: got %d, want 1", bm.Cursor)
	}
	if _, ok := bm.Selected[bm.Current.Children[0]]; !ok {
		t.Errorf("row 0 should be selected after Shift+Down")
	}
	if _, ok := bm.Selected[bm.Current.Children[1]]; !ok {
		t.Errorf("row 1 should be selected after Shift+Down")
	}
}

func TestBrowserShiftDownTwiceExtendsByOne(t *testing.T) {
	m := NewBrowser(wideTree(5))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	next, _ = next.(BrowserModel).Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	bm := next.(BrowserModel)
	if bm.Cursor != 2 {
		t.Errorf("Cursor after two Shift+Down: got %d, want 2", bm.Cursor)
	}
	for i := 0; i <= 2; i++ {
		if _, ok := bm.Selected[bm.Current.Children[i]]; !ok {
			t.Errorf("row %d should be selected", i)
		}
	}
}

func TestBrowserShiftUpExtendsSelection(t *testing.T) {
	m := NewBrowser(wideTree(5))
	// Move cursor to row 3 via plain Down.
	for i := 0; i < 3; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(BrowserModel)
	}
	if len(m.Selected) != 0 {
		t.Fatal("plain Down must not select")
	}
	su, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	bm := su.(BrowserModel)
	if bm.Cursor != 2 {
		t.Errorf("Cursor after Shift+Up: got %d, want 2", bm.Cursor)
	}
	for _, i := range []int{2, 3} {
		if _, ok := bm.Selected[bm.Current.Children[i]]; !ok {
			t.Errorf("row %d should be selected after Shift+Up", i)
		}
	}
}

func TestBrowserShiftDownAtBottomSelectsButDoesNotMove(t *testing.T) {
	m := NewBrowser(wideTree(3))
	// Jump cursor to last row.
	g, _ := m.Update(tea.KeyMsg{Runes: []rune("G"), Type: tea.KeyRunes})
	m = g.(BrowserModel)
	last := m.Cursor
	sd, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	bm := sd.(BrowserModel)
	if bm.Cursor != last {
		t.Errorf("Cursor should stay at last row; got %d, want %d", bm.Cursor, last)
	}
	if _, ok := bm.Selected[bm.Current.Children[last]]; !ok {
		t.Errorf("last row should still be selected")
	}
}

func TestBrowserShiftUpAtTopSelectsButDoesNotMove(t *testing.T) {
	m := NewBrowser(wideTree(3))
	// Cursor starts at 0.
	su, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	bm := su.(BrowserModel)
	if bm.Cursor != 0 {
		t.Errorf("Cursor should stay at 0; got %d", bm.Cursor)
	}
	if _, ok := bm.Selected[bm.Current.Children[0]]; !ok {
		t.Errorf("row 0 should still be selected")
	}
}

func TestBrowserUppercaseJBehavesAsShiftDown(t *testing.T) {
	m := NewBrowser(wideTree(5))
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("J"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if bm.Cursor != 1 {
		t.Errorf("Cursor after J: got %d, want 1", bm.Cursor)
	}
	for i := 0; i <= 1; i++ {
		if _, ok := bm.Selected[bm.Current.Children[i]]; !ok {
			t.Errorf("row %d should be selected after J", i)
		}
	}
}

func TestBrowserUppercaseKBehavesAsShiftUp(t *testing.T) {
	m := NewBrowser(wideTree(5))
	for i := 0; i < 2; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(BrowserModel)
	}
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("K"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if bm.Cursor != 1 {
		t.Errorf("Cursor after K: got %d, want 1", bm.Cursor)
	}
	for _, i := range []int{1, 2} {
		if _, ok := bm.Selected[bm.Current.Children[i]]; !ok {
			t.Errorf("row %d should be selected after K", i)
		}
	}
}

func TestBrowserPlainArrowDoesNotSelect(t *testing.T) {
	m := NewBrowser(wideTree(3))
	for _, k := range []tea.KeyType{tea.KeyDown, tea.KeyUp, tea.KeyDown} {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		m = next.(BrowserModel)
	}
	if len(m.Selected) != 0 {
		t.Errorf("plain navigation must not select; got %d", len(m.Selected))
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run "TestBrowserShift|TestBrowserUppercaseJBehavesAsShiftDown|TestBrowserUppercaseKBehavesAsShiftUp|TestBrowserPlainArrowDoesNotSelect" -v`

Expected: FAIL — shift+arrow / J / K not handled.

- [ ] **Step 3: Add range-select handlers**

In `internal/tui/browser.go`'s `Update` switch, immediately after the `case "A":` block, add:

```go
		case "shift+down", "J":
			if len(m.Current.Children) > 0 {
				if m.Selected == nil {
					m.Selected = map[*tree.Node]struct{}{}
				}
				m.Selected[m.Current.Children[m.Cursor]] = struct{}{}
				if m.Cursor < len(m.Current.Children)-1 {
					m.Cursor++
					m.Selected[m.Current.Children[m.Cursor]] = struct{}{}
				}
			}
		case "shift+up", "K":
			if len(m.Current.Children) > 0 {
				if m.Selected == nil {
					m.Selected = map[*tree.Node]struct{}{}
				}
				m.Selected[m.Current.Children[m.Cursor]] = struct{}{}
				if m.Cursor > 0 {
					m.Cursor--
					m.Selected[m.Current.Children[m.Cursor]] = struct{}{}
				}
			}
```

- [ ] **Step 4: Run the tests**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -count=1`

Expected: all pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go
git commit -m "feat(tui): range-select via Shift+arrow and J/K

Shift+Down/Up extends the selection one row at a time. Uppercase
J/K is the vim-style equivalent for terminals that don't report
modifier keys (legacy console, ssh sessions, etc.).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: PendingDelete → PendingDeletes (rename) and 'd' uses selection if any

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`
- Modify: `cmd/disky/main.go`

- [ ] **Step 1: Write the failing tests**

Replace the existing `TestBrowserDeleteRequest` and `TestBrowserCancelDelete` in `internal/tui/browser_test.go` with these, and add the new selection-aware tests:

```go
func TestBrowserDeleteRequestUsesCursorWhenNoSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("d"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if len(bm.PendingDeletes) != 1 {
		t.Fatalf("PendingDeletes length: got %d, want 1", len(bm.PendingDeletes))
	}
	if bm.PendingDeletes[0].Name != "sub" {
		t.Errorf("PendingDeletes[0].Name: got %q, want %q", bm.PendingDeletes[0].Name, "sub")
	}
}

func TestBrowserDeleteRequestUsesSelectionWhenNonEmpty(t *testing.T) {
	m := NewBrowser(sampleTree())
	// Select both children via 'a'.
	a, _ := m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m = a.(BrowserModel)
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("d"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if len(bm.PendingDeletes) != 2 {
		t.Fatalf("PendingDeletes length: got %d, want 2", len(bm.PendingDeletes))
	}
	// Order must match Current.Children display order.
	if bm.PendingDeletes[0] != bm.Current.Children[0] || bm.PendingDeletes[1] != bm.Current.Children[1] {
		t.Errorf("PendingDeletes order does not match Current.Children order")
	}
}

func TestBrowserDeleteOnEmptyFolderIsNoop(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	m := NewBrowser(root)
	next, _ := m.Update(tea.KeyMsg{Runes: []rune("d"), Type: tea.KeyRunes})
	bm := next.(BrowserModel)
	if len(bm.PendingDeletes) != 0 {
		t.Errorf("PendingDeletes should be empty on empty folder; got %d", len(bm.PendingDeletes))
	}
}

func TestBrowserCancelDeleteClearsPendingButKeepsSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	a, _ := m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m = a.(BrowserModel)
	d, _ := m.Update(tea.KeyMsg{Runes: []rune("d"), Type: tea.KeyRunes})
	m = d.(BrowserModel)
	m = m.CancelDelete()
	if len(m.PendingDeletes) != 0 {
		t.Errorf("PendingDeletes should be cleared after Cancel")
	}
	if len(m.Selected) != 2 {
		t.Errorf("Selection should be preserved after Cancel; got %d", len(m.Selected))
	}
}
```

Also remove the existing `TestBrowserDeleteRequest` and `TestBrowserCancelDelete` from the file (the replacements above cover them more thoroughly).

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run "TestBrowserDelete|TestBrowserCancel" -v`

Expected: build error — `PendingDeletes` field undefined.

- [ ] **Step 3: Rename the field and update key handler**

In `internal/tui/browser.go`, change `PendingDelete *tree.Node` to `PendingDeletes []*tree.Node` in the struct (keep all other field positions identical).

Replace the existing `case "d":` block with:

```go
		case "d":
			if len(m.Selected) > 0 {
				// Take the selection in current-folder display order so the
				// confirm dialog and recycle loop see a predictable order.
				var batch []*tree.Node
				for _, c := range m.Current.Children {
					if _, ok := m.Selected[c]; ok {
						batch = append(batch, c)
					}
				}
				m.PendingDeletes = batch
				return m, tea.Quit
			}
			if m.Cursor < len(m.Current.Children) {
				m.PendingDeletes = []*tree.Node{m.Current.Children[m.Cursor]}
				return m, tea.Quit
			}
```

Update `CancelDelete` to clear the new field (without touching Selected):

```go
// CancelDelete clears the pending request without touching the tree.
// Selection (if any) is preserved so the user can adjust and retry.
func (m BrowserModel) CancelDelete() BrowserModel {
	m.PendingDeletes = nil
	return m
}
```

- [ ] **Step 4: Update `cmd/disky/main.go`**

In the `browse` switch (around lines 97-104), replace:

```go
		case bm.PendingDelete != nil:
			bm = handleDelete(bm)
```

with:

```go
		case len(bm.PendingDeletes) > 0:
			bm = handleDelete(bm)
```

In `handleDelete` (around lines 108-130), replace the body with the multi-item-aware version. Note: this step only wires the dispatch and confirm-dialog input. The actual batch loop (with per-item recycle calls and a summary toast) comes in Task 7 — for now, keep behavior identical to the single-item path when `len(PendingDeletes) == 1`. Use a TEMPORARY single-item branch:

```go
func handleDelete(bm tui.BrowserModel) tui.BrowserModel {
	targets := bm.PendingDeletes

	// Build confirm items.
	items := make([]tui.ConfirmItem, 0, len(targets))
	for _, t := range targets {
		count := 0
		if t.IsDir {
			count = countItems(t)
		}
		items = append(items, tui.ConfirmItem{Path: t.Path(), Size: t.Size, ItemCount: count})
	}

	cm := tui.NewConfirm(items)
	final, err := tea.NewProgram(cm, tea.WithAltScreen()).Run()
	if err != nil {
		return bm.CancelDelete()
	}
	if final.(tui.ConfirmModel).Result != tui.ConfirmYes {
		return bm.CancelDelete()
	}

	// Single-item path keeps the old behavior — gives Task 7 a clean diff
	// to extend into a batch loop.
	if len(targets) == 1 {
		target := targets[0]
		if err := recycle.Send(target.Path()); err != nil {
			bm = bm.CancelDelete()
			bm.Toast = fmt.Sprintf("could not delete %s: %v", target.Name, err)
			return bm
		}
		return bm.ApplyDelete(target)
	}

	// Multi-item: stub for now — clear pending, set a toast pointing at the
	// follow-up task. Replaced in Task 7.
	bm = bm.CancelDelete()
	bm.Toast = "multi-delete not yet wired (Task 7)"
	return bm
}
```

- [ ] **Step 5: Run the tests**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./... -count=1`

Expected: all pass. `go build ./...` should also succeed (verify with `go build ./...`).

- [ ] **Step 6: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go cmd/disky/main.go
git commit -m "feat(tui): 'd' uses selection if any, else falls back to cursor

Renames PendingDelete -> PendingDeletes and switches the browser/main
dispatch to a slice. Multi-item branch in main is stubbed; Task 7
wires the actual batch recycle loop.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: ApplyBatchDelete + main.go batch loop + summary toast + ApplyRescan clears selection

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`
- Modify: `cmd/disky/main.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/browser_test.go`:

```go
func TestBrowserApplyBatchDeleteRemovesSucceededOnly(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true, Size: 60}
	a := &tree.Node{Name: "a", Size: 10, Parent: root}
	b := &tree.Node{Name: "b", Size: 20, Parent: root}
	c := &tree.Node{Name: "c", Size: 30, Parent: root}
	root.Children = []*tree.Node{c, b, a} // sorted descending by size

	m := NewBrowser(root)
	m.Selected = map[*tree.Node]struct{}{a: {}, c: {}}
	// Simulate: c succeeded, a failed.
	m = m.ApplyBatchDelete([]*tree.Node{c})

	// c removed, a and b remain.
	if len(m.Current.Children) != 2 {
		t.Fatalf("Current.Children count: got %d, want 2", len(m.Current.Children))
	}
	// a stays in Selected (still selected — user may retry).
	if _, ok := m.Selected[a]; !ok {
		t.Errorf("failed item 'a' should remain in Selected")
	}
	// c is gone from Selected (it was successfully deleted).
	if _, ok := m.Selected[c]; ok {
		t.Errorf("succeeded item 'c' should be removed from Selected")
	}
	if len(m.PendingDeletes) != 0 {
		t.Errorf("PendingDeletes should be cleared")
	}
}

func TestBrowserApplyRescanClearsSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	a, _ := m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m = a.(BrowserModel)
	if len(m.Selected) == 0 {
		t.Fatal("precondition: Selected should be populated")
	}
	m.PendingRescan = true

	newCur := &tree.Node{Name: `C:\`, IsDir: true, Size: 1}
	newCur.Children = []*tree.Node{{Name: "fresh", Size: 1}}

	m = m.ApplyRescan(newCur)
	if len(m.Selected) != 0 {
		t.Errorf("ApplyRescan must clear Selected (stale pointers); got %d", len(m.Selected))
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run "TestBrowserApplyBatchDeleteRemovesSucceededOnly|TestBrowserApplyRescanClearsSelection" -v`

Expected: FAIL — `ApplyBatchDelete` undefined; `ApplyRescan` doesn't touch `Selected`.

- [ ] **Step 3: Add `ApplyBatchDelete` and update `ApplyRescan`**

Append to `internal/tui/browser.go` (next to `ApplyDelete`):

```go
// ApplyBatchDelete removes each successfully-recycled node from the tree,
// drops it from the Selected set, and clears PendingDeletes. Nodes that
// failed (i.e. are NOT in succeeded) stay in the tree and stay selected
// so the user can adjust and retry. Caller is responsible for surfacing
// a Toast summarizing successes vs failures.
func (m BrowserModel) ApplyBatchDelete(succeeded []*tree.Node) BrowserModel {
	for _, n := range succeeded {
		tree.RemoveAndRecompute(n)
		delete(m.Selected, n)
	}
	tree.Sort(m.Current)
	tree.SortAncestors(m.Current)
	if m.Cursor >= len(m.Current.Children) && m.Cursor > 0 {
		m.Cursor = len(m.Current.Children) - 1
	}
	m.PendingDeletes = nil
	return m.adjustOffset()
}
```

Modify `ApplyRescan` (around lines 295-316). Find the tail of the function:

```go
	if m.Cursor >= len(m.Current.Children) {
		m.Cursor = 0
	}
	m.PendingRescan = false
	return m.adjustOffset()
```

Replace with:

```go
	if m.Cursor >= len(m.Current.Children) {
		m.Cursor = 0
	}
	// Selection holds pointers into the OLD children, which we just
	// replaced. Stale pointers would point at orphaned tree nodes.
	m.Selected = nil
	m.PendingRescan = false
	return m.adjustOffset()
```

- [ ] **Step 4: Run the browser tests**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -count=1`

Expected: all pass.

- [ ] **Step 5: Replace the main.go multi-item stub with the real batch loop**

In `cmd/disky/main.go` `handleDelete`, replace the stub branch:

```go
	// Multi-item: stub for now — clear pending, set a toast pointing at the
	// follow-up task. Replaced in Task 7.
	bm = bm.CancelDelete()
	bm.Toast = "multi-delete not yet wired (Task 7)"
	return bm
```

with the real loop:

```go
	var succeeded []*tree.Node
	var failed []string
	for _, t := range targets {
		if err := recycle.Send(t.Path()); err != nil {
			failed = append(failed, t.Name)
			continue
		}
		succeeded = append(succeeded, t)
	}
	bm = bm.ApplyBatchDelete(succeeded)
	if len(failed) > 0 {
		bm.Toast = formatBatchFailure(len(succeeded), len(targets), failed)
	}
	return bm
```

Also remove the now-unnecessary `if len(targets) == 1 { ... }` shortcut block — the batch loop handles N=1 correctly too. The final `handleDelete` should be:

```go
func handleDelete(bm tui.BrowserModel) tui.BrowserModel {
	targets := bm.PendingDeletes

	items := make([]tui.ConfirmItem, 0, len(targets))
	for _, t := range targets {
		count := 0
		if t.IsDir {
			count = countItems(t)
		}
		items = append(items, tui.ConfirmItem{Path: t.Path(), Size: t.Size, ItemCount: count})
	}

	cm := tui.NewConfirm(items)
	final, err := tea.NewProgram(cm, tea.WithAltScreen()).Run()
	if err != nil {
		return bm.CancelDelete()
	}
	if final.(tui.ConfirmModel).Result != tui.ConfirmYes {
		return bm.CancelDelete()
	}

	var succeeded []*tree.Node
	var failed []string
	for _, t := range targets {
		if err := recycle.Send(t.Path()); err != nil {
			failed = append(failed, t.Name)
			continue
		}
		succeeded = append(succeeded, t)
	}
	bm = bm.ApplyBatchDelete(succeeded)
	if len(failed) > 0 {
		bm.Toast = formatBatchFailure(len(succeeded), len(targets), failed)
	}
	return bm
}

// formatBatchFailure produces a one-line summary toast for a batch
// recycle. Lists up to 3 failed names, summarizing the rest.
func formatBatchFailure(succeeded, total int, failedNames []string) string {
	const maxNames = 3
	shown := failedNames
	tail := ""
	if len(failedNames) > maxNames {
		shown = failedNames[:maxNames]
		tail = fmt.Sprintf(" and %d more", len(failedNames)-maxNames)
	}
	return fmt.Sprintf("deleted %d of %d; failed: %s%s",
		succeeded, total, strings.Join(shown, ", "), tail)
}
```

This requires adding `"strings"` to the import block in `cmd/disky/main.go`. The full import block should be:

```go
import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Poonsai/disky/internal/drives"
	"github.com/Poonsai/disky/internal/recycle"
	"github.com/Poonsai/disky/internal/tree"
	"github.com/Poonsai/disky/internal/tui"
)
```

- [ ] **Step 6: Run all tests + build**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go build ./...; go test ./... -count=1`

Expected: build OK, all tests pass.

- [ ] **Step 7: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go cmd/disky/main.go
git commit -m "feat(tui): batch recycle with summary toast

ApplyBatchDelete removes succeeded items from the tree and trims the
Selected set; failed items stay selected for retry. main.handleDelete
loops over PendingDeletes, calls recycle.Send per item, builds a
toast like 'deleted 7 of 9; failed: foo, bar'. ApplyRescan now also
clears the selection because its pointers refer to the old subtree.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Help text update + final sweep

**Files:**
- Modify: `internal/tui/browser.go`
- Modify: `internal/tui/browser_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write a failing test for the help line**

Append to `internal/tui/browser_test.go`:

```go
func TestBrowserHelpLineMentionsSelection(t *testing.T) {
	m := NewBrowser(sampleTree())
	m.Width = 200 // wide enough to render the full help line
	m.Height = 24
	out := m.View()
	for _, want := range []string{"navigate", "Space select", "Shift+", "a all", "A clear", "d delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("help line missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "↑/↓ select") {
		t.Errorf("help line still says '↑/↓ select'; should say 'navigate'")
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -run TestBrowserHelpLineMentionsSelection -v`

Expected: FAIL — current help line still says `↑/↓ select`.

- [ ] **Step 3: Update the help line**

In `internal/tui/browser.go`'s `View` method, locate the bottom-slot block (around lines 226-239). Replace the help text assignment:

```go
		help := "↑/↓ navigate   →/Enter open   ←/Backspace back   Space select   Shift+↑/↓ range   a all   A clear   d delete   r rescan   q quit"
```

- [ ] **Step 4: Verify the help-line regression test in `TestBrowserToastRendersInBottomSlot`**

That existing test checks for `"d delete   r rescan"` as a substring. The new help text still contains that exact substring, so the existing test should keep passing without modification. Run all tui tests:

Run: `$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path; go test ./internal/tui/... -count=1`

Expected: all pass.

- [ ] **Step 5: Update README key table**

Modify the keys table in `README.md` (around lines 29-40). Replace with:

```markdown
### Keys (browser)

| Key                       | Action                                     |
| ------------------------- | ------------------------------------------ |
| `↑` / `↓` (or `k` / `j`)  | Move selection (cursor)                    |
| `→` / `Enter`             | Enter selected folder                      |
| `←` / `Backspace` / `h`   | Go to parent                               |
| `Space`                   | Toggle bulk-select on cursor row           |
| `Shift+↑` / `Shift+↓`     | Range-select (or `K` / `J`)                |
| `a` / `A`                 | Select all / clear selection               |
| `d`                       | Delete selection (Recycle Bin, confirmable) |
| `r`                       | Rescan current folder                      |
| `g` / `G`                 | Jump to top / bottom                       |
| `q` / `Esc`               | Quit                                       |
```

- [ ] **Step 6: Run the full test + build sweep**

Run:

```powershell
$env:Path = "C:\Users\Boozer\go-sdk\bin;" + $env:Path
go vet ./...
go test ./... -count=1
go test -tags=recycletest ./internal/recycle/... -count=1
go build -o disky.exe ./cmd/disky
if (Test-Path disky.exe) { Remove-Item disky.exe }
```

Expected: vet clean, all tests pass, build succeeds.

- [ ] **Step 7: Manual smoke test**

Run `disky.exe` against a real folder with a handful of items:

1. Open the browser. Verify the help line shows the new key list.
2. Move cursor, press `Space` on 2-3 rows. Verify `*` markers appear, cursor row marker is visible inside the reverse-video block.
3. Press `a`. Verify all rows marked.
4. Press `A`. Verify all markers cleared.
5. Position cursor on row 1. Press `Shift+↓` twice. Verify rows 1-3 selected, cursor at row 3.
6. Press `K`. Verify cursor moves to row 2, both rows 2 and 3 still selected.
7. Press `Space` on row 2 to deselect it. Verify only row 3 marked.
8. Press `d`. Verify confirm dialog shows: `1 item, <size>` (or bullet list if multiple selected).
9. Cancel with `Esc`. Verify selection survives.
10. Press `a`, `d`, confirm with `Enter`. Verify items removed; tree resorts; cursor stays in range.
11. With selection present, navigate into a subfolder with `Enter`. Verify selection cleared.
12. Press `r` (rescan). Verify selection cleared after rescan completes.

- [ ] **Step 8: Commit**

```powershell
git add internal/tui/browser.go internal/tui/browser_test.go README.md
git commit -m "feat(tui): update help line and README for bulk-select

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review Checklist (for the planner)

Verified before handoff:

- **Spec coverage:**
  - Keybindings table (Space, a, A, Shift+↑/↓, J, K, d) → Tasks 2, 3, 5, 6
  - Visual marker (2-cell slot) → Task 2
  - Navigation clears selection → Task 4
  - Rescan clears selection → Task 7
  - `PendingDeletes` slice + selection-or-cursor logic → Task 6
  - Multi-item ConfirmModel + bullets + "and N more" → Task 1
  - `ApplyBatchDelete` partial-failure semantics → Task 7
  - Summary toast format → Task 7
  - Help line text → Task 8
  - README key table → Task 8
  - Single-item confirm renders unchanged → Task 1 (regression test)
  - Plain arrow does not select → Task 5 regression test
  - Edge cases (Shift+arrow at boundary, empty folder, etc.) → Tasks 3, 5

- **Type consistency:**
  - `Selected map[*tree.Node]struct{}` — same name in Tasks 2-7
  - `PendingDeletes []*tree.Node` — same name in Tasks 6-7
  - `ConfirmItem` struct + `NewConfirm([]ConfirmItem)` — same shape in Tasks 1, 6, 7
  - `ApplyBatchDelete(succeeded []*tree.Node) BrowserModel` — same signature in Tasks 7 (definition) and Task 7 (caller)

- **No placeholders:** all steps have concrete code or commands.
