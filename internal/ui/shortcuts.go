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

// keyHandler defines a function type for handling simple key presses.
type keyHandler func()

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

	// --- Define handlers for OnTypedKey ---

	// imageViewKeyHandlers maps keys to functions that should only run when the image view is active.
	imageViewKeyHandlers := map[fyne.KeyName]keyHandler{
		fyne.KeyUp:   func() { a.zoomPanArea.Pan(fyne.Delta{DY: -keyboardPanStep}) }, // Pan image up
		fyne.KeyDown: func() { a.zoomPanArea.Pan(fyne.Delta{DY: keyboardPanStep}) },  // Pan image down
		fyne.KeyPlus: func() { // Numpad Add or regular '+' / '='
			a.slideshowManager.Pause(true)                                         // Pause slideshow
			a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: 1}}) // Positive DY for zoom in
		},
		fyne.KeyMinus: func() { // Numpad Subtract or regular '-' / '_'
			a.slideshowManager.Pause(true)                                          // Pause slideshow
			a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -1}}) // Negative DY for zoom out
		},
		fyne.Key0:      func() { a.zoomPanArea.Reset() },
		fyne.KeyInsert: func() { a.zoomPanArea.Reset() },
	}

	// globalKeyHandlers maps keys to functions that run in any view.
	globalKeyHandlers := map[fyne.KeyName]keyHandler{
		fyne.KeyRight:    func() { a.Navigation.Navigate(1) },
		fyne.KeyLeft:     func() { a.Navigation.ShowPreviousImage() },
		fyne.KeyR:        func() { a.RotateImage() },
		fyne.KeyQ:        func() { a.app.Quit() },
		fyne.KeyP:        func() { a.TogglePlay() },
		fyne.KeySpace:    func() { a.TogglePlay() },
		fyne.KeyPageUp:   func() { a.Navigation.Navigate(-a.skipCount) },
		fyne.KeyPageDown: func() { a.Navigation.Navigate(a.skipCount) },
		fyne.KeyHome:     func() { a.Navigation.FirstImage() },
		fyne.KeyEnd:      func() { a.Navigation.LastImage() },
		fyne.KeyDelete:   func() { a.deleteFileCheck() },
		fyne.KeyEscape: func() {
			if len(a.UI.MainWin.Canvas().Overlays().List()) > 0 {
				a.UI.MainWin.Canvas().Overlays().Top().Hide()
			}
		},
	}

	a.UI.MainWin.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		// First, check for image-view-specific shortcuts if the view is active.
		if a.isImageViewActive() {
			if handler, ok := imageViewKeyHandlers[key.Name]; ok {
				handler()
				return
			}
		}

		// If no specific handler was found or the view wasn't active, check global shortcuts.
		if handler, ok := globalKeyHandlers[key.Name]; ok {
			handler()
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
