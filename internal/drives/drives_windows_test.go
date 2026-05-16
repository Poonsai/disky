//go:build windows

package drives

import "testing"

func TestListReturnsAtLeastC(t *testing.T) {
	got, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one drive")
	}
	hasC := false
	for _, d := range got {
		if d.Letter == `C:\` {
			hasC = true
			if d.Total <= 0 {
				t.Errorf("C:\\ Total: got %d, want > 0", d.Total)
			}
		}
	}
	if !hasC {
		t.Errorf("C:\\ not in drive list: %+v", got)
	}
}
