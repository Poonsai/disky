package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boozercab/disky/internal/tree"
)

type BrowserModel struct {
	Root    *tree.Node
	Current *tree.Node
	Cursor  int
}

func NewBrowser(root *tree.Node) BrowserModel {
	return BrowserModel{Root: root, Current: root}
}

func (m BrowserModel) Init() tea.Cmd { return nil }

func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
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
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m BrowserModel) View() string {
	var b strings.Builder
	// Header: path + total size, right-aligned.
	header := fmt.Sprintf("%s    %s",
		StyleTitle.Render(m.Current.Path()),
		tree.FormatSize(m.Current.Size),
	)
	b.WriteString(header + "\n\n")

	if len(m.Current.Children) == 0 {
		b.WriteString(StyleDim.Render("  (empty)") + "\n\n")
	} else {
		maxSize := int64(1)
		for _, c := range m.Current.Children {
			if c.Size > maxSize {
				maxSize = c.Size
			}
		}
		for i, c := range m.Current.Children {
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
	b.WriteString("\n" + StyleHelp.Render("↑/↓ select   →/Enter open   ←/Backspace back   d delete   r rescan   q quit") + "\n")
	return b.String()
}
