package ui

import (
	"image"
	"image/color"
	"math"

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

// ScaleAlgorithmType defines the type of scaling algorithm.
type ScaleAlgorithmType int

const (
	// NearestNeighbor uses the nearest pixel, fast but can be blocky.
	NearestNeighbor ScaleAlgorithmType = iota
	// Bilinear uses linear interpolation of four nearest pixels, smoother.
	Bilinear
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

	OnInteraction    func() // Callback for when user interacts (scrolls, drags) - e.g., to pause slideshow
	onZoomPanChange  func() // Callback for when zoom or pan changes - e.g., to update UI elements
	currentAlgorithm ScaleAlgorithmType
}

// NewZoomPanArea creates a new ZoomPanArea widget.
// The onInteraction func will be called when the user zooms or starts panning.
func NewZoomPanArea(img image.Image, onInteraction func()) *ZoomPanArea {
	zpa := &ZoomPanArea{
		originalImg:      img,
		zoomFactor:       1.0,
		panOffset:        fyne.Position{},
		minZoom:          defaultMinZoom,
		maxZoom:          defaultMaxZoom,
		OnInteraction:    onInteraction,
		currentAlgorithm: Bilinear, // Default to Bilinear for better quality
	}
	zpa.raster = canvas.NewRaster(zpa.draw)
	zpa.ExtendBaseWidget(zpa)
	if img != nil {
		zpa.Reset() // Center the initial image
	}
	return zpa
}

// SetScaleAlgorithm sets the scaling algorithm to be used.
func (zpa *ZoomPanArea) SetScaleAlgorithm(algo ScaleAlgorithmType) {
	if zpa.currentAlgorithm != algo {
		zpa.currentAlgorithm = algo
		zpa.Refresh() // Redraw with the new algorithm
	}
}

// GetScaleAlgorithm returns the currently selected scaling algorithm.
func (zpa *ZoomPanArea) GetScaleAlgorithm() ScaleAlgorithmType {
	return zpa.currentAlgorithm
}

// SetImage updates the image displayed by the widget.
func (zpa *ZoomPanArea) SetImage(img image.Image) {
	zpa.originalImg = img
	zpa.Reset() // Reset zoom/pan for the new image, this will also call onZoomPanChange
}

// SetOnZoomPanChange sets a callback function to be invoked when zoom or pan changes.
func (zpa *ZoomPanArea) SetOnZoomPanChange(callback func()) {
	zpa.onZoomPanChange = callback
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
	if zpa.onZoomPanChange != nil {
		zpa.onZoomPanChange()
	}
}

// ShowFullSize sets the zoom to 100% (1.0) and centers the image.
func (zpa *ZoomPanArea) ShowFullSize() {
	if zpa.originalImg == nil {
		return
	}
	zpa.zoomFactor = 1.0

	imgBounds := zpa.originalImg.Bounds()
	imgW := float32(imgBounds.Dx())
	imgH := float32(imgBounds.Dy())
	viewW := zpa.Size().Width
	viewH := zpa.Size().Height

	// Center the 100% zoomed image
	zpa.panOffset.X = (viewW - imgW) / 2
	zpa.panOffset.Y = (viewH - imgH) / 2

	zpa.Refresh()
	if zpa.onZoomPanChange != nil {
		zpa.onZoomPanChange()
	}
}

// IsOriginalLargerThanView returns true if the original image dimensions are greater than the current view size.
func (zpa *ZoomPanArea) IsOriginalLargerThanView() bool {
	if zpa.originalImg == nil || zpa.Size().Width == 0 || zpa.Size().Height == 0 {
		return false
	}
	imgBounds := zpa.originalImg.Bounds()
	return float32(imgBounds.Dx()) > zpa.Size().Width || float32(imgBounds.Dy()) > zpa.Size().Height
}

// clampInt ensures val is within min and max (inclusive).
func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// bilinearInterpolate calculates the color at a floating-point coordinate using bilinear interpolation.
// This provides a smoother appearance than nearest-neighbor when an image is scaled up or down.
// It works by taking the four nearest pixel colors (c00, c10, c01, c11) and blending them
// based on the fractional distance (tx, ty) of the target coordinate from the top-left
// pixel (x0, y0).
func (zpa *ZoomPanArea) bilinearInterpolate(x, y float32) color.Color {
	img := zpa.originalImg
	bounds := img.Bounds()
	maxX := bounds.Max.X - 1
	maxY := bounds.Max.Y - 1

	// Fallback for tiny images where interpolation isn't meaningful or might fail.
	if maxX < bounds.Min.X || maxY < bounds.Min.Y {
		return img.At(clampInt(int(x), bounds.Min.X, maxX), clampInt(int(y), bounds.Min.Y, maxY))
	}

	x0 := int(math.Floor(float64(x)))
	y0 := int(math.Floor(float64(y)))
	x1 := x0 + 1
	y1 := y0 + 1

	// Clamp coordinates to be within image bounds for sampling
	x0c := clampInt(x0, bounds.Min.X, maxX)
	y0c := clampInt(y0, bounds.Min.Y, maxY)
	x1c := clampInt(x1, bounds.Min.X, maxX)
	y1c := clampInt(y1, bounds.Min.Y, maxY)

	c00 := img.At(x0c, y0c) // Top-left
	c10 := img.At(x1c, y0c) // Top-right
	c01 := img.At(x0c, y1c) // Bottom-left
	c11 := img.At(x1c, y1c) // Bottom-right

	r00, g00, b00, a00 := c00.RGBA()
	r10, g10, b10, a10 := c10.RGBA()
	r01, g01, b01, a01 := c01.RGBA()
	r11, g11, b11, a11 := c11.RGBA()

	tx := x - float32(x0) // Fractional part for x
	ty := y - float32(y0) // Fractional part for y

	// Interpolate each channel
	finalR := uint16((float32(r00)*(1-tx)+float32(r10)*tx)*(1-ty) + (float32(r01)*(1-tx)+float32(r11)*tx)*ty)
	finalG := uint16((float32(g00)*(1-tx)+float32(g10)*tx)*(1-ty) + (float32(g01)*(1-tx)+float32(g11)*tx)*ty)
	finalB := uint16((float32(b00)*(1-tx)+float32(b10)*tx)*(1-ty) + (float32(b01)*(1-tx)+float32(b11)*tx)*ty)
	finalA := uint16((float32(a00)*(1-tx)+float32(a10)*tx)*(1-ty) + (float32(a01)*(1-tx)+float32(a11)*tx)*ty)

	return color.RGBA64{R: finalR, G: finalG, B: finalB, A: finalA}
}

// draw is the rendering function for the canvas.Raster.
// It's called by Fyne whenever the widget needs to be redrawn.
// For each pixel (dx, dy) in the destination view, it calculates the corresponding
// source pixel (sx, sy) in the original image based on the current zoom and pan,
// then sets the destination pixel's color.
func (zpa *ZoomPanArea) draw(w, h int) image.Image {
	if zpa.originalImg == nil || w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, w, h)) // Return empty/transparent
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	srcBounds := zpa.originalImg.Bounds()

	invZoomFactor := float32(1.0) / zpa.zoomFactor

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			// Calculate the corresponding source pixel coordinates in the original image.
			sx := (float32(dx) - zpa.panOffset.X) * invZoomFactor
			sy := (float32(dy) - zpa.panOffset.Y) * invZoomFactor

			// Check if the source point is within the original image bounds
			if sx >= float32(srcBounds.Min.X) && sx < float32(srcBounds.Max.X) &&
				sy >= float32(srcBounds.Min.Y) && sy < float32(srcBounds.Max.Y) {

				switch zpa.currentAlgorithm {
				case Bilinear:
					dst.Set(dx, dy, zpa.bilinearInterpolate(sx, sy))
				case NearestNeighbor:
					fallthrough
				default: // Default to NearestNeighbor
					// int(sx), int(sy) effectively performs nearest neighbor
					dst.Set(dx, dy, zpa.originalImg.At(int(sx), int(sy)))
				}
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

	// --- Zoom-towards-point logic ---
	// 1. Identify the point to zoom towards (center of the view is a stable choice).
	viewWidth, viewHeight := zpa.Size().Width, zpa.Size().Height
	// Use event position if available and reliable, otherwise center of view
	// Fyne's ScrollEvent.Position is often (0,0), so centering is safer.
	mouseX, mouseY := viewWidth/2, viewHeight/2 // Zoom towards center

	// 2. Calculate which point in the *original image* corresponds to our zoom point.
	// This is the "anchor" point that should remain stationary relative to the view.
	imgSpaceX := (mouseX - zpa.panOffset.X) / zpa.zoomFactor
	imgSpaceY := (mouseY - zpa.panOffset.Y) / zpa.zoomFactor

	// 3. Apply the new zoom factor.
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

	// 4. Adjust the pan offset. The new pan must be set so that the anchor point
	// (imgSpaceX, imgSpaceY) is still under the same view point (mouseX, mouseY) after zooming.
	zpa.panOffset.X = mouseX - (imgSpaceX * zpa.zoomFactor)
	zpa.panOffset.Y = mouseY - (imgSpaceY * zpa.zoomFactor)

	zpa.Refresh()
	if zpa.onZoomPanChange != nil {
		zpa.onZoomPanChange()
	}
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
	if zpa.onZoomPanChange != nil {
		zpa.onZoomPanChange()
	}
}

// DragEnd finalizes panning.
func (zpa *ZoomPanArea) DragEnd() {
	zpa.isPanning = false
}

// CurrentZoom returns the current zoom factor.
func (zpa *ZoomPanArea) CurrentZoom() float32 {
	return zpa.zoomFactor
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
