package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// --- Tappable Image Custom Widget ---

// tappableImage is a custom widget that displays an image and handles tap events.
type tappableImage struct {
	widget.BaseWidget
	image    *canvas.Image
	onTapped func()
}

// newTappableImage creates a new tappableImage widget.
func newTappableImage(res fyne.Resource, onTapped func()) *tappableImage {
	ti := &tappableImage{
		image:    canvas.NewImageFromResource(res),
		onTapped: onTapped,
	}
	ti.image.FillMode = canvas.ImageFillContain
	ti.ExtendBaseWidget(ti) // Important: call this to register the widget
	return ti
}

// CreateRenderer is a mandatory method for a Fyne widget.
func (t *tappableImage) CreateRenderer() fyne.WidgetRenderer {
	// We just need to render the image, so it has no background of its own.
	return widget.NewSimpleRenderer(t.image)
}

// Tapped is called when the widget is tapped.
func (t *tappableImage) Tapped(_ *fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

// SetResource updates the image resource and refreshes.
func (t *tappableImage) SetResource(res fyne.Resource) {
	t.image.Resource = res
	canvas.Refresh(t.image)
}

// SetMinSize sets the minimum size of the tappable image.
func (t *tappableImage) SetMinSize(size fyne.Size) {
	t.image.SetMinSize(size)
}
