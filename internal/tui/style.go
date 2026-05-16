// Package tui contains the Bubble Tea models for disky's screens.
package tui

import "github.com/charmbracelet/lipgloss"

const (
	MinWidth  = 60
	MinHeight = 20

	BarWidth = 12
)

var (
	StyleTitle    = lipgloss.NewStyle().Bold(true)
	StyleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	StyleSelected = lipgloss.NewStyle().Reverse(true)
	StyleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StyleBar      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	StyleHelp     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// Bar renders a horizontal proportion bar of the given fraction (0..1)
// fitted to BarWidth characters using block glyphs.
func Bar(fraction float64) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	full := int(fraction * BarWidth)
	out := ""
	for i := 0; i < BarWidth; i++ {
		if i < full {
			out += "█"
		} else {
			out += "░"
		}
	}
	return StyleBar.Render(out)
}
