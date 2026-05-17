// Command disky is a lightweight terminal disk-space analyzer for Windows.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boozercab/disky/internal/drives"
	"github.com/boozercab/disky/internal/recycle"
	"github.com/boozercab/disky/internal/tree"
	"github.com/boozercab/disky/internal/tui"
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

	final, err := tea.NewProgram(tui.NewPicker(list)).Run()
	if err != nil {
		return nil, err
	}
	pm := final.(tui.PickerModel)
	return pm.Chosen, nil
}

func runScan(root string) (*tree.Node, error) {
	final, err := tea.NewProgram(tui.NewProgress(root)).Run()
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
		final, err := tea.NewProgram(bm).Run()
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
	final, err := tea.NewProgram(cm).Run()
	if err != nil {
		return bm.CancelDelete()
	}
	if final.(tui.ConfirmModel).Result != tui.ConfirmYes {
		return bm.CancelDelete()
	}

	if err := recycle.Send(target.Path()); err != nil {
		// Print to stderr after Run() returns — proper toast is a follow-up.
		fmt.Fprintln(os.Stderr, "could not delete:", err)
		time.Sleep(2 * time.Second)
		return bm.CancelDelete()
	}
	return bm.ApplyDelete(target)
}

func handleRescan(bm tui.BrowserModel) tui.BrowserModel {
	path := bm.Current.Path()
	final, err := tea.NewProgram(tui.NewProgress(path)).Run()
	if err != nil {
		return bm.CancelRescan()
	}
	pm := final.(tui.ProgressModel)
	if pm.Result == nil {
		return bm.CancelRescan()
	}
	return bm.ApplyRescan(pm.Result)
}

func countItems(n *tree.Node) int {
	count := 1
	for _, c := range n.Children {
		count += countItems(c)
	}
	return count
}
