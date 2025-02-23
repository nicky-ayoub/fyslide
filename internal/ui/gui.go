package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type gui struct {
	win *fyne.Window
}

func NewGui(win *fyne.Window) *gui {
	return &gui{
		win: win,
	}
}

func (g *gui) makeGui() fyne.CanvasObject {
	// TODO: make the ui
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentCreateIcon(), func() {
			log.Println("Create Pressed on toolbar")
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.ContentCutIcon(), func() { log.Println("Cut Pressed on toolbar") }),
		widget.NewToolbarAction(theme.ContentCopyIcon(), func() { log.Println("Copy Pressed on toolbar") }),
		widget.NewToolbarAction(theme.ContentPasteIcon(), func() { log.Println("Paste Pressed on toolbar") }),
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			log.Println("Display help")
		}),
	)
	return container.NewBorder(
		toolbar, //top
		widget.NewButton("About", func() { g.showAboutDialog() }), //bottom
		nil, //left
		nil, //right
		container.NewCenter(widget.NewLabel("We're just getting started...")))
}

func (g *gui) showAboutDialog() {
	about := NewAbout(g.win, "About", resourceIconPng)
	about.Show()
}
