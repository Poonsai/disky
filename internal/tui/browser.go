package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Poonsai/disky/internal/tree"
)

type BrowserModel struct {
	Root           *tree.Node
	Current        *tree.Node
	Cursor         int
	Offset         int                     // first visible index in Current.Children
	Width, Height  int                     // terminal size (set by tea.WindowSizeMsg)
	Selected       map[*tree.Node]struct{} // multi-select set; nil/empty = no selection
	PendingDeletes []*tree.Node            // set when the user pressed 'd'; cleared on Apply/Cancel
	PendingRescan  bool
	Toast          string // transient error message; rendered in help slot, cleared on any key
}

func NewBrowser(root *tree.Node) BrowserModel {
	return BrowserModel{Root: root, Current: root}
}

func (m BrowserModel) Init() tea.Cmd { return nil }

// viewportRows reports how many child rows can be shown in the current
// terminal. View renders 4 lines of chrome (header, blank, blank, help)
// around the child list. Defaults to 20 when Height hasn't been
// reported yet (WindowSizeMsg always arrives shortly after Init).
func (m BrowserModel) viewportRows() int {
	if m.Height == 0 {
		return 20
	}
	rows := m.Height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

// adjustOffset clamps Offset so Cursor is visible inside the viewport.
// Call after any change to Cursor, Current.Children, or Height.
func (m BrowserModel) adjustOffset() BrowserModel {
	n := len(m.Current.Children)
	if n == 0 {
		m.Offset = 0
		return m
	}
	vp := m.viewportRows()
	if vp >= n {
		m.Offset = 0
		return m
	}
	if m.Cursor < m.Offset {
		m.Offset = m.Cursor
	} else if m.Cursor >= m.Offset+vp {
		m.Offset = m.Cursor - vp + 1
	}
	if m.Offset+vp > n {
		m.Offset = n - vp
	}
	if m.Offset < 0 {
		m.Offset = 0
	}
	return m
}

func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m.adjustOffset(), nil
	case tea.KeyMsg:
		// Any key dismisses a transient error toast.
		m.Toast = ""
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Current.Children)-1 {
				m.Cursor++
			}
		case "g":
			m.Cursor = 0
		case "G":
			if n := len(m.Current.Children); n > 0 {
				m.Cursor = n - 1
			}
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
		case "r":
			m.PendingRescan = true
			return m, tea.Quit
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
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m.adjustOffset(), nil
}

func (m BrowserModel) View() string {
	var b strings.Builder

	width := m.Width
	if width <= 0 {
		width = 80
	}

	// Header: bolded path on the left, total size right-aligned, with a
	// horizontal rule below so it's obvious where the user is regardless
	// of what's in the terminal scrollback above. Length math uses runes
	// (not bytes) so non-ASCII paths don't get cut mid-codepoint.
	pathStr := m.Current.Path()
	sizeStr := tree.FormatSize(m.Current.Size)
	maxPath := width - runeLen(sizeStr) - 2
	if maxPath < 10 {
		maxPath = 10
	}
	if pathRunes := []rune(pathStr); len(pathRunes) > maxPath {
		pathStr = "…" + string(pathRunes[len(pathRunes)-maxPath+1:])
	}
	pad := width - runeLen(pathStr) - runeLen(sizeStr)
	if pad < 1 {
		pad = 1
	}
	header := StyleTitle.Render(pathStr) + strings.Repeat(" ", pad) + sizeStr
	b.WriteString(header + "\n")
	b.WriteString(StyleDim.Render(strings.Repeat("─", width)) + "\n")

	n := len(m.Current.Children)
	if n == 0 {
		b.WriteString(StyleDim.Render("  (empty)") + "\n")
	} else {
		// Bar fraction is relative to the largest sibling overall so the
		// visible rows stay comparable as the viewport scrolls.
		maxSize := int64(1)
		for _, c := range m.Current.Children {
			if c.Size > maxSize {
				maxSize = c.Size
			}
		}
		vp := m.viewportRows()
		end := m.Offset + vp
		if end > n {
			end = n
		}
		// Row layout: "  %10s  %12s  %s%s%s" = 2 + 10 + 2 + 12 + 2 + 2 = 30 fixed
		// cells plus optional 4-cell "[!] " marker, plus the name. The extra
		// 2 cells are the selection marker slot ("* " or "  "). Truncate the
		// name so each row stays on ONE terminal line — wrapping would
		// inflate row count and overflow the viewport, hiding the header.
		const fixedCells = 18 + BarWidth
		for i := m.Offset; i < end; i++ {
			c := m.Current.Children[i]
			markerCells := 0
			if c.Err != nil {
				markerCells = 4
			}
			nameBudget := width - fixedCells - markerCells
			name := truncateRunes(c.Name, nameBudget)

			// Cursor row: render plain text and wrap once in StyleSelected.
			// Wrapping over already-styled inner segments breaks the reverse
			// video because each inner Render emits its own \x1b[0m reset.
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

			// Non-cursor row: inner segments are styled (blue bar, red marker).
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
		}
	}
	// Bottom slot: error toast takes priority over the normal help line.
	// Either way it's exactly one row to keep the viewport math correct.
	var bottom string
	if m.Toast != "" {
		bottom = StyleError.Render(truncateRunes(m.Toast, width))
	} else {
		help := "↑/↓ nav  →/Enter open  ←/Back back  Space sel  a/A all/clear  d delete  r rescan  q quit"
		if n > m.viewportRows() {
			end := m.Offset + m.viewportRows()
			if end > n {
				end = n
			}
			help = fmt.Sprintf("%s   [%d-%d / %d]", help, m.Offset+1, end, n)
		}
		bottom = StyleHelp.Render(truncateRunes(help, width))
	}
	b.WriteString("\n" + bottom)
	return b.String()
}

// runeLen returns the rune count of s. Cheaper-to-write alias.
func runeLen(s string) int { return len([]rune(s)) }

// truncateRunes returns s truncated to at most budget runes, with an
// ellipsis appended when truncation happens. budget < 1 returns "".
func truncateRunes(s string, budget int) string {
	if budget < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= budget {
		return s
	}
	if budget <= 1 {
		return string(r[:budget])
	}
	return string(r[:budget-1]) + "…"
}

// ApplyDelete removes target from the tree and clears PendingDeletes.
// Caller should already have moved the actual filesystem path via recycle.Send.
func (m BrowserModel) ApplyDelete(target *tree.Node) BrowserModel {
	tree.RemoveAndRecompute(target)
	tree.Sort(m.Current)
	// m.Current's Size shrank, so its position among its siblings — and
	// every ancestor's position — may be stale. Walk up sorting.
	tree.SortAncestors(m.Current)
	if m.Cursor >= len(m.Current.Children) && m.Cursor > 0 {
		m.Cursor = len(m.Current.Children) - 1
	}
	m.PendingDeletes = nil
	return m.adjustOffset()
}

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

// CancelDelete clears the pending request without touching the tree.
// Selection (if any) is preserved so the user can adjust and retry.
func (m BrowserModel) CancelDelete() BrowserModel {
	m.PendingDeletes = nil
	return m
}

// CancelRescan clears the pending rescan flag without touching the tree.
// Mirrors CancelDelete; used when the user cancels the progress screen.
func (m BrowserModel) CancelRescan() BrowserModel {
	m.PendingRescan = false
	return m
}

// ApplyRescan replaces the current node's subtree with newCurrent's children
// and updates its size. It walks the parent chain so ancestor sizes stay
// consistent. The parent pointers of newCurrent.Children are rewritten to
// point at m.Current (the old node) so the rest of the tree stays valid.
func (m BrowserModel) ApplyRescan(newCurrent *tree.Node) BrowserModel {
	oldSize := m.Current.Size
	m.Current.Children = newCurrent.Children
	for _, c := range m.Current.Children {
		c.Parent = m.Current
	}
	m.Current.Size = newCurrent.Size
	m.Current.Err = newCurrent.Err
	// Propagate delta up.
	delta := m.Current.Size - oldSize
	for cur := m.Current.Parent; cur != nil; cur = cur.Parent {
		cur.Size += delta
	}
	tree.Sort(m.Current)
	// m.Current's Size may have changed, so ancestor child lists are stale.
	tree.SortAncestors(m.Current)
	if m.Cursor >= len(m.Current.Children) {
		m.Cursor = 0
	}
	// Selection holds pointers into the OLD children, which we just
	// replaced. Stale pointers would point at orphaned tree nodes.
	m.Selected = nil
	m.PendingRescan = false
	return m.adjustOffset()
}
