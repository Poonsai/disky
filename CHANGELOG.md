# Changelog

All notable changes to disky are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Nothing yet._

## [1.1.3] — 2026-05-17

### Fixed

- **Help line was still truncating on 80-col terminals.** The v1.1.1
  trim took the help line from 128 → 88 runes but the default Windows
  Terminal width is 80, so `q quit` was still being clipped. Tightened
  to 71 runes by dropping the `/Enter` and `/Backspace` mentions
  (arrows are obvious; the README key table documents the full set)
  and shortening `all/clear` to `all`. A new regression test asserts
  `q quit` is visible at width 80.

## [1.1.2] — 2026-05-17

### Fixed

- **Crash in batch delete.** `BrowserModel.ApplyBatchDelete` left the
  cursor at `-1` when every child of the current folder was deleted in
  one batch from a non-zero cursor position (e.g. `Down`, `a`, `d`).
  The next keypress in the now-empty folder — `d`, `Space`, `Enter`,
  or `→` — indexed `Children[-1]` and panicked. Clamp now resets the
  cursor to `0` in the empty-folder case.

## [1.1.1] — 2026-05-17

### Added

- `--version` / `-v` flag prints the version and exits. Release binaries
  inject the tag via `-ldflags "-X main.version=v…"`; `go install
  github.com/Poonsai/disky/cmd/disky@vX.Y.Z` users see the right version
  automatically via `runtime/debug.ReadBuildInfo`.

## [1.1.0] — 2026-05-17

### Added

- **Bulk select for delete.** Pick several files/folders in one folder, then
  recycle them with a single `d`.
  - `Space` toggles the cursor row.
  - `Shift+↑` / `Shift+↓` (or `K` / `J` for terminals that don't report
    modifier keys) extend the selection one row at a time.
  - `a` selects everything in the current folder; `A` clears the selection.
  - `d` operates on the selection when non-empty; falls back to the cursor
    row otherwise (single-item behavior unchanged from v1.0.0).
- Confirm dialog generalized to render multi-item summaries — count, total
  size, up to 5 bulleted paths with "... and N more" for longer lists.
- Best-effort batch recycle: items that fail (locked, permission denied)
  stay selected for retry; successes are removed. Summary toast like
  `deleted 7 of 9; failed: foo, bar`.
- Visual `*` marker in a 2-cell row slot for selected rows.

### Changed

- Browser help line trimmed to ~95 runes so the `d` / `r` / `q` keys
  remain visible on 80-column terminals.
- README key table documents the new bindings.
- Selection clears automatically when navigating between folders (`Enter`,
  `Backspace`) and after a rescan (`r`), because the selection holds
  pointers into the previous subtree.

## [1.0.0] — 2026-05-17

### Added

- Initial release: drive picker, parallel scan with progress, sortable
  folder browser, Recycle Bin integration via IFileOperation (COM).
- Bounded worker-pool scanner; bounded goroutine count regardless of tree
  size, with prompt mid-loop cancellation.
- Windows-aware filesystem walker: junctions / reparse points skipped
  (not double-counted), NTFS hard links deduplicated by file ID.
- Long-path-safe recycle (modern shell API; no `MAX_PATH` failures).
- Honest error reporting from the Recycle Bin call — non-recyclable items
  return an HRESULT instead of silently permanent-deleting.

[Unreleased]: https://github.com/Poonsai/disky/compare/v1.1.3...HEAD
[1.1.3]: https://github.com/Poonsai/disky/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/Poonsai/disky/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/Poonsai/disky/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/Poonsai/disky/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Poonsai/disky/releases/tag/v1.0.0
