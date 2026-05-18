package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmEnterConfirms(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\big.bin", Size: 1024}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	c := next.(ConfirmModel)
	if c.Result != ConfirmYes {
		t.Errorf("Result: got %v, want ConfirmYes", c.Result)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\big.bin", Size: 1024}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	c := next.(ConfirmModel)
	if c.Result != ConfirmNo {
		t.Errorf("Result: got %v, want ConfirmNo", c.Result)
	}
}

func TestConfirmInitialPending(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\big.bin", Size: 1024}})
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
		m := NewConfirm([]ConfirmItem{{Path: "C:\\thing", Size: 1024, ItemCount: c.count}})
		out := m.View()
		if !strings.Contains(out, c.wantSub) {
			t.Errorf("count=%d: view missing %q\n%s", c.count, c.wantSub, out)
		}
		if strings.Contains(out, c.negative) {
			t.Errorf("count=%d: view should not contain %q\n%s", c.count, c.negative, out)
		}
	}
}

func TestConfirmSingleItemViewUnchanged(t *testing.T) {
	m := NewConfirm([]ConfirmItem{{Path: "C:\\foo", Size: 1024, ItemCount: 0}})
	out := m.View()
	if strings.Contains(out, "•") {
		t.Errorf("single item should not show bullets; got:\n%s", out)
	}
	if !strings.Contains(out, "C:\\foo") {
		t.Errorf("path missing from single-item view:\n%s", out)
	}
	if !strings.Contains(out, "1.0 KB") {
		t.Errorf("size missing from single-item view:\n%s", out)
	}
}

func TestConfirmMultiItemSummary(t *testing.T) {
	items := []ConfirmItem{
		{Path: "C:\\a", Size: 500},
		{Path: "C:\\b", Size: 1500},
		{Path: "C:\\c", Size: 2048},
	}
	m := NewConfirm(items)
	out := m.View()
	// Header summary: count + total size (500 + 1500 + 2048 = 4048 bytes = 3.95 KB rounded).
	if !strings.Contains(out, "3 items") {
		t.Errorf("multi-item view missing count; got:\n%s", out)
	}
	// Total size rendered via tree.FormatSize (4048 bytes → "4.0 KB" with the
	// 1024 unit divisor used elsewhere).
	if !strings.Contains(out, "4.0 KB") {
		t.Errorf("multi-item view missing total size; got:\n%s", out)
	}
	for _, want := range []string{"• C:\\a", "• C:\\b", "• C:\\c"} {
		if !strings.Contains(out, want) {
			t.Errorf("multi-item view missing bullet %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "and 0 more") {
		t.Errorf("no overflow expected; got:\n%s", out)
	}
}

func TestConfirmMultiItemTruncatesPaths(t *testing.T) {
	var items []ConfirmItem
	for i := 0; i < 12; i++ {
		items = append(items, ConfirmItem{Path: "C:\\item-" + string(rune('a'+i)), Size: 100})
	}
	m := NewConfirm(items)
	out := m.View()
	if !strings.Contains(out, "12 items") {
		t.Errorf("count missing; got:\n%s", out)
	}
	if !strings.Contains(out, "• C:\\item-a") {
		t.Errorf("first bullet missing; got:\n%s", out)
	}
	if !strings.Contains(out, "• C:\\item-e") {
		t.Errorf("fifth bullet (item-e) missing; got:\n%s", out)
	}
	if strings.Contains(out, "• C:\\item-f") {
		t.Errorf("sixth bullet should be truncated; got:\n%s", out)
	}
	if !strings.Contains(out, "... and 7 more") {
		t.Errorf("overflow line missing; got:\n%s", out)
	}
}

func TestConfirmExactlyAtBulletCap(t *testing.T) {
	// Boundary: maxBullets = 5. Five items must show all bullets and NO
	// "... and N more" overflow line.
	var items []ConfirmItem
	for i := 0; i < 5; i++ {
		items = append(items, ConfirmItem{Path: "C:\\item-" + string(rune('a'+i)), Size: 1})
	}
	out := NewConfirm(items).View()
	for i := 0; i < 5; i++ {
		want := "• C:\\item-" + string(rune('a'+i))
		if !strings.Contains(out, want) {
			t.Errorf("bullet %d missing %q; got:\n%s", i, want, out)
		}
	}
	if strings.Contains(out, "more") {
		t.Errorf("at exact bullet cap, no overflow line expected; got:\n%s", out)
	}
}

func TestConfirmJustOverBulletCap(t *testing.T) {
	// Boundary: 6 items shows the first 5 bullets plus "... and 1 more".
	var items []ConfirmItem
	for i := 0; i < 6; i++ {
		items = append(items, ConfirmItem{Path: "C:\\item-" + string(rune('a'+i)), Size: 1})
	}
	out := NewConfirm(items).View()
	if !strings.Contains(out, "• C:\\item-e") {
		t.Errorf("fifth bullet missing; got:\n%s", out)
	}
	if strings.Contains(out, "• C:\\item-f") {
		t.Errorf("sixth bullet must be hidden by overflow; got:\n%s", out)
	}
	if !strings.Contains(out, "... and 1 more") {
		t.Errorf("expected '... and 1 more' overflow line; got:\n%s", out)
	}
}
