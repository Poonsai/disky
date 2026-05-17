# disky — Design

**Date:** 2026-05-16
**Status:** Approved (brainstorming complete; ready for implementation planning)

## Summary

A lightweight, terminal-based disk space analyzer for Windows. Run `disky`, pick a drive, watch a fast parallel scan, then navigate folders sorted by size — drilling in and out with arrow keys and reclaiming space by sending items to the Recycle Bin. Conceptually: ncdu for Windows, scoped at the level of a single small binary.

## Goals

- **Lightweight:** single self-contained ~5 MB binary, no runtime install, fast scan on a 500 GB drive.
- **Simple:** no settings, no config files, no setup. Run and use.
- **Useful:** lets the user find and delete what's taking space without leaving the terminal.

## Non-goals

- Treemap visualization. The CLI/TUI is the product.
- Cross-platform support in v1. Windows-only ships first.
- Bulk/multi-select operations.
- Cloud-storage placeholder awareness, dedup-aware sizing, or hard-link deduplication.
- File-type breakdowns or extension-grouped views.

## User flow

1. User runs `disky` (no arguments).
2. **Drive picker** appears, listing mounted drives with capacity and % used. Arrow keys + Enter to pick.
3. **Progress screen** shows a live counter while the scan runs: items scanned, bytes scanned, current path, elapsed time, spinner. Cancellable with `q`/`Esc`.
4. **Browser screen** opens at the scan root: current folder path at the top, sorted children below (largest first) with a per-row size bar.
5. User navigates with arrow keys, deletes selected items to the Recycle Bin with `d` (confirmation modal), quits with `q`.

## Architecture

Four internal packages plus a thin entry point.

```
cmd/disky/main.go        # entry point, ~30 lines
internal/
  scan/                  # filesystem walker, builds Node tree
  tree/                  # operations on Node tree (sort, recompute, format)
  tui/                   # Bubble Tea models for the three screens
  recycle/               # Windows Recycle Bin wrapper
```

Boundaries:

- `scan/` and `tree/` are pure data layers with no TUI dependency — fully unit-testable.
- `tui/` calls into `scan/` and `tree/`; never reaches into the filesystem directly.
- `recycle/` is the only OS-specific package and is the single seam for the Windows shell call.

## Data model

```go
type Node struct {
    Name     string  // basename; full path reconstructed by walking parents
    Size     int64   // file: file size. dir: sum of descendants (computed bottom-up)
    Children []*Node // nil for files; sorted descending by Size after scan
    Parent   *Node   // upward navigation and size recomputation after delete
    IsDir    bool
    Err      error   // permission denied etc.; surfaces as marker in UI, doesn't abort scan
}
```

- Basename-only storage keeps memory modest on million-file scans.
- Parent pointers enable O(depth) parent-size recomputation after delete — no rescan needed.
- `Err` is per-node, not per-scan — partial failure is normal on Windows.

## Scan engine

**Algorithm:** bounded worker pool over a directory queue.

- `runtime.NumCPU()` worker goroutines (default; configurable later if needed).
- Each worker pulls a directory off a channel, reads its entries with one syscall, stats each entry, and pushes any subdirectories back onto the queue.
- Files are handled inline when their parent is read — they are not their own work units.
- An atomic counter tracks bytes and items scanned for the progress UI.
- A `context.Context` is threaded through all workers. Cancellation drains and returns the partial tree.

**Termination:** scan completes when the queue is empty AND no worker is currently processing a directory. Implemented with a wait group plus a coordinator.

**Post-scan pass:** a single sequential walk sorts each directory's `Children` descending by `Size`. Bottom-up size computation happens during the walk back up.

**Sizing semantics:** apparent (logical) size from `os.Lstat`. Cluster/on-disk size is a future flag.

**Symlinks and junctions:** detected via `Lstat`, **not followed**. They contribute 0 bytes and render with a `[link]` marker. Windows is full of junctions (e.g., `C:\Users\All Users` → `C:\ProgramData`); following them would double-count and risk loops.

**Permission errors:** logged on the node, scan continues. Folder renders with `[!]` marker.

**Disappearing files:** race between enumeration and stat is treated as 0 bytes; no error surfaced.

## TUI screens

All three screens are Bubble Tea models. Lipgloss handles styling.

### Drive picker

```
 disky — pick a drive

 > C:\   Local Disk    512 GB    ████████░░  78% used
   D:\   Data          2.0 TB    ███░░░░░░░  31% used
   E:\   USB Drive     32 GB     █░░░░░░░░░  12% used

 ↑/↓ select   Enter scan   q quit
```

### Progress

```
 Scanning C:\ ...

 ⠋ 1,247,891 items  •  342.1 GB  •  00:01:47
   Currently: C:\Users\Boozer\AppData\Local\...\node_modules\@types

 q cancel
```

Progress updates throttled to ~10/sec to keep render cost negligible while the scan thrashes disk.

### Browser

```
 C:\Users\Boozer                                    412.7 GB

   123.4 GB  ████████████  AppData
    87.2 GB  ████████░     Documents
    45.1 GB  ████░░░░░     Downloads
    12.0 GB  █░░░░░░░░     [!] Application Data         (junction, skipped)
       8 KB  ░░░░░░░░░     .gitconfig
       ...

 ↑/↓ select   →/Enter open   ←/Backspace back   d delete   r rescan   q quit
```

Per-row bar is proportional to `row.Size / max(siblings.Size)`. Sizes right-aligned in a fixed column.

## Key bindings (browser)

| Key                       | Action                                     |
| ------------------------- | ------------------------------------------ |
| `↑` / `↓` (or `k` / `j`)  | Move selection                             |
| `→` / `Enter`             | Enter selected folder                      |
| `←` / `Backspace` / `h`   | Go to parent (disabled at scan root)       |
| `d`                       | Delete selected (confirmation modal)       |
| `r`                       | Rescan current folder                      |
| `g` / `G`                 | Jump to top / bottom of list               |
| `q` / `Esc`               | Quit                                       |

## Delete flow

**Confirmation modal** on `d`:

```
 Move to Recycle Bin?

   C:\Users\Boozer\Downloads\big-installer.exe
   2.3 GB

 [Enter] confirm    [Esc] cancel
```

For folders, also display item count: `247 items, 12.0 GB`.

**Mechanism:** Windows Shell API `SHFileOperationW` with `FOF_ALLOWUNDO`. A small existing Go library is the first choice; if none is solid, a thin syscall wrapper is acceptable (~30 lines).

**On success:** remove node from tree, walk parent pointers subtracting the deleted size up to the root, re-sort the affected folder's children, select the next sibling. No rescan needed.

**On error** (locked file, in-use, permission denied): show a transient toast at the bottom of the screen — `Could not delete: file is open in another program` — and keep the node in the tree.

**Explicit non-features:** no bulk delete, no in-app undo (the Recycle Bin is the undo).

## Edge cases

- **Permission-denied folders:** marked `[!]`, contribute 0 bytes, no popup during scan.
- **Symlinks/junctions:** marked `[link]`, not followed.
- **Other reparse points** (OneDrive placeholders, dedup): size is whatever Windows reports via `Lstat`. Documented as a known limitation.
- **Empty directories:** rendered with size `0 B`.
- **Disappearing files during scan:** treated as 0 bytes, no error.
- **Very deep trees:** iterative channel-queue walk; no recursion stack risk.
- **Terminal too small** (under 60×20): show a "terminal too small" message that updates live on resize.

## Performance targets

These are design checks, not hard SLAs.

- 500 GB SSD with ~1M files: well under one minute on modern hardware.
- Memory: comfortably under 500 MB for a 1M-node tree (rough budget: ~200 B per node).
- Progress render: throttled to ~10/sec to keep UI cost negligible during scan.

## Testing

- **`tree/`:** unit tests for sort, parent-size recompute, size formatting. Pure functions, fast and deterministic.
- **`scan/`:** unit tests using a temp-directory fixture. Cover nested dirs, empty dirs, simulated permission errors, deep paths. Symlink test gated with `t.Skip` when the runner cannot create one.
- **`recycle/`:** integration test gated behind a build tag so CI does not accidentally trash files. Run locally before release.
- **`tui/`:** light state-transition tests for screen models; trust Bubble Tea for rendering. Manual smoke test before release.

## Dependencies

Kept deliberately small.

- `github.com/charmbracelet/bubbletea` — TUI runtime.
- `github.com/charmbracelet/lipgloss` — styling.
- One small dep for Recycle Bin (evaluate during planning) — or a hand-rolled `syscall` wrapper.

## Project layout

```
disky/
├── cmd/disky/main.go
├── internal/
│   ├── scan/
│   │   ├── scan.go
│   │   ├── node.go
│   │   └── scan_test.go
│   ├── tree/
│   │   ├── tree.go
│   │   └── tree_test.go
│   ├── tui/
│   │   ├── picker.go        # drive picker
│   │   ├── progress.go      # scan progress
│   │   ├── browser.go       # main browser
│   │   ├── confirm.go       # delete confirmation modal
│   │   └── style.go         # colors, layout constants
│   └── recycle/
│       ├── recycle_windows.go
│       └── recycle_test.go
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```
