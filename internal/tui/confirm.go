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

type ConfirmModel struct {
	Path      string
	Size      int64
	ItemCount int // 0 for files; populated for directories
	Result    ConfirmResult
}

func NewConfirm(path string, size int64, itemCount int) ConfirmModel {
	return ConfirmModel{Path: path, Size: size, ItemCount: itemCount}
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
	b.WriteString("  " + m.Path + "\n")
	if m.ItemCount > 0 {
		b.WriteString(fmt.Sprintf("  %d items, %s\n", m.ItemCount, tree.FormatSize(m.Size)))
	} else {
		b.WriteString("  " + tree.FormatSize(m.Size) + "\n")
	}
	b.WriteString("\n" + StyleHelp.Render("[Enter] confirm    [Esc] cancel") + "\n")
	return b.String()
}
