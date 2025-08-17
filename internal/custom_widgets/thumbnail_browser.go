package custom_widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	// ThumbnailWidth is the width of the thumbnails in the browser.
	ThumbnailWidth = 100
	// ThumbnailHeight is the height of the thumbnails in the browser.
	ThumbnailHeight = 100
	// MaxVisibleThumbnails defines the maximum number of thumbnails to display in the strip.
	MaxVisibleThumbnails = 11
)

// ThumbnailHost defines the interface that the ThumbnailBrowser widget needs to
// communicate with its hosting application.
type ThumbnailHost interface {
	GetViewportItems(centerIndex, windowSize int) ([]ViewportItem, int)
	GetCurrentIndex() int
	GetThumbnail(path string, onComplete func(fyne.Resource)) fyne.Resource
	NavigateToImageIndex(index int)
	IsSlideshowPaused() bool
	ToggleSlideshow()
}

// ViewportItem is a local struct to avoid importing the ui package.
type ViewportItem struct {
	Path      string
	ViewIndex int
}

// ThumbnailBrowser is a custom widget for displaying a collapsible strip of thumbnails.
type ThumbnailBrowser struct {
	widget.BaseWidget
	host ThumbnailHost

	stripContainer *fyne.Container
	collapseButton *widget.Button
	sizedStrip     *fyne.Container
}

// NewThumbnailBrowser creates a new instance of the ThumbnailBrowser widget.
func NewThumbnailBrowser(host ThumbnailHost) *ThumbnailBrowser {
	tb := &ThumbnailBrowser{
		host: host,
	}
	tb.ExtendBaseWidget(tb)
	return tb
}

// CreateRenderer implements fyne.Widget.
func (tb *ThumbnailBrowser) CreateRenderer() fyne.WidgetRenderer {
	tb.stripContainer = container.NewHBox()

	stripSizer := canvas.NewRectangle(color.Transparent)
	stripSizer.SetMinSize(fyne.NewSize(0, ThumbnailHeight+10))
	tb.sizedStrip = container.NewStack(stripSizer, tb.stripContainer)

	tb.collapseButton = widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		if tb.sizedStrip.Visible() {
			tb.sizedStrip.Hide()
			tb.collapseButton.SetIcon(theme.MoveUpIcon())
		} else {
			tb.sizedStrip.Show()
			tb.collapseButton.SetIcon(theme.MoveDownIcon())
		}
	})

	content := container.NewBorder(nil, nil, nil, tb.collapseButton, tb.sizedStrip)
	return widget.NewSimpleRenderer(content)
}

// Refresh updates the content of the horizontal thumbnail strip.
func (tb *ThumbnailBrowser) Refresh() {
	if tb.stripContainer == nil {
		return
	}
	tb.stripContainer.RemoveAll()

	viewportItems, centerThumbIndex := tb.host.GetViewportItems(tb.host.GetCurrentIndex(), MaxVisibleThumbnails)

	if len(viewportItems) == 0 {
		tb.stripContainer.Refresh()
		return
	}

	tb.stripContainer.Add(layout.NewSpacer())

	for i, viewportItem := range viewportItems {
		path := viewportItem.Path
		viewIndex := viewportItem.ViewIndex

		// Create local variables for this specific iteration of the loop.
		// This is crucial because the updateThumb closure will capture these
		// variables. Without this, all closures would capture the same variables
		// from the last iteration of the loop.
		capturedPath := path
		tappableThumb := NewTappableImage(theme.FileImageIcon(), func() {
			if viewIndex == tb.host.GetCurrentIndex() {
				return // Do nothing if the current image's thumbnail is clicked
			}
			if !tb.host.IsSlideshowPaused() {
				tb.host.ToggleSlideshow()
			}
			tb.host.NavigateToImageIndex(viewIndex)
		})
		tappableThumb.SetMinSize(fyne.NewSize(ThumbnailWidth, ThumbnailHeight))

		thumbWidget := container.NewStack(tappableThumb)

		updateThumb := func(resource fyne.Resource) {
			tappableThumb.SetResource(resource)
			// Refresh the parent container to ensure the new image is drawn.
			thumbWidget.Refresh()
		}
		initialResource := tb.host.GetThumbnail(capturedPath, updateThumb)
		tappableThumb.SetResource(initialResource)

		if i == centerThumbIndex {
			border := canvas.NewRectangle(color.Transparent)
			border.StrokeColor = theme.PrimaryColor()
			border.StrokeWidth = 3
			thumbWidget.Add(border)
		}
		tb.stripContainer.Add(thumbWidget)
	}

	tb.stripContainer.Add(layout.NewSpacer())
	tb.stripContainer.Refresh()
}
