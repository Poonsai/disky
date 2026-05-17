package main

import "testing"

func TestFormatBatchFailureExactlyMaxNames(t *testing.T) {
	got := formatBatchFailure(7, 10, []string{"a", "b", "c"})
	want := "deleted 7 of 10; failed: a, b, c"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatBatchFailureSingleFailure(t *testing.T) {
	got := formatBatchFailure(2, 3, []string{"only"})
	want := "deleted 2 of 3; failed: only"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatBatchFailureManyTruncated(t *testing.T) {
	got := formatBatchFailure(0, 5, []string{"a", "b", "c", "d", "e"})
	want := "deleted 0 of 5; failed: a, b, c and 2 more"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatBatchFailureAllFailed(t *testing.T) {
	got := formatBatchFailure(0, 3, []string{"x", "y", "z"})
	want := "deleted 0 of 3; failed: x, y, z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
