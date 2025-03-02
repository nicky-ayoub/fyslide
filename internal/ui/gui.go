package ui

import (
	"fmt"
	"image"
	"log"
	"os"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) DisplayImage() error {
	// decode and update the image + get image path
	//fmt.Printf("Displaying %s\n", a.images[a.index].Path)
	var err error
	file, err := os.Open(a.images[a.index].Path)
	if err != nil {
		return fmt.Errorf("unable to open image '%s' %v", a.images[a.index].Path, err)
	}
	image, name, err := image.Decode(file)
	if err != nil {
		fmt.Printf("Unable image.Decode(%q) of format %q\n", file.Name(), name)
		return fmt.Errorf("unable to decode image %v", err)
	}
	a.img.OriginalImage = image
	a.img.Path = file.Name()
	a.image.Image = a.img.OriginalImage
	a.image.Refresh()

	a.imgSize.SetText(fmt.Sprintf("Size: %d bytes", a.images[a.index].Size))
	a.imgLastMod.SetText(fmt.Sprintf("Last modified: \n%s", a.images[a.index].ModTime.Format("02-01-2006")))
	w := fmt.Sprintf("Width:   %dpx", a.img.OriginalImage.Bounds().Max.X)
	h := fmt.Sprintf("Height: %dpx", a.img.OriginalImage.Bounds().Max.Y)
	c := fmt.Sprintf("Count: %d", a.ImageCount())
	a.widthLabel.SetText(w)
	a.heightLabel.SetText(h)
	a.countLabel.SetText(c)

	a.MainWin.SetTitle(fmt.Sprintf("FySlide - %v", (strings.Split(a.img.Path, "/")[len(strings.Split(a.img.Path, "/"))-1])))
	a.statusLabel.SetText(fmt.Sprintf("Image %s, %d of %d", a.img.Path, a.index+1, len(a.images)))

	a.leftArrow.Enable()
	a.rightArrow.Enable()
	a.first.Enable()
	a.last.Enable()
	return nil
}

func (a *App) firstImage() {
	a.index = 0
	a.DisplayImage()
}

func (a *App) lastImage() {
	a.index = len(a.images) - 1
	a.DisplayImage()
}

func (a *App) nextImage(dir int) {
	a.index += dir
	if a.index < 0 {
		a.index = 0
	} else if a.index >= len(a.images) {
		a.index = len(a.images) - 1
	}
	a.DisplayImage()
}

func (a *App) tagFile() {
	dialog.ShowCustom("TAGGER", "Ok", container.NewVBox(
		widget.NewLabel("Add image tag."),
		widget.NewHyperlink("Help and more information on Github", parseURL("https://github.com/nicky-ayoub/fyslide")),
		widget.NewLabel("v1.2 | License: MIT"),
	), a.MainWin)
}
func (a *App) deleteFileCheck() {
	dialog.ShowConfirm("Delete file!", "Are you sure?\n This action can't be undone.", func(b bool) {
		if b {
			a.deleteFile()
		}
	}, a.MainWin)
}
func (a *App) buildSatusBar() *fyne.Container {
	a.first = widget.NewButtonWithIcon("", theme.MediaFastRewindIcon(), func() { a.firstImage() })
	a.leftArrow = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { a.nextImage(-1) })
	a.rightArrow = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { a.nextImage(1) })
	a.last = widget.NewButtonWithIcon("", theme.MediaFastForwardIcon(), func() { a.lastImage() })
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
			a.rightArrow,
			a.last,
			a.deleteBtn,
			a.tagBtn,
			a.statusLabel,
			layout.NewSpacer(),
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

		widget.NewToolbarAction(theme.MediaFastRewindIcon(), func() { a.firstImage() }),
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() { a.nextImage(-1) }),
		widget.NewToolbarAction(theme.NavigateNextIcon(), func() { a.nextImage(1) }),
		widget.NewToolbarAction(theme.MediaFastForwardIcon(), func() { a.lastImage() }),
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
	toolbar := a.buildToolbar()
	status := a.buildSatusBar()

	a.fileTree = binding.NewURITree()
	files := widget.NewTreeWithData(a.fileTree, func(branch bool) fyne.CanvasObject {
		return widget.NewLabel("filename.ext")
	}, func(data binding.DataItem, branch bool, obj fyne.CanvasObject) {
		l := obj.(*widget.Label)
		u, _ := data.(binding.URI).Get()
		name := u.Name()
		l.SetText(name)
	})

	explorer := widget.NewAccordion(
		widget.NewAccordionItem("Files", files),
		widget.NewAccordionItem("Data", widget.NewLabel("data")),
	)

	// main menu
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("File"),

		fyne.NewMenu("Edit",
			fyne.NewMenuItem("Delete Image", a.deleteFileCheck),
			fyne.NewMenuItem("Keyboard Shortucts", a.showShortcuts),
		),
		fyne.NewMenu("View",
			fyne.NewMenuItem("Next Image", func() { a.nextImage(1) }),
			fyne.NewMenuItem("Previous Image", func() { a.nextImage(-1) }),
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
		toolbar,  // Top
		status,   //Bottom
		explorer, //left
		nil,      //right
		a.split,
	)
}

func (a *App) deleteFile() {
	if err := os.Remove(a.img.Path); err != nil {
		dialog.NewError(err, a.MainWin)
		return
	}
	if a.index == len(a.images)-1 {
		a.nextImage(-1)
	} else if len(a.images) == 1 {
		a.image.Image = nil
		a.img.EditedImage = nil
		a.img.OriginalImage = nil
		a.rightArrow.Disable()
		a.leftArrow.Disable()
		a.deleteBtn.Disable()
		a.image.Refresh()
	} else {
		a.nextImage(1)
	}
}
