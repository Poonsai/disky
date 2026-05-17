package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boozercab/disky/internal/tree"
)

type BrowserModel struct {
	Root          *tree.Node
	Current       *tree.Node
	Cursor        int
	Offset        int        // first visible index in Current.Children
	Width, Height int        // terminal size (set by tea.WindowSizeMsg)
	PendingDelete *tree.Node // set when the user pressed 'd'; cleared on Apply/Cancel
	PendingRescan bool
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
			}
		case "r":
			m.PendingRescan = true
			return m, tea.Quit
		case "d":
			if m.Cursor < len(m.Current.Children) {
				m.PendingDelete = m.Current.Children[m.Cursor]
				return m, tea.Quit
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
	// of what's in the terminal scrollback above.
	pathStr := m.Current.Path()
	sizeStr := tree.FormatSize(m.Current.Size)
	maxPath := width - len(sizeStr) - 2
	if maxPath < 10 {
		maxPath = 10
	}
	if len(pathStr) > maxPath {
		pathStr = "…" + pathStr[len(pathStr)-maxPath+1:]
	}
	pad := width - len(pathStr) - len(sizeStr)
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
		for i := m.Offset; i < end; i++ {
			c := m.Current.Children[i]
			marker := ""
			if c.Err != nil {
				marker = StyleError.Render("[!] ")
			}
			row := fmt.Sprintf("  %10s  %s  %s%s",
				tree.FormatSize(c.Size),
				Bar(float64(c.Size)/float64(maxSize)),
				marker,
				c.Name,
			)
			if i == m.Cursor {
				row = StyleSelected.Render(row)
			}
			b.WriteString(row + "\n")
		}
	}
	help := "↑/↓ select   →/Enter open   ←/Backspace back   d delete   r rescan   q quit"
	if n > m.viewportRows() {
		end := m.Offset + m.viewportRows()
		if end > n {
			end = n
		}
		help = fmt.Sprintf("%s   [%d-%d / %d]", help, m.Offset+1, end, n)
	}
	b.WriteString("\n" + StyleHelp.Render(help))
	return b.String()
}

// ApplyDelete removes target from the tree and clears PendingDelete.
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
	m.PendingDelete = nil
	return m.adjustOffset()
}

// CancelDelete clears the pending request without touching the tree.
func (m BrowserModel) CancelDelete() BrowserModel {
	m.PendingDelete = nil
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
	m.PendingRescan = false
	return m.adjustOffset()
}
