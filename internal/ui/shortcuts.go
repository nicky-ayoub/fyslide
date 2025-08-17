// Package ui  Shortcuts for keyboard actions
package ui

import (
	"fyslide/internal/custom_widgets"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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
		// --- Image Navigation ---
		case fyne.KeyRight:
			a.Navigation.Navigate(1)
		case fyne.KeyLeft:
			a.Navigation.ShowPreviousImage()
		case fyne.KeyQ:
			a.app.Quit()
		case fyne.KeyP, fyne.KeySpace: // Toggle Play
			a.TogglePlay()
		case fyne.KeyPageUp, fyne.KeyUp:
			a.Navigation.Navigate(-a.skipCount)
		case fyne.KeyPageDown, fyne.KeyDown:
			a.Navigation.Navigate(a.skipCount)
		case fyne.KeyHome:
			a.Navigation.FirstImage()
		case fyne.KeyEnd:
			a.Navigation.LastImage()
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
			if a.zoomPanArea != nil && a.UI.contentStack.Objects[custom_widgets.ImageViewIndex].Visible() {
				a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: 1}}) // Positive DY for zoom in
			}
		case fyne.KeyMinus: // Numpad Subtract or regular '-' / '_'
			a.slideshowManager.Pause(true) // Pause slideshow
			if a.zoomPanArea != nil && a.UI.contentStack.Objects[custom_widgets.ImageViewIndex].Visible() {
				a.zoomPanArea.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -1}}) // Negative DY for zoom out
			}
		case fyne.Key0, fyne.KeyInsert: // Reset zoom/pan
			// Resetting zoom/pan might also warrant a pause, depending on desired behavior.
			// If so, uncomment the line below.
			// a.slideshowManager.Pause(true) // Pause slideshow

			if a.zoomPanArea != nil && a.UI.contentStack.Objects[custom_widgets.ImageViewIndex].Visible() {
				a.zoomPanArea.Reset()
			}

		}
	})
}

func (a *App) showShortcuts() {
	win := a.app.NewWindow("Keyboard Shortcuts")
	shortcutsWidget := custom_widgets.NewShortcutTable()
	win.SetContent(shortcutsWidget)
	win.Resize(fyne.NewSize(520, 500))
	win.Show()
}
