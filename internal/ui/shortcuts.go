// Package ui  Shortcuts for keyboard actions
package ui

import (
	"fyslide/internal/customwidgets"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

const (
	keyboardPanStep = 30.0
)

func (a *App) buildKeyboardShortcuts() {
	// keyboard shortcuts

	// ctrl+q to quit application
	a.UI.MainWin.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyQ,
		Modifier: a.UI.mainModKey,
	}, func(_ fyne.Shortcut) { a.app.Quit() })

	// ctrl+right to go to next untagged image
	a.UI.MainWin.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyRight,
		Modifier: a.UI.mainModKey,
	}, func(_ fyne.Shortcut) { a.Navigation.NextUntaggedImage() })

	// ctrl+left to go to previous untagged image
	a.UI.MainWin.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyLeft,
		Modifier: a.UI.mainModKey,
	}, func(_ fyne.Shortcut) { a.Navigation.PreviousUntaggedImage() })

	a.UI.MainWin.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		// --- Shortcuts that only apply when the image view is active ---
		if a.isImageViewActive() {
			switch key.Name {
			case fyne.KeyUp:
				a.zoomPanArea.Pan(fyne.Delta{DY: -keyboardPanStep}) // Pan image up
				return
			case fyne.KeyDown:
				a.zoomPanArea.Pan(fyne.Delta{DY: keyboardPanStep}) // Pan image down
				return
			case fyne.KeyPlus: // Numpad Add or regular '+' / '='
				a.slideshowManager.Pause(true)                                         // Pause slideshow
				a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: 1}}) // Positive DY for zoom in
				return
			case fyne.KeyMinus: // Numpad Subtract or regular '-' / '_'
				a.slideshowManager.Pause(true)                                          // Pause slideshow
				a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -1}}) // Negative DY for zoom out
				return
			case fyne.Key0, fyne.KeyInsert: // Reset zoom/pan
				a.zoomPanArea.Reset()
				return
			}
		}

		// --- Global shortcuts that work in any view ---
		switch key.Name {
		case fyne.KeyRight:
			a.Navigation.Navigate(1)
		case fyne.KeyLeft:
			a.Navigation.ShowPreviousImage()
		case fyne.KeyR:
			a.RotateImage()
		case fyne.KeyQ:
			a.app.Quit()
		case fyne.KeyP, fyne.KeySpace:
			a.TogglePlay()
		case fyne.KeyPageUp:
			a.Navigation.Navigate(-a.skipCount)
		case fyne.KeyPageDown:
			a.Navigation.Navigate(a.skipCount)
		case fyne.KeyHome:
			a.Navigation.FirstImage()
		case fyne.KeyEnd:
			a.Navigation.LastImage()
		case fyne.KeyDelete:
			a.deleteFileCheck()
		case fyne.KeyEscape:
			if len(a.UI.MainWin.Canvas().Overlays().List()) > 0 {
				a.UI.MainWin.Canvas().Overlays().Top().Hide()
			}
		}
	})
}

func (a *App) showShortcuts() {
	win := a.app.NewWindow("Keyboard Shortcuts")
	shortcutsWidget := customwidgets.NewShortcutTable()
	win.SetContent(shortcutsWidget)
	win.Resize(fyne.NewSize(520, 500))
	win.Show()
}
