package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func (a *App) buildKeyboardShortcuts() {
	// keyboard shortcuts

	// ctrl+q to quit application
	a.MainWin.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyQ,
		Modifier: a.mainModKey,
	}, func(shortcut fyne.Shortcut) { a.app.Quit() })

	a.MainWin.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		// move forward/back within the current folder of images
		case fyne.KeyRight:
			a.direction = 1
			a.nextImage()
		case fyne.KeyLeft:
			a.direction = -1
			a.nextImage()
		case fyne.KeyQ:
			a.app.Quit()
		case fyne.KeyP, fyne.KeySpace:
			a.togglePlay()
		case fyne.KeyPageUp, fyne.KeyUp:
			a.index -= 20
			a.nextImage()
		case fyne.KeyPageDown, fyne.KeyDown:
			a.index += 20
			a.nextImage()
		case fyne.KeyHome:
			a.firstImage()
		case fyne.KeyEnd:
			a.lastImage()
		case fyne.KeyDelete:
			a.deleteFileCheck()
		// close dialogs with esc key
		case fyne.KeyEscape:
			if len(a.MainWin.Canvas().Overlays().List()) > 0 {
				a.MainWin.Canvas().Overlays().Top().Hide()
			}
		}
	})
}

func (a *App) showShortcuts() {
	shortcuts := []string{
		"Ctrl+Q",
		"Arrow Right", "Arrow Left",
		"Page Up", "Page Down",
	}
	descriptions := []string{
		"Quit Application",
		"Next Image", "Previous Image",
		"Skip 20 Images Back", "Skip 20 Images Forward",
	}

	win := a.app.NewWindow("Keyboard Shortcuts")
	table := widget.NewTable(
		func() (int, int) { return len(shortcuts), 2 },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				if id.Col == 0 {
					label.SetText("Description")
					label.TextStyle.Bold = true
				} else {
					label.SetText("Shortcut")
					label.TextStyle.Bold = true
				}
			} else {
				if id.Col == 0 {
					label.SetText(descriptions[id.Row-1])
				} else {
					label.SetText(descriptions[id.Row-1])
				}
			}
		},
	)
	table.SetColumnWidth(0, 250)
	table.SetColumnWidth(1, 250)
	win.SetContent(table)
	win.Resize(fyne.NewSize(500, 500))
	win.Show()
}
