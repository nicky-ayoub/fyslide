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

func (a *App) buildSatusBar() *fyne.Container {
	a.first = widget.NewButtonWithIcon("", theme.MediaFastRewindIcon(), a.firstImage)
	a.leftArrow = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { a.direction = -1; a.nextImage() })
	a.pauseBtn = widget.NewButtonWithIcon("", theme.MediaPauseIcon(), a.togglePlay)
	a.rightArrow = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { a.direction = 1; a.nextImage() })
	a.last = widget.NewButtonWithIcon("", theme.MediaFastForwardIcon(), a.lastImage)
	a.tagBtn = widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), a.tagFile)
	a.deleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), a.deleteFileCheck)
	a.statusLabel = widget.NewLabel("")
	a.leftArrow.Enable()
	a.rightArrow.Enable()
	a.deleteBtn.Enable()
	a.tagBtn.Enable()
	a.first.Enable()
	a.last.Enable()

	a.statusBar = container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			a.first,
			a.leftArrow,
			a.pauseBtn,
			a.rightArrow,
			a.last,
			a.tagBtn,
			a.deleteBtn,
			layout.NewSpacer(),
			a.statusLabel,
		),
	)
	return a.statusBar
}

func (a *App) buildInformationTab() *container.TabItem {
	a.countLabel = widget.NewLabel("Count: ")
	a.widthLabel = widget.NewLabel("Width: ")
	a.heightLabel = widget.NewLabel("Height: ")
	a.imgSize = widget.NewLabel("Size: ")
	a.imgLastMod = widget.NewLabel("Last modified: ")
	return container.NewTabItem("Information", container.NewScroll(
		container.NewVBox(
			a.countLabel,
			a.widthLabel,
			a.heightLabel,
			a.imgSize,
			a.imgLastMod,
		),
	))
}

func (a *App) buildToolbar() *widget.Toolbar {
	t := widget.NewToolbar(

		widget.NewToolbarAction(theme.MediaFastRewindIcon(), a.firstImage),
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() { a.direction = -1; a.nextImage() }),
		widget.NewToolbarAction(theme.MediaPauseIcon(), a.togglePlay),
		widget.NewToolbarAction(theme.NavigateNextIcon(), func() { a.direction = 1; a.nextImage() }),
		widget.NewToolbarAction(theme.MediaFastForwardIcon(), a.lastImage),
		widget.NewToolbarAction(theme.DocumentCreateIcon(), a.tagFile),
		widget.NewToolbarAction(theme.DeleteIcon(), a.deleteFileCheck),
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			log.Println("Display help")
		}),
	)
	return t
}

func (a *App) buildMainUI() fyne.CanvasObject {
	a.MainWin.SetMaster()
	// set main mod key to super on darwin hosts, else set it to ctrl
	if runtime.GOOS == "darwin" {
		a.mainModKey = fyne.KeyModifierSuper
	} else {
		a.mainModKey = fyne.KeyModifierControl
	}
	a.toolbar = a.buildToolbar()
	status := a.buildSatusBar()

	// a.fileTree = binding.NewURITree()
	// files := widget.NewTreeWithData(a.fileTree, func(branch bool) fyne.CanvasObject {
	// 	return widget.NewLabel("filename.ext")
	// }, func(data binding.DataItem, branch bool, obj fyne.CanvasObject) {
	// 	l := obj.(*widget.Label)
	// 	u, _ := data.(binding.URI).Get()
	// 	name := u.Name()
	// 	l.SetText(name)
	// })

	// explorer := widget.NewAccordion(
	// 	widget.NewAccordionItem("Files", files),
	// 	widget.NewAccordionItem("Data", widget.NewLabel("data")),
	// )

	// main menu
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("File"),

		fyne.NewMenu("Edit",
			fyne.NewMenuItem("Delete Image", a.deleteFileCheck),
			fyne.NewMenuItem("Keyboard Shortucts", a.showShortcuts),
		),
		fyne.NewMenu("View",
			fyne.NewMenuItem("Next Image", func() { a.direction = 1; a.nextImage() }),
			fyne.NewMenuItem("Previous Image", func() { a.direction = -1; a.nextImage() }),
		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About", func() {
				dialog.ShowCustom("About", "Ok", container.NewVBox(
					widget.NewLabel("A simple image slide show."),
					widget.NewHyperlink("Help and more information on Github", parseURL("https://github.com/nicky-ayoub/fyslide")),
					widget.NewLabel("v1.2 | License: MIT"),
				), a.MainWin)
			}),
		),
	)
	a.MainWin.SetMainMenu(mainMenu)
	a.buildKeyboardShortcuts()

	// image canvas
	a.image = &canvas.Image{}
	a.image.FillMode = canvas.ImageFillContain

	a.split = container.NewHSplit(
		a.image,
		container.NewAppTabs(
			a.buildInformationTab(),
		),
	)
	a.split.SetOffset(0.90)
	return container.NewBorder(
		a.toolbar, // Top
		status,    // Bottom
		nil,       // explorer left
		nil,       // right
		a.split,
	)
}
