package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	defaultMinZoom        float32 = 0.1  // Example: 10% zoom
	defaultMaxZoom        float32 = 10.0 // Example: 1000% zoom
	defaultZoomScrollStep float32 = 0.1  // Zoom step for scroll events
)

// ZoomPanArea is a custom widget for displaying an image with zoom and pan.
type ZoomPanArea struct {
	widget.BaseWidget

	originalImg image.Image    // Store the original image
	raster      *canvas.Raster // Use Raster for custom drawing

	zoomFactor float32
	panOffset  fyne.Position

	minZoom float32
	maxZoom float32

	isPanning    bool
	lastMousePos fyne.Position

	OnInteraction func() // Callback for when user interacts (scrolls, drags)
}

// NewZoomPanArea creates a new ZoomPanArea widget.
// The onInteraction func will be called when the user zooms or starts panning.
func NewZoomPanArea(img image.Image, onInteraction func()) *ZoomPanArea {
	zpa := &ZoomPanArea{
		originalImg:   img,
		zoomFactor:    1.0,
		panOffset:     fyne.Position{},
		minZoom:       defaultMinZoom,
		maxZoom:       defaultMaxZoom,
		OnInteraction: onInteraction,
	}
	zpa.raster = canvas.NewRaster(zpa.draw)
	zpa.ExtendBaseWidget(zpa)
	if img != nil {
		zpa.Reset() // Center the initial image
	}
	return zpa
}

// SetImage updates the image displayed by the widget.
func (zpa *ZoomPanArea) SetImage(img image.Image) {
	zpa.originalImg = img
	zpa.Reset() // Reset zoom/pan for the new image
}

// Reset centers the image and resets zoom to 1.0 or a fit-to-view.
func (zpa *ZoomPanArea) Reset() {
	zpa.panOffset = fyne.Position{} // Reset pan first

	if zpa.originalImg != nil && zpa.Size().Width > 0 && zpa.Size().Height > 0 {
		imgBounds := zpa.originalImg.Bounds()
		imgW := float32(imgBounds.Dx())
		imgH := float32(imgBounds.Dy())
		viewW := zpa.Size().Width
		viewH := zpa.Size().Height

		// Calculate zoom factor to fit the image within the view
		// while maintaining aspect ratio.
		zoomW := viewW / imgW
		zoomH := viewH / imgH

		// Use the smaller zoom factor to ensure the whole image fits
		zpa.zoomFactor = zoomW
		if zoomH < zoomW {
			zpa.zoomFactor = zoomH
		}

		// Center the scaled image
		scaledImgW := imgW * zpa.zoomFactor
		scaledImgH := imgH * zpa.zoomFactor
		zpa.panOffset.X = (viewW - scaledImgW) / 2
		zpa.panOffset.Y = (viewH - scaledImgH) / 2
	} else {
		// Default if no image or size is not ready (e.g. initial load before layout)
		zpa.zoomFactor = 1.0
	}
	zpa.Refresh()
}

// draw is the rendering function for the canvas.Raster.
func (zpa *ZoomPanArea) draw(w, h int) image.Image {
	if zpa.originalImg == nil || w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, w, h)) // Return empty/transparent
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	srcBounds := zpa.originalImg.Bounds()

	// Pre-calculate inverse zoom factor to avoid division in the loop.
	// zpa.minZoom should prevent zpa.zoomFactor from being zero.
	invZoomFactor := float32(1.0) / zpa.zoomFactor

	// Transform defines how to map destination pixels to source pixels
	// For each pixel (dx, dy) in dst, find corresponding (sx, sy) in src
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			// Screen point (dx, dy) to image point (sx, sy)
			// Inverse of pan, then inverse of zoom
			sx := (float32(dx) - zpa.panOffset.X) * invZoomFactor
			sy := (float32(dy) - zpa.panOffset.Y) * invZoomFactor

			// Check if the source point is within the original image bounds
			if sx >= float32(srcBounds.Min.X) && sx < float32(srcBounds.Max.X) &&
				sy >= float32(srcBounds.Min.Y) && sy < float32(srcBounds.Max.Y) {
				dst.Set(dx, dy, zpa.originalImg.At(int(sx), int(sy)))
			}
		}
	}
	return dst
}

// CreateRenderer is a Fyne lifecycle method.
func (zpa *ZoomPanArea) CreateRenderer() fyne.WidgetRenderer {
	return &zoomPanAreaRenderer{zpa: zpa}
}

// Scrolled handles mouse wheel events for zooming.
func (zpa *ZoomPanArea) Scrolled(ev *fyne.ScrollEvent) {
	if zpa.OnInteraction != nil {
		zpa.OnInteraction()
	}

	viewWidth, viewHeight := zpa.Size().Width, zpa.Size().Height
	// Use event position if available and reliable, otherwise center of view
	// Fyne's ScrollEvent.Position is often (0,0), so centering is safer.
	mouseX, mouseY := viewWidth/2, viewHeight/2 // Zoom towards center

	// Point in image space that was under the mouse/center
	imgSpaceX := (mouseX - zpa.panOffset.X) / zpa.zoomFactor
	imgSpaceY := (mouseY - zpa.panOffset.Y) / zpa.zoomFactor

	// Apply zoom
	if ev.Scrolled.DY < 0 { // Scroll up/away from user (content moves down) -> zoom out
		zpa.zoomFactor /= (1.0 + defaultZoomScrollStep)
	} else if ev.Scrolled.DY > 0 { // Scroll down/towards user (content moves up) -> zoom in
		zpa.zoomFactor *= (1.0 + defaultZoomScrollStep)
	}

	if zpa.zoomFactor < zpa.minZoom {
		zpa.zoomFactor = zpa.minZoom
	}
	if zpa.zoomFactor > zpa.maxZoom {
		zpa.zoomFactor = zpa.maxZoom
	}

	// Adjust panOffset to keep the point (imgSpaceX, imgSpaceY) under the mouse/center
	zpa.panOffset.X = mouseX - (imgSpaceX * zpa.zoomFactor)
	zpa.panOffset.Y = mouseY - (imgSpaceY * zpa.zoomFactor)

	zpa.Refresh()
}

// MouseDown starts panning.
func (zpa *ZoomPanArea) MouseDown(ev *desktop.MouseEvent) {
	if zpa.OnInteraction != nil && ev.Button == desktop.MouseButtonPrimary {
		zpa.OnInteraction()
	}
	if ev.Button == desktop.MouseButtonPrimary { // Or check for a specific modifier if needed
		zpa.isPanning = true
		zpa.lastMousePos = ev.Position
	}
}

// MouseUp stops panning.
func (zpa *ZoomPanArea) MouseUp(_ *desktop.MouseEvent) {
	zpa.isPanning = false
}

// Dragged handles mouse drag for panning.
func (zpa *ZoomPanArea) Dragged(ev *fyne.DragEvent) {
	if !zpa.isPanning {
		return
	}
	delta := ev.Position.Subtract(zpa.lastMousePos)
	zpa.panOffset = zpa.panOffset.Add(delta)
	zpa.lastMousePos = ev.Position
	zpa.Refresh()
}

// DragEnd finalizes panning.
func (zpa *ZoomPanArea) DragEnd() {
	zpa.isPanning = false
}

// --- Renderer for ZoomPanArea ---
type zoomPanAreaRenderer struct{ zpa *ZoomPanArea }

func (r *zoomPanAreaRenderer) Layout(size fyne.Size)        { r.zpa.raster.Resize(size) }
func (r *zoomPanAreaRenderer) MinSize() fyne.Size           { return fyne.NewSize(100, 100) } // Basic min size
func (r *zoomPanAreaRenderer) Refresh()                     { canvas.Refresh(r.zpa.raster) }
func (r *zoomPanAreaRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.zpa.raster} }
func (r *zoomPanAreaRenderer) Destroy()                     {}

var _ fyne.Widget = (*ZoomPanArea)(nil)
var _ fyne.Scrollable = (*ZoomPanArea)(nil)
var _ fyne.Draggable = (*ZoomPanArea)(nil)
