package tui

import (
	"strings"
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

func TestConfirmPluralizesItemCount(t *testing.T) {
	cases := []struct {
		count    int
		wantSub  string
		negative string
	}{
		{1, "1 item, ", "1 items"},
		{3, "3 items, ", "3 item,"},
		{0, "1.0 KB", "items"}, // file path: no item count rendered at all
	}
	for _, c := range cases {
		m := NewConfirm("C:\\thing", 1024, c.count)
		out := m.View()
		if !strings.Contains(out, c.wantSub) {
			t.Errorf("count=%d: view missing %q\n%s", c.count, c.wantSub, out)
		}
		if strings.Contains(out, c.negative) {
			t.Errorf("count=%d: view should not contain %q\n%s", c.count, c.negative, out)
		}
	}
}
