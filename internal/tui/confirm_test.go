package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmEnterConfirms(t *testing.T) {
	m := NewConfirm("C:\\big.bin", 1024, 0)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	c := next.(ConfirmModel)
	if c.Result != ConfirmYes {
		t.Errorf("Result: got %v, want ConfirmYes", c.Result)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	m := NewConfirm("C:\\big.bin", 1024, 0)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	c := next.(ConfirmModel)
	if c.Result != ConfirmNo {
		t.Errorf("Result: got %v, want ConfirmNo", c.Result)
	}
}

func TestConfirmInitialPending(t *testing.T) {
	m := NewConfirm("C:\\big.bin", 1024, 0)
	if m.Result != ConfirmPending {
		t.Errorf("initial Result: got %v, want ConfirmPending", m.Result)
	}
}
