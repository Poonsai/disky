# disky

A lightweight terminal disk-space analyzer for Windows. Think of it as `ncdu` for Windows: pick a drive, watch a fast parallel scan, then navigate folders sorted by size and delete what you don't need to the Recycle Bin.

## Install

Prerequisites: Go 1.22+ and Windows. (Current toolchain pinned to 1.26 in `go.mod`; older versions may still compile — try and see.)

```powershell
go install github.com/Poonsai/disky/cmd/disky@latest
```

Or build from source:

```powershell
git clone https://github.com/Poonsai/disky.git
cd disky
go build -o disky.exe ./cmd/disky
```

For a release-style build that embeds the version string:

```powershell
go build -ldflags "-s -w -X main.version=v1.1.3" -o disky.exe ./cmd/disky
```

## Usage

```powershell
disky              # launch the TUI
disky --version    # print version and exit (also -v)
```

The TUI opens with an interactive drive picker, then a progress screen while the drive is scanned, then a sortable folder browser.

### Keys (browser)

| Key                       | Action                                     |
| ------------------------- | ------------------------------------------ |
| `↑` / `↓` (or `k` / `j`)  | Move selection (cursor)                    |
| `→` / `Enter`             | Enter selected folder                      |
| `←` / `Backspace` / `h`   | Go to parent                               |
| `Space`                   | Toggle bulk-select on cursor row           |
| `Shift+↑` / `Shift+↓`     | Range-select (or `K` / `J`)                |
| `a` / `A`                 | Select all / clear selection               |
| `d`                       | Delete selection, or cursor row if none (Recycle Bin, confirmable) |
| `r`                       | Rescan current folder                      |
| `g` / `G`                 | Jump to top / bottom                       |
| `q` / `Esc`               | Quit                                       |

## Design

See [docs/superpowers/specs/2026-05-16-disky-design.md](docs/superpowers/specs/2026-05-16-disky-design.md) for the full architecture.

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT. See [LICENSE](LICENSE).
