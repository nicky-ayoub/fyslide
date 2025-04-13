package ui

import (
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) buildStatusBar() *fyne.Container {
	a.UI.quit = widget.NewButtonWithIcon("", theme.CancelIcon(), func() { a.app.Quit() })
	a.UI.firstBtn = widget.NewButtonWithIcon("", theme.MediaSkipPreviousIcon(), a.firstImage)
	a.UI.previousBtn = widget.NewButtonWithIcon("", resourceBackPng, func() { a.direction = -1; a.nextImage() })
	a.UI.pauseBtn = widget.NewButtonWithIcon("", theme.MediaPauseIcon(), a.togglePlay)
	a.UI.nextBtn = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() { a.direction = 1; a.nextImage() })
	a.UI.lastBtn = widget.NewButtonWithIcon("", theme.MediaSkipNextIcon(), a.lastImage)
	// Use the renamed function addTag (if you renamed it)
	a.UI.tagBtn = widget.NewButtonWithIcon("", theme.DocumentIcon(), a.addTag) // Changed from a.tagFile
	// You could add a remove tag button here too if desired
	a.UI.removeTagBtn = widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), a.removeTag) // Need to add removeTagBtn to UI struct

	a.UI.deleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), a.deleteFileCheck)
	a.UI.randomBtn = widget.NewButtonWithIcon("", resourceDice24Png, a.toggleRandom)

	a.UI.statusLabel = widget.NewLabel("")
	a.UI.previousBtn.Enable()
	a.UI.nextBtn.Enable()
	a.UI.deleteBtn.Enable()
	a.UI.tagBtn.Enable()
	a.UI.firstBtn.Enable()
	a.UI.lastBtn.Enable()
	a.UI.randomBtn.Enable()

	s := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			a.UI.quit,
			a.UI.firstBtn,
			a.UI.previousBtn,
			a.UI.pauseBtn,
			a.UI.nextBtn,
			a.UI.lastBtn,
			a.UI.tagBtn,
			a.UI.removeTagBtn,
			a.UI.deleteBtn,
			a.UI.randomBtn,
			layout.NewSpacer(),
			a.UI.statusLabel,
		),
	)
	return s
}

func (a *App) buildInformationTab() *container.TabItem {
	a.UI.clockLabel = widget.NewLabel("Time: ")
	a.UI.infoText = widget.NewRichTextFromMarkdown("# Info\n---\n")
	return container.NewTabItem("Information", container.NewScroll(
		container.NewVBox(
			a.UI.clockLabel,
			a.UI.infoText,
		),
	))
}

func (a *App) buildToolbar() *widget.Toolbar {
	a.UI.randomAction = widget.NewToolbarAction(resourceDice24Png, a.toggleRandom)
	a.UI.pauseAction = widget.NewToolbarAction(theme.MediaPauseIcon(), a.togglePlay)

	t := widget.NewToolbar(
		widget.NewToolbarAction(theme.CancelIcon(), func() { a.app.Quit() }),
		widget.NewToolbarAction(theme.MediaFastRewindIcon(), a.firstImage),
		widget.NewToolbarAction(resourceBackPng, func() { a.direction = -1; a.nextImage() }),
		a.UI.pauseAction,
		widget.NewToolbarAction(theme.MediaPlayIcon(), func() { a.direction = 1; a.nextImage() }),
		widget.NewToolbarAction(theme.MediaFastForwardIcon(), a.lastImage),
		// Use the renamed function addTag (if you renamed it)
		widget.NewToolbarAction(theme.DocumentIcon(), a.addTag), // Changed from a.tagFile
		// You could add a remove tag button here too if desired
		widget.NewToolbarAction(theme.ContentRemoveIcon(), a.removeTag),
		widget.NewToolbarAction(theme.DeleteIcon(), a.deleteFileCheck),
		a.UI.randomAction,
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			log.Println("Display help")
		}),
	)

	return t
}

func (a *App) buildMainUI() fyne.CanvasObject {
	a.UI.MainWin.SetMaster()
	// set main mod key to super on darwin hosts, else set it to ctrl
	if runtime.GOOS == "darwin" {
		a.UI.mainModKey = fyne.KeyModifierSuper
	} else {
		a.UI.mainModKey = fyne.KeyModifierControl
	}
	a.UI.toolbar = a.buildToolbar()
	a.UI.statusBar = a.buildStatusBar()

	// main menu
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("File"),
		fyne.NewMenu("Edit",
			fyne.NewMenuItem("Add Tag", a.addTag),
			fyne.NewMenuItem("Remove Tag", a.removeTag),
			fyne.NewMenuItemSeparator(), // Optional separator
			fyne.NewMenuItem("Delete Image", a.deleteFileCheck),
			fyne.NewMenuItem("Keyboard Shortucts", a.showShortcuts),
		),
		fyne.NewMenu("View",
			fyne.NewMenuItem("Next Image", func() { a.direction = 1; a.nextImage() }),
			fyne.NewMenuItem("Previous Image", func() { a.direction = -1; a.nextImage() }),
			fyne.NewMenuItemSeparator(),                              // NEW Separator
			fyne.NewMenuItem("Filter by Tag...", a.showFilterDialog), // NEW Filter option

		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About", func() {
				dialog.ShowCustom("About", "Ok", container.NewVBox(
					widget.NewLabel("A simple image slide show."),
					widget.NewHyperlink("Help and more information on Github", parseURL("https://github.com/nicky-ayoub/fyslide")),
					widget.NewLabel("v1.2 | License: MIT"),
				), a.UI.MainWin)
			}),
		),
	)
	a.UI.MainWin.SetMainMenu(mainMenu)
	a.buildKeyboardShortcuts()

	// image canvas
	a.image = &canvas.Image{}
	a.image.FillMode = canvas.ImageFillContain

	a.UI.split = container.NewHSplit(
		a.image,
		container.NewAppTabs(
			a.buildInformationTab(),
		),
	)
	a.UI.split.SetOffset(0.90)
	return container.NewBorder(
		a.UI.toolbar,   // Top
		a.UI.statusBar, // Bottom
		nil,            // a.UI.explorer, // explorer left
		nil,            // right
		a.UI.split,
	)
}
