package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// About represents the about dialog
type About struct {
	title     string
	parent    *fyne.Window
	container *fyne.Container
	d         dialog.Dialog
}

// NewAbout creates a new about dialog
func NewAbout(parent *fyne.Window, title string, image fyne.Resource) *About {
	a := &About{
		title:  title,
		parent: parent,
	}

	img := canvas.NewImageFromResource(image)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(200, 200))

	vbox := container.NewVBox(img)

	ok := container.NewHBox(
		layout.NewSpacer(),
		widget.NewButton("OK", func() { a.Hide() }),
		layout.NewSpacer(),
	)

	a.container = container.NewBorder(nil, ok, nil, nil, vbox)

	return a
}

// Hide hides the dialog
func (a *About) Hide() {
	a.d.Hide()
}

// Show shows the dialog
func (a *About) Show() {
	a.d = dialog.NewCustomWithoutButtons(a.title, a.container, *a.parent)
	a.d.Show()
}
