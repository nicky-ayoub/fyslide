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
	a.UI.first = widget.NewButtonWithIcon("", theme.MediaFastRewindIcon(), a.firstImage)
	a.UI.leftArrow = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { a.direction = -1; a.nextImage() })
	a.UI.pauseBtn = widget.NewButtonWithIcon("", theme.MediaPauseIcon(), a.togglePlay)
	a.UI.rightArrow = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { a.direction = 1; a.nextImage() })
	a.UI.last = widget.NewButtonWithIcon("", theme.MediaFastForwardIcon(), a.lastImage)
	a.UI.tagBtn = widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), a.tagFile)
	a.UI.deleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), a.deleteFileCheck)
	a.UI.statusLabel = widget.NewLabel("")
	a.UI.leftArrow.Enable()
	a.UI.rightArrow.Enable()
	a.UI.deleteBtn.Enable()
	a.UI.tagBtn.Enable()
	a.UI.first.Enable()
	a.UI.last.Enable()

	s := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			a.UI.first,
			a.UI.leftArrow,
			a.UI.pauseBtn,
			a.UI.rightArrow,
			a.UI.last,
			a.UI.tagBtn,
			a.UI.deleteBtn,
			layout.NewSpacer(),
			a.UI.statusLabel,
		),
	)
	return s
}

func (a *App) buildInformationTab() *container.TabItem {
	a.UI.clockLabel = widget.NewLabel("Time: ")
	a.UI.countLabel = widget.NewLabel("Count: ")
	a.UI.widthLabel = widget.NewLabel("Width: ")
	a.UI.heightLabel = widget.NewLabel("Height: ")
	a.UI.imgSize = widget.NewLabel("Size: ")
	a.UI.imgLastMod = widget.NewLabel("Last modified: ")
	return container.NewTabItem("Information", container.NewScroll(
		container.NewVBox(
			a.UI.clockLabel,
			a.UI.countLabel,
			a.UI.widthLabel,
			a.UI.heightLabel,
			a.UI.imgSize,
			a.UI.imgLastMod,
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

// func (a *App) findIndex(target string) int {
// 	for i, v := range a.images {
// 		if v.Path == target {
// 			return i
// 		}
// 	}
// 	return -1
// }

func (a *App) buildMainUI() fyne.CanvasObject {
	a.UI.MainWin.SetMaster()
	a.UI.MainWin.SetMaster()
	// set main mod key to super on darwin hosts, else set it to ctrl
	if runtime.GOOS == "darwin" {
		a.UI.mainModKey = fyne.KeyModifierSuper
	} else {
		a.UI.mainModKey = fyne.KeyModifierControl
	}
	a.UI.toolbar = a.buildToolbar()
	a.UI.toolbar = a.buildToolbar()
	a.UI.statusBar = a.buildSatusBar()

	// if false {
	// 	a.fileTree = binding.NewURITree()
	// 	files := widget.NewTreeWithData(a.fileTree, func(branch bool) fyne.CanvasObject {
	// 		return widget.NewLabel("filename.ext")
	// 	}, func(data binding.DataItem, branch bool, obj fyne.CanvasObject) {
	// 		l := obj.(*widget.Label)
	// 		u, err := data.(binding.URI).Get()
	// 		if err != nil {
	// 			dialog.ShowError(err, a.UI.MainWin)
	// 			return
	// 		}
	// 		l.SetText(u.Name())
	// 	})

	// 	a.UI.explorer = widget.NewAccordion(
	// 		widget.NewAccordionItem("Files", files),
	// 	)
	// 	a.UI.explorer.Open(0)

	// 	files.OnSelected = func(id widget.TreeNodeID) {
	// 		u, err := a.fileTree.GetValue(id)
	// 		if err != nil {
	// 			dialog.ShowError(err, a.UI.MainWin)
	// 			return
	// 		}
	// 		i := a.findIndex(u.Path())
	// 		if i == -1 {
	// 			dialog.ShowError(fmt.Errorf("Bad index for "+u.Path()), a.UI.MainWin)
	// 			return
	// 		}
	// 		a.index = i
	// 		a.DisplayImage()
	// 	}
	// }
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
				), a.UI.MainWin)
			}),
		),
	)
	a.UI.MainWin.SetMainMenu(mainMenu)
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
