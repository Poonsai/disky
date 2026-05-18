# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`disky` is a Windows-only TUI disk-space analyzer (think `ncdu` for Windows). Single ~5 MB binary, three screens (drive picker → scan progress → folder browser), `d` recycles the cursor row or the bulk selection.

Module: `github.com/Poonsai/disky`. Go 1.22+; toolchain pinned to 1.26. Built on Bubble Tea (`charmbracelet/bubbletea`) + Lipgloss.

## Commands

```powershell
# build, test, run
go build ./...
go test ./... -count=1
go run ./cmd/disky

# release-style binary (~3.4 MB stripped vs ~5 MB) — also injects the
# version string surfaced by `disky --version`
go build -ldflags "-s -w -X main.version=v1.1.0" -o disky.exe ./cmd/disky
```

Without `-X main.version=...` the binary prints `dev` for `--version`,
unless it was installed via `go install …@vX.Y.Z` — in that case
`runtime/debug.ReadBuildInfo` provides the module version automatically.
Tagging a new release is what makes `@latest` users see the new version.

### Recycle-bin integration tests

`internal/recycle/recycle_windows_test.go` actually moves files to (and out of) the Recycle Bin, so it sits behind a build tag and is OFF by default:

```powershell
go test -tags=recycletest ./internal/recycle/... -count=1
```

Run this when changing anything in `internal/recycle/`.

### Single test

```powershell
go test -run TestBrowserShiftDownExtendsSelection ./internal/tui/...
```

### Race detector

Requires CGO + a C compiler. Not available in the default toolchain on this machine — skip unless you have one.

## Architecture

Four `internal/` packages plus a thin entry point. The split is intentional and not flexible:

```
cmd/disky/main.go        entry: drive picker -> scan -> browser loop
internal/drives/         Windows drive enumeration (windows-only file)
internal/scan/           parallel filesystem walker; pure data, no TUI
internal/tree/           tree.Node operations: sort, recompute, format. No I/O.
internal/tui/            Bubble Tea models (picker, progress, browser, confirm) + style
internal/recycle/        Windows shell wrapper around IFileOperation (windows-only file)
```

Boundaries:

- `scan/` and `tree/` are pure: zero dependency on `tui/` or `bubbletea`. Fully unit-testable on any OS.
- `tui/` consumes `scan/` and `tree/`. It does NOT touch the filesystem directly.
- `recycle/` is the only Windows shell-API call. It's the seam.
- `drives/` and `recycle/` use `//go:build windows`. There are NO non-Windows shims — disky doesn't compile on other OSes.

### `main.go` event loop

This is the non-obvious bit. Each TUI screen runs as a separate `tea.NewProgram(...).Run()` and exits via `tea.Quit`. State is handed back to `main` through public fields on the model:

- `ProgressModel.Result/Err/Canceled` — set on exit; `main.runScan` reads them.
- `BrowserModel.PendingDeletes []*tree.Node` — when `d` is pressed, the model populates this and quits. `main.browse` switches on it.
- `BrowserModel.PendingRescan bool` — same pattern for `r`.

`main.browse` loops, running a fresh `BrowserModel` program each iteration. After a delete batch finishes (`handleDelete` calls `recycle.Send` per item, partitions into succeeded/failed, calls `bm.ApplyBatchDelete(succeeded)`, sets a summary toast), it re-runs the browser. Same for rescan via `handleRescan`. Empty `PendingDeletes` + `!PendingRescan` after `tea.Quit` means the user actually wants to quit.

When adding a new "the browser needs main.go to do something" action, follow this pattern: add a `Pending*` field, populate + `tea.Quit` in the handler, branch on it in `main.browse`.

### `scan` worker pool

`Scan` uses a fixed pool of `runtime.NumCPU()` workers reading from an unbounded mutex/cond queue (`queue` in `scan.go`). LIFO order for DFS cache locality. The earlier design spawned one goroutine per subdirectory, which OOMed on Windows trees with millions of dirs (node_modules, WinSxS); the rewrite is the worker-pool form.

Cancellation: `Scan` watches `ctx.Done` via a side goroutine that flips the queue to `done`. Workers also check `ctx.Err()` inside the per-entry loop so cancel takes effect inside huge directories, not just at the next `ReadDir` boundary. When the user cancels via `ProgressModel`, the returned tree is partial and `Scan` returns `ctx.Err()` — callers MUST check the err and discard the tree.

Hard-link dedup and reparse-point (junction) detection are in `scan_attrs_windows.go`. The `scan_attrs_other.go` shim is a no-op so `scan/` tests still run on non-Windows CI (the package, but not the app, is cross-platform). `fileUniqueID` short-circuits when `NumberOfLinks <= 1` to avoid the open-syscall cost on the common case.

### `recycle` uses IFileOperation (COM), not SHFileOperationW

The older `SHFileOperationW` API silently downgrades `FOF_ALLOWUNDO` to a permanent delete when the file is too big for the bin or the volume doesn't support it (network shares), AND fails on long paths. `recycle_windows.go` invokes the modern `IFileOperation` COM interface via raw `syscall.SyscallN` against the vtable (no third-party `go-ole` dep). Flags include `FOFX_RECYCLEONDELETE | FOFX_EARLYFAILURE` so non-recyclable files return an HRESULT instead of silent permanent-delete.

The `comObj{p unsafe.Pointer}` wrapper exists specifically to keep `go vet` quiet about the `uintptr → unsafe.Pointer` round-trip pattern that COM vtable calls require. Don't "simplify" it.

### `tree` mutation rules

- `tree.ComputeSizes(root)` runs once after a scan completes; sums children bottom-up.
- `tree.RemoveAndRecompute(n)` is called per deleted node — it updates ancestor sizes incrementally, so a batch delete loop with one `RemoveAndRecompute` per item + a single `tree.Sort(m.Current)` + `tree.SortAncestors(m.Current)` at the end is correct (and what `BrowserModel.ApplyBatchDelete` does).
- `tree.SortAncestors(start)` walks `Parent` chain re-sorting because a node's size change can move it in its parent's `Children` slice.

`BrowserModel.ApplyDelete` (singular) is now production-dead — only the tests still call it. It's intentionally left in place; cleanup will happen separately. Use `ApplyBatchDelete([]*tree.Node{n})` for new code paths.

### `tui` selection model

`BrowserModel.Selected map[*tree.Node]struct{}` is the multi-select set, scope is per-folder. Navigation (`enter`/`right`/`backspace`/`left`/`h`) and `ApplyRescan` clear it because the underlying node pointers become stale. `d` chooses selection if non-empty, else cursor row. Range select via `Shift+↑/↓` or `J`/`K` extends one row at a time (no anchor state).

The row layout has a 2-cell selection slot between the size-bar and the optional `[!]` error marker; the cursor row wraps the whole row in `StyleSelected` (reverse video) so inner styled segments must be PLAIN strings to avoid SGR-reset breakage (cf. `TestBrowserCursorRowHasNoInnerResetBeforeName`).

## Testing notes

- `internal/tui/browser_test.go` forces `lipgloss.SetColorProfile(termenv.ANSI256)` in `TestMain` so styling tests see real ANSI SGR codes. Without this, `Render()` returns plain text.
- `internal/scan/scan_windows_test.go` covers junction (`mklink /J`) and hard-link (`os.Link`) dedup — tagged `//go:build windows`.
- Process drives, walked dirs, and confirm UIs are all behind direct testable models; no Bubble Tea program needs to be `.Run()` in tests.

## Branch & PR convention

`master` is the integration branch (no protection; force-push avoided). Commit subject style: `feat(tui): …`, `fix: …`, `refactor: …`, `docs(spec): …`, `style(tui): …`. Substantive work goes through a `docs/superpowers/specs/YYYY-MM-DD-*-design.md` + `docs/superpowers/plans/YYYY-MM-DD-*.md` pair before code lands — see existing entries for the format.

CRLF/LF warnings from `git add` are expected (`core.autocrlf=true`); the index canonicalizes to LF and on-disk Windows checkouts get CRLF. Don't fight gofmt over them.
