package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func CreateApplication() {
	a := app.NewWithID("com.github.nicky-ayoub/fyslide")
	w := a.NewWindow("FySlide")
	w.Resize(fyne.NewSize(1024, 768))
	w.CenterOnScreen()

	// Create the Gui
	gui := NewGui(&w)
	w.SetContent(gui.makeGui())

	w.ShowAndRun()
}
