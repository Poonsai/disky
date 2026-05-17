# Bulk select for delete

## Overview

Add multi-select to the browser so users can pick several files/folders at once
and recycle them with a single `d` keypress. Selection is scoped to the
currently-viewed folder and clears on navigation.

## User flow

1. User is in the browser viewing a folder.
2. User moves the cursor to an item and presses `Space` to toggle its selection.
   Selected rows show a `*` marker before the name.
3. User can repeat `Space` on other rows, press `a` to select everything in the
   folder, or press `A` (Shift+A) to clear the whole selection.
4. User presses `d`:
   - If at least one item is selected, the confirm dialog summarizes the
     selection (count, total size, bullet list of up to 5 paths with
     "... and X more" if longer) and the recycle batch runs on confirmation.
   - If nothing is selected, `d` keeps its current single-item behavior
     (target = item under the cursor).
5. On confirmation, each item is recycled in order. Best-effort: items that
   fail (locked, permission denied) stay in the tree and stay selected; items
   that succeed are removed. A toast summarizes the outcome
   (e.g. `"deleted 7 of 9; 2 failed: foo, bar"`).
6. Navigating into a subfolder or back to the parent clears the selection.

## Visual mockups

Browser with two items selected, cursor on the third row:

```
C:\Users\Boozer\AppData\Local                              4.2 GB
────────────────────────────────────────────────────────────────
       2.1 GB  ████████████  *   npm-cache
       1.4 GB  ████████      *   pip
       512 MB  ███               yarn-cache       ← cursor (reverse video)
        80 MB  ▌                 Temp
        12 MB                [!] Microsoft

↑/↓ navigate  →/Enter open  ←/Backspace back  Space select  a all  A clear  d delete  r rescan  q quit
```

Confirm dialog with three selected:

```
Move to Recycle Bin?

  3 items (3.6 GB)

  • C:\Users\Boozer\AppData\Local\npm-cache
  • C:\Users\Boozer\AppData\Local\pip
  • C:\Users\Boozer\AppData\Local\yarn-cache

[Enter] confirm    [Esc] cancel
```

Confirm dialog with many selected (truncated to first 5 + count):

```
Move to Recycle Bin?

  12 items (8.4 GB)

  • node_modules
  • dist
  • .next
  • build
  • coverage
  ... and 7 more

[Enter] confirm    [Esc] cancel
```

## Design

### Keybindings (browser)

| Key       | Action                                          |
|-----------|-------------------------------------------------|
| `Space`   | Toggle selection on the cursor row              |
| `a`       | Select every item in the current folder         |
| `A`       | Clear the entire selection                      |
| `d`       | Delete the selection (or cursor row if empty)   |

All other browser keys keep their current behavior. Navigation keys
(`Enter`, `Right`, `Backspace`, `Left`, `h`) clear the selection as a
side-effect, because selection scope is per-folder.

### Component changes

#### `internal/tui/browser.go`

- Add `Selected map[*tree.Node]struct{}` field on `BrowserModel`. Lazily
  initialized on first insert.
- Replace `PendingDelete *tree.Node` with `PendingDeletes []*tree.Node`.
  Length 1 for cursor-driven delete, length N for batch.
- Key handlers:
  - `space`: toggle `Current.Children[Cursor]` in `Selected`
  - `a`: insert every `Current.Children[i]` into `Selected`
  - `A`: reset `Selected` to nil
  - `d`: if `len(Selected) > 0`, set `PendingDeletes` to the selected items
    in current-folder display order; else keep the existing single-item
    behavior using the cursor row.
  - Navigation keys clear `Selected` before changing `Current` / `Cursor`.
- Row layout adds a 2-cell selection slot between the bar and the existing
  `[!]` error marker. The format becomes
  `"  %10s  %s  %s%s%s"` = padding + size + bar + selMarker + errMarker + name.
  `selMarker` is `"* "` for selected rows, `"  "` otherwise. Constant
  `fixedCells` bumps from `16 + BarWidth` to `18 + BarWidth`.
- New method `ApplyBatchDelete(succeeded []*tree.Node) BrowserModel`:
  removes each succeeded node via `tree.RemoveAndRecompute`, re-sorts the
  current folder and ancestors, clamps cursor, clears `PendingDeletes`,
  and removes successful nodes from `Selected`. Items that failed stay in
  `Selected` so the user can retry.
- Update help line text:
  `↑/↓ navigate   →/Enter open   ←/Backspace back   Space select   a all   A clear   d delete   r rescan   q quit`.
- `CancelDelete` clears `PendingDeletes` (no other state change). Selection
  is preserved so the user can adjust and retry.

#### `internal/tui/confirm.go`

- Generalize `ConfirmModel` to take a list. New struct shape:
  ```go
  type ConfirmModel struct {
      Items  []ConfirmItem // length >= 1
      Result ConfirmResult
  }
  type ConfirmItem struct {
      Path      string
      Size      int64
      ItemCount int // 0 for files
  }
  ```
- `NewConfirm` takes `[]ConfirmItem`. Single-item call sites pass a
  one-element slice.
- `View`:
  - Title stays "Move to Recycle Bin?".
  - Header: `"  N items (TOTAL_SIZE)"` summing across `Items`.
    For N == 1, render exactly as today (`"  path"` + size line) to keep
    the single-delete experience unchanged.
  - For N > 1, render up to the first 5 paths as bullets, then
    `"  ... and X more"` if truncated.

#### `cmd/disky/main.go` `handleDelete`

- Read `PendingDeletes` (was `PendingDelete`). Build `[]ConfirmItem` from
  the list, including per-item `countItems` for directories.
- After confirm:
  - Iterate `PendingDeletes` in order. For each, call `recycle.Send`. On
    success append to `succeeded`; on failure append to `failed` (along
    with the error message).
  - Call `bm.ApplyBatchDelete(succeeded)`.
  - Toast: `""` if no failures, otherwise
    `"deleted X of Y; failed: <name1>, <name2>"` (truncate the names list
    at 3 with "… and Z more" tail if longer).

### Selection lifetime

- Created lazily on first `Space`/`a`.
- Mutated by Space / a / A handlers only.
- Trimmed by `ApplyBatchDelete` (successful items removed; failed items
  remain selected).
- Cleared entirely on any navigation key (`enter`, `right`, `backspace`,
  `left`, `h`).
- Persisted across `r` (rescan) is NOT a concern: rescan replaces
  `Current.Children` with fresh nodes, so any pointers in `Selected` would
  be stale. `ApplyRescan` clears `Selected` to avoid acting on stale
  pointers.

### Edge cases

| Scenario | Behavior |
|---|---|
| `Space` on empty folder | No-op (cursor < 0 not possible since cursor sits at 0 with no children) |
| `a` on empty folder | No-op |
| `A` with empty selection | No-op |
| `d` with empty selection AND empty folder | No-op (no cursor target) |
| `d` then cancel via Esc | `PendingDeletes` cleared; selection preserved |
| Rescan during selection | Selection cleared (stale pointers) |
| Toast interaction | `Space` keypress dismisses any existing toast first (existing behavior), then toggles selection |

## Testing

### `internal/tui/browser_test.go` additions

- `TestBrowserSpaceToggleSelectsCursorItem`
- `TestBrowserSelectAllPopulatesSelected`
- `TestBrowserShiftAClearsSelection`
- `TestBrowserNavigationClearsSelection` (enter + backspace cases)
- `TestBrowserDeleteUsesSelectionWhenNonEmpty` (verifies `PendingDeletes`
  has the selected items, not the cursor item)
- `TestBrowserDeleteFallsBackToCursorWhenSelectionEmpty`
- `TestBrowserApplyBatchDeleteRemovesSucceededOnly` (succeeded items gone,
  failed item stays in Selected and in tree)

### `internal/tui/confirm_test.go` additions

- `TestConfirmSingleItemRendersUnchanged` (regression — output bytes match
  the pre-feature format)
- `TestConfirmMultiItemSummary` (N items, total size line, bullets, no
  truncation tail when count <= 5)
- `TestConfirmMultiItemTruncatesPaths` (count > 5 → "... and X more")

### Manual smoke test

Run `disky.exe` against a populated test folder:
1. Select 2-3 items with Space; verify `*` markers appear.
2. Press `a`; verify all rows marked.
3. Press `A`; verify markers cleared.
4. Select 3 items; press `d`; verify confirm dialog shows count, total
   size, bullet list.
5. Confirm; verify all three removed from the tree.
6. Re-select; cancel with Esc; verify selection survives.
7. Select; navigate into a subfolder; navigate back; verify selection
   cleared.

## Out of scope

These are listed only to make the boundary clear; they will NOT be built
under this spec:

- Range selection (`Shift`+`↓` to select-to-here)
- "Invert selection" command
- Cross-folder selection (selection that survives navigation)
- A dedicated "review selected" view
- Drag-and-paint selection with mouse

## File summary

| File | Change |
|---|---|
| `internal/tui/browser.go` | Selection field, keybindings, row marker, batch apply, help text |
| `internal/tui/browser_test.go` | New tests for selection behaviors |
| `internal/tui/confirm.go` | `Items []ConfirmItem` API, multi-item view |
| `internal/tui/confirm_test.go` | New tests for single and multi item rendering |
| `cmd/disky/main.go` | Batch loop in `handleDelete`, summary toast |
