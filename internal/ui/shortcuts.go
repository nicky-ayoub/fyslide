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
		// Zoom and Pan shortcuts - only if image view is active
		case fyne.KeyPlus: // Numpad Add or regular '+' / '='
			a.slideshowManager.Pause(true) // Pause slideshow
			if a.zoomPanArea != nil && a.UI.contentStack.Objects[imageViewIndex].Visible() {
				a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: 1}}) // Positive DY for zoom in
			}
		case fyne.KeyMinus: // Numpad Subtract or regular '-' / '_'
			a.slideshowManager.Pause(true) // Pause slideshow
			if a.zoomPanArea != nil && a.UI.contentStack.Objects[imageViewIndex].Visible() {
				a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -1}}) // Negative DY for zoom out
			}
		case fyne.Key0, fyne.KeyInsert: // Reset zoom/pan
			// Resetting zoom/pan might also warrant a pause, depending on desired behavior.
			// If so, uncomment the line below.
			// a.slideshowManager.Pause(true) // Pause slideshow

			if a.zoomPanArea != nil && a.UI.contentStack.Objects[imageViewIndex].Visible() {
				a.zoomPanArea.Reset()
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
		"+", // Zoom In
		"-", // Zoom Out
		"0", // Reset Zoom/Pan
	}
	descriptions := []string{
		"Quit Application", // Simplified, as Ctrl+Q is also listed
		"Next Image", "Previous Image",
		"Skip Images Back (Page Up)", "Skip Images Forward (Page Down)",
		"Skip Images Back (Arrow Up)", "Skip Images Forward (Arrow Down)", // Descriptions for Arrow Up/Down
		"First Image", "Last Image",
		"Toggle Play/Pause Slideshow", "Delete Current Image", "Close Dialog/Overlay",
		"Zoom In Image",
		"Zoom Out Image",
		"Reset Image Zoom/Pan",
	}
	// Ensure shortcuts and descriptions have the same length for safety
	// This is a defensive check; ideally, they are maintained to be equal.
	minLen := min(len(shortcuts), len(descriptions))

	win := a.app.NewWindow("Keyboard Shortcuts")
	table := widget.NewTable(
		func() (int, int) { return minLen + 1, 2 }, // +1 for header row, use minLen
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			isHeader := id.Row == 0 // First row is header
			dataRowIndex := id.Row - 1

			if !isHeader && dataRowIndex >= minLen { // Safety break
				label.SetText("") // Avoid panic
				return
			}
			if id.Col == 0 { // Description column
				if isHeader {
					label.SetText("Description")
				} else {
					label.SetText(descriptions[dataRowIndex])
				}
			} else { // Shortcut column
				if isHeader {
					label.SetText("Shortcut")
				} else {
					label.SetText(shortcuts[dataRowIndex])
				}
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

// ternaryString is a helper, assuming it's defined elsewhere or should be local.
// If it's the one from app.go, ensure it's accessible or duplicate if needed.
func ternaryString(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

// min returns the smaller of x or y.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
