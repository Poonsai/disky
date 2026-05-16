package tree

import "testing"

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.0 TB"},
	}
	for _, c := range cases {
		if got := FormatSize(c.bytes); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}
