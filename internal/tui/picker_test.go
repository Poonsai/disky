package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boozercab/disky/internal/drives"
)

func samplePicker() PickerModel {
	return NewPicker([]drives.Drive{
		{Letter: `C:\`, Label: "Local Disk", Total: 500 << 30, Free: 100 << 30},
		{Letter: `D:\`, Label: "Data", Total: 2 << 40, Free: 1 << 40},
	})
}

func TestPickerInitialCursor(t *testing.T) {
	m := samplePicker()
	if m.Cursor != 0 {
		t.Errorf("initial cursor: got %d, want 0", m.Cursor)
	}
}

func TestPickerArrowsMoveCursor(t *testing.T) {
	m := samplePicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := next.(PickerModel).Cursor; got != 1 {
		t.Errorf("down: cursor got %d, want 1", got)
	}
	wrapBack, _ := next.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := wrapBack.(PickerModel).Cursor; got != 0 {
		t.Errorf("up: cursor got %d, want 0", got)
	}
}

func TestPickerCursorClampsAtEnds(t *testing.T) {
	m := samplePicker()
	up, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := up.(PickerModel).Cursor; got != 0 {
		t.Errorf("up at top: cursor got %d, want 0", got)
	}
	// move past the end
	m1, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2, _ := m1.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := m2.(PickerModel).Cursor; got != 1 {
		t.Errorf("down at bottom: cursor got %d, want 1", got)
	}
}

func TestPickerEnterSelectsAndQuits(t *testing.T) {
	m := samplePicker()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := next.(PickerModel)
	if pm.Chosen == nil {
		t.Fatal("Chosen should be set after Enter")
	}
	if pm.Chosen.Letter != `C:\` {
		t.Errorf("Chosen.Letter: got %q, want %q", pm.Chosen.Letter, `C:\`)
	}
	if cmd == nil {
		t.Error("Enter should return tea.Quit")
	}
}
