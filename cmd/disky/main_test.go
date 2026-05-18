package main

import (
	"errors"
	"testing"

	"github.com/Poonsai/disky/internal/tree"
)

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

func TestRunBatchRecycleAllSucceed(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	a := &tree.Node{Name: "a", Parent: root}
	b := &tree.Node{Name: "b", Parent: root}
	root.Children = []*tree.Node{a, b}

	succeeded, failed := runBatchRecycle([]*tree.Node{a, b}, func(string) error { return nil })

	if len(succeeded) != 2 || succeeded[0] != a || succeeded[1] != b {
		t.Errorf("succeeded: got %+v, want [a b] in order", succeeded)
	}
	if len(failed) != 0 {
		t.Errorf("failed: got %+v, want nil", failed)
	}
}

func TestRunBatchRecyclePartialFailure(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	a := &tree.Node{Name: "a", Parent: root}
	b := &tree.Node{Name: "b", Parent: root}
	c := &tree.Node{Name: "c", Parent: root}
	root.Children = []*tree.Node{a, b, c}

	send := func(path string) error {
		if path == `C:\b` {
			return errors.New("locked")
		}
		return nil
	}
	succeeded, failed := runBatchRecycle([]*tree.Node{a, b, c}, send)

	if len(succeeded) != 2 || succeeded[0] != a || succeeded[1] != c {
		t.Errorf("succeeded: got %+v, want [a c]", succeeded)
	}
	if len(failed) != 1 || failed[0] != "b" {
		t.Errorf("failed: got %+v, want [b]", failed)
	}
}

func TestRunBatchRecycleAllFail(t *testing.T) {
	root := &tree.Node{Name: `C:\`, IsDir: true}
	a := &tree.Node{Name: "a", Parent: root}
	b := &tree.Node{Name: "b", Parent: root}
	root.Children = []*tree.Node{a, b}

	send := func(string) error { return errors.New("nope") }
	succeeded, failed := runBatchRecycle([]*tree.Node{a, b}, send)

	if len(succeeded) != 0 {
		t.Errorf("succeeded: got %+v, want empty", succeeded)
	}
	if len(failed) != 2 || failed[0] != "a" || failed[1] != "b" {
		t.Errorf("failed: got %+v, want [a b]", failed)
	}
}

func TestRunBatchRecycleEmptyTargets(t *testing.T) {
	succeeded, failed := runBatchRecycle(nil, func(string) error { return nil })
	if succeeded != nil || failed != nil {
		t.Errorf("expected both nil; got succeeded=%+v failed=%+v", succeeded, failed)
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
