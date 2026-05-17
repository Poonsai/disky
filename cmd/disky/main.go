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

	root, canceled, err := runScan(drv.Letter)
	if err != nil {
		return err
	}
	if canceled {
		// User pressed q/esc/ctrl+c. Exit silently rather than printing a
		// misleading "no entries" message that implies the drive is empty.
		return nil
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

// runScan returns (tree, canceled, err). canceled is true when the user
// pressed q/esc/ctrl+c on the progress screen, distinguishing that from a
// genuinely empty scan or an underlying failure.
func runScan(root string) (*tree.Node, bool, error) {
	final, err := tea.NewProgram(tui.NewProgress(root), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, false, err
	}
	pm := final.(tui.ProgressModel)
	if pm.Canceled {
		return nil, true, nil
	}
	if pm.Err != nil {
		return nil, false, pm.Err
	}
	// The scan can return a non-nil Result with Result.Err set when the root
	// itself was unreadable (drive ejected, permission denied). Surface that
	// instead of silently treating the drive as empty.
	if pm.Result != nil && pm.Result.Err != nil {
		return nil, false, fmt.Errorf("scan %s: %w", root, pm.Result.Err)
	}
	return pm.Result, false, nil
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
	// Three "don't apply" cases:
	//   pm.Canceled       — user pressed q/esc/ctrl+c during rescan
	//   pm.Result == nil  — scan returned nothing (shouldn't happen, defensive)
	//   pm.Result.Err     — root path was unreadable mid-rescan (deleted /
	//                       renamed / locked); applying the partial tree
	//                       would silently shrink ancestor totals
	if pm.Canceled || pm.Err != nil || pm.Result == nil || pm.Result.Err != nil {
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
