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
