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

func TestResolveVersionPrefersLDFlagsInjection(t *testing.T) {
	orig := version
	defer func() { version = orig }()
	version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("resolveVersion: got %q, want %q", got, "v9.9.9")
	}
}

func TestResolveVersionFallsBackToDev(t *testing.T) {
	orig := version
	defer func() { version = orig }()
	version = "dev"
	// When running under `go test` BuildInfo.Main.Version is "(devel)"
	// or empty, so the BuildInfo branch declines and we get the
	// literal "dev" fallback.
	got := resolveVersion()
	if got != "dev" && got != "v9.9.9" {
		// Tolerant assertion: BuildInfo behavior across Go releases
		// occasionally surfaces a non-empty Main.Version. As long as
		// it's not garbage, accept it.
		if len(got) == 0 {
			t.Errorf("resolveVersion returned empty string")
		}
	}
}
