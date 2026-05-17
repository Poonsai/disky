// Command disky is a lightweight terminal disk-space analyzer for Windows.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Poonsai/disky/internal/drives"
	"github.com/Poonsai/disky/internal/recycle"
	"github.com/Poonsai/disky/internal/tree"
	"github.com/Poonsai/disky/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "disky:", err)
		os.Exit(1)
	}
}

func run() error {
	drv, err := pickDrive()
	if err != nil || drv == nil {
		return err
	}

	root, err := runScan(drv.Letter)
	if err != nil {
		return err
	}
	if root == nil || len(root.Children) == 0 {
		fmt.Println("Scan returned no entries.")
		return nil
	}

	return browse(root)
}

func pickDrive() (*drives.Drive, error) {
	list, err := drives.List()
	if err != nil {
		return nil, fmt.Errorf("list drives: %w", err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no drives found")
	}

	final, err := tea.NewProgram(tui.NewPicker(list), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	pm := final.(tui.PickerModel)
	return pm.Chosen, nil
}

func runScan(root string) (*tree.Node, error) {
	final, err := tea.NewProgram(tui.NewProgress(root), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	pm := final.(tui.ProgressModel)
	if pm.Err != nil {
		return nil, pm.Err
	}
	return pm.Result, nil
}

func browse(root *tree.Node) error {
	bm := tui.NewBrowser(root)
	for {
		final, err := tea.NewProgram(bm, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		bm = final.(tui.BrowserModel)

		switch {
		case bm.PendingDelete != nil:
			bm = handleDelete(bm)
		case bm.PendingRescan:
			bm = handleRescan(bm)
		default:
			// User quit.
			return nil
		}
	}
}

func handleDelete(bm tui.BrowserModel) tui.BrowserModel {
	target := bm.PendingDelete
	itemCount := 0
	if target.IsDir {
		itemCount = countItems(target)
	}

	cm := tui.NewConfirm(target.Path(), target.Size, itemCount)
	final, err := tea.NewProgram(cm, tea.WithAltScreen()).Run()
	if err != nil {
		return bm.CancelDelete()
	}
	if final.(tui.ConfirmModel).Result != tui.ConfirmYes {
		return bm.CancelDelete()
	}

	if err := recycle.Send(target.Path()); err != nil {
		bm = bm.CancelDelete()
		bm.Toast = fmt.Sprintf("could not delete %s: %v", target.Name, err)
		return bm
	}
	return bm.ApplyDelete(target)
}

func handleRescan(bm tui.BrowserModel) tui.BrowserModel {
	path := bm.Current.Path()
	final, err := tea.NewProgram(tui.NewProgress(path), tea.WithAltScreen()).Run()
	if err != nil {
		return bm.CancelRescan()
	}
	pm := final.(tui.ProgressModel)
	// pm.Err non-nil today only means context.Canceled (user pressed q).
	// pm.Result.Err non-nil means the root itself couldn't be read (e.g.
	// the folder was deleted/renamed/locked between scans). In both
	// cases the tree we got back doesn't reflect the on-disk state, and
	// applying it would silently shrink ancestor totals.
	if pm.Err != nil || pm.Result == nil || pm.Result.Err != nil {
		bm = bm.CancelRescan()
		if pm.Result != nil && pm.Result.Err != nil {
			bm.Toast = fmt.Sprintf("rescan failed: %v", pm.Result.Err)
		}
		return bm
	}
	return bm.ApplyRescan(pm.Result)
}

// countItems counts the descendants of n (files + subdirectories), not
// including n itself. So a folder with 3 files reports 3, not 4.
func countItems(n *tree.Node) int {
	count := 0
	for _, c := range n.Children {
		count += 1 + countItems(c)
	}
	return count
}
