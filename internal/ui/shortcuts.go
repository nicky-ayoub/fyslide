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
			a.nextImage()
		case fyne.KeyLeft:
			a.ShowPreviousImage()
		case fyne.KeyQ:
			a.app.Quit()
		case fyne.KeyP, fyne.KeySpace:
			a.togglePlay()
		case fyne.KeyPageUp, fyne.KeyUp:
			a.skipImages(-a.skipCount) // Use new skipImages method
		case fyne.KeyPageDown, fyne.KeyDown:
			a.skipImages(a.skipCount) // Use new skipImages method
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

type shortcutDetail struct {
	Description string
	Shortcut    string
}

func (a *App) showShortcuts() {
	shortcutData := []shortcutDetail{
		{Description: "Quit Application", Shortcut: "Ctrl+Q"},
		{Description: "Next Image", Shortcut: "Arrow Right"},
		{Description: "Previous Image", Shortcut: "Arrow Left"},
		{Description: "Skip Images Back (Page Up)", Shortcut: "Page Up"},
		{Description: "Skip Images Forward (Page Down)", Shortcut: "Page Down"},
		{Description: "Skip Images Back (Arrow Up)", Shortcut: "Arrow Up"},
		{Description: "Skip Images Forward (Arrow Down)", Shortcut: "Arrow Down"},
		{Description: "First Image", Shortcut: "Home"},
		{Description: "Last Image", Shortcut: "End"},
		{Description: "Toggle Play/Pause Slideshow", Shortcut: "P or Space"},
		{Description: "Delete Current Image", Shortcut: "Delete"},
		{Description: "Close Dialog/Overlay", Shortcut: "Esc"},
		{Description: "Zoom In Image", Shortcut: "+"},
		{Description: "Zoom Out Image", Shortcut: "-"},
		{Description: "Reset Image Zoom/Pan", Shortcut: "0"},
	}

	win := a.app.NewWindow("Keyboard Shortcuts")
	table := widget.NewTable(
		func() (int, int) { return len(shortcutData) + 1, 2 }, // +1 for header row
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			isHeader := id.Row == 0 // First row is header
			dataIndex := id.Row - 1

			if id.Col == 0 { // Description column
				if isHeader {
					label.SetText("Description")
				} else {
					label.SetText(shortcutData[dataIndex].Description)
				}
			} else { // Shortcut column
				if isHeader {
					label.SetText("Shortcut")
				} else {
					label.SetText(shortcutData[dataIndex].Shortcut)
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
