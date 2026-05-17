package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Poonsai/disky/internal/drives"
	"github.com/Poonsai/disky/internal/tree"
)

type PickerModel struct {
	Drives []drives.Drive
	Cursor int
	Chosen *drives.Drive
}

func NewPicker(d []drives.Drive) PickerModel {
	return PickerModel{Drives: d}
}

func (m PickerModel) Init() tea.Cmd { return nil }

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Drives)-1 {
				m.Cursor++
			}
		case "enter":
			if len(m.Drives) > 0 {
				d := m.Drives[m.Cursor]
				m.Chosen = &d
				return m, tea.Quit
			}
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m PickerModel) View() string {
	var b strings.Builder
	b.WriteString(StyleTitle.Render("disky — pick a drive") + "\n\n")
	for i, d := range m.Drives {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		fraction := 0.0
		if d.Total > 0 {
			fraction = float64(d.Total-d.Free) / float64(d.Total)
		}
		row := fmt.Sprintf("%s%-6s %-12s %8s   %s  %d%% used",
			prefix,
			d.Letter,
			d.Label,
			tree.FormatSize(d.Total),
			Bar(fraction),
			d.UsedPercent(),
		)
		if i == m.Cursor {
			row = StyleSelected.Render(row)
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + StyleHelp.Render("↑/↓ select   Enter scan   q quit") + "\n")
	return b.String()
}
