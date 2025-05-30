// Package ui  Shortcuts for keyboard actions
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func (a *App) buildKeyboardShortcuts() {
	// keyboard shortcuts

	// ctrl+q to quit application
	a.UI.MainWin.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyQ,
		Modifier: a.UI.mainModKey,
	}, func(_ fyne.Shortcut) { a.app.Quit() })

	a.UI.MainWin.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		// move forward/back within the current folder of images
		case fyne.KeyRight:
			a.direction = 1
			a.nextImage()
		case fyne.KeyLeft:
			a.ShowPreviousImage()
		case fyne.KeyQ:
			a.app.Quit()
		case fyne.KeyP, fyne.KeySpace:
			a.togglePlay()
		case fyne.KeyPageUp, fyne.KeyUp:
			a.index -= a.skipCount // Use configurable skip count
			a.nextImage()
		case fyne.KeyPageDown, fyne.KeyDown:
			a.index += a.skipCount // Use configurable skip count
			a.nextImage()
		case fyne.KeyHome:
			a.firstImage()
		case fyne.KeyEnd:
			a.lastImage()
		case fyne.KeyDelete:
			a.deleteFileCheck()
		// close dialogs with esc key
		case fyne.KeyEscape:
			if len(a.UI.MainWin.Canvas().Overlays().List()) > 0 {
				a.UI.MainWin.Canvas().Overlays().Top().Hide()
			}
		}
	})
}

func (a *App) showShortcuts() {
	shortcuts := []string{
		"Ctrl+Q",
		"Arrow Right", "Arrow Left",
		"Page Up", "Page Down",
		"Arrow Up", "Arrow Down", // KeyUp and KeyDown also skip
		"Home", "End",
		"P or Space", "Delete", "Esc",
	}
	descriptions := []string{
		"Quit Application (Ctrl+Q or Q)",
		"Next Image", "Previous Image",
		"Skip 20 Images Back", "Skip 20 Images Forward",
		"First Image", "Last Image",
	}

	win := a.app.NewWindow("Keyboard Shortcuts")
	table := widget.NewTable(
		func() (int, int) { return len(descriptions) + 1, 2 }, // +1 for header row
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			isHeader := id.Row == 0 // First row is header
			dataRowIndex := id.Row - 1

			if id.Col == 0 { // Description column
				label.SetText(ternary(isHeader, "Description", descriptions[dataRowIndex]))
			} else { // Shortcut column
				label.SetText(ternary(isHeader, "Shortcut", shortcuts[dataRowIndex]))
			}
			label.TextStyle.Bold = isHeader
		},
	)
	table.SetColumnWidth(0, 250)
	table.SetColumnWidth(1, 250)
	win.SetContent(table)
	win.Resize(fyne.NewSize(500, 500))
	win.Show()
}
