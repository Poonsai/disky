package tui

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boozercab/disky/internal/scan"
	"github.com/boozercab/disky/internal/tree"
)

// ProgressModel runs a scan in the background and renders its live counters.
// On completion (or cancel) the model's Result/Err are populated and the
// Bubble Tea program quits.
type ProgressModel struct {
	root     string
	progress *scan.Progress
	start    time.Time
	tick     int // for spinner glyph

	Result *tree.Node
	Err    error
}

type scanDoneMsg struct {
	root *tree.Node
	err  error
}

type tickMsg struct{}

var spinnerGlyphs = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// NewProgress builds the model. The scan starts on Init.
func NewProgress(root string) ProgressModel {
	return ProgressModel{
		root:     root,
		progress: &scan.Progress{},
		start:    time.Now(),
	}
}

func (m ProgressModel) Init() tea.Cmd {
	// NOTE: Bubble Tea Init returns tea.Cmd only (no model update), so any cancel
	// func stored on the model would be lost. The user-facing quit keys return
	// tea.Quit which tears down the program; the scan goroutine finishes
	// naturally afterward (wasted I/O is bounded by remaining scan time).
	return tea.Batch(
		tickCmd(),
		func() tea.Msg {
			root, err := scan.Scan(context.Background(), m.root, m.progress)
			return scanDoneMsg{root: root, err: err}
		},
	)
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c", "enter":
			return m, tea.Quit
		}
	case tickMsg:
		m.tick++
		return m, tickCmd()
	case scanDoneMsg:
		m.Result = msg.root
		m.Err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m ProgressModel) View() string {
	spinner := string(spinnerGlyphs[m.tick%len(spinnerGlyphs)])
	elapsed := time.Since(m.start).Round(time.Second)
	currentPath := m.progress.CurrentPath()
	if len(currentPath) > 70 {
		currentPath = "…" + currentPath[len(currentPath)-69:]
	}
	return fmt.Sprintf(
		"%s\n\n %s %s items  •  %s  •  %s\n   Currently: %s\n\n%s\n",
		StyleTitle.Render(fmt.Sprintf("Scanning %s ...", m.root)),
		spinner,
		formatInt(atomic.LoadInt64(&m.progress.Items)),
		tree.FormatSize(atomic.LoadInt64(&m.progress.Bytes)),
		elapsed,
		currentPath,
		StyleHelp.Render("q cancel"),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func formatInt(n int64) string {
	// Add thousand separators: 1234567 -> 1,234,567
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	return out
}
