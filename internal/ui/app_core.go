// package ui contains the core application state and host interface implementations.
package ui

import (
	"fyslide/internal/custom_widgets"
	"fyslide/internal/service"
	"fyslide/internal/slideshow"
	"fyslide/internal/tagging"
	"image"

	"fyne.io/fyne/v2"
)

// Img struct holds data for the currently displayed image.
type Img struct {
	OriginalImage image.Image
	Path          string
	Directory     string
	EXIFData      map[string]string // To store selected EXIF fields
}

// App represents the whole application with all its windows, widgets and functions
type App struct {
	app fyne.App
	UI  UI

	Navigation *NavigationController
	imageState *ImageState
	Tagging    *TaggingController

	img         Img
	zoomPanArea *ZoomPanArea

	slideshowManager *slideshow.SlideshowManager

	tagDB *tagging.TagDB

	isDarkTheme bool

	// RefreshTagsFunc holds a callback to refresh the tags view.
	RefreshTagsFunc func()

	logBuffer        []string
	skipCount        int
	maxLogMessages   int
	logUIManager     *LogUIManager
	Service          *service.Service
	thumbnailManager *ThumbnailManager
	scanCompleteChan chan bool
	ImageService     *service.ImageService
}

// --- Host Interface Implementations ---

func (a *App) GetImageService() *service.ImageService {
	return a.ImageService
}

func (a *App) GetMainWindow() fyne.Window {
	return a.UI.MainWin
}

func (a *App) RefreshTags() {
	if a.RefreshTagsFunc != nil {
		a.RefreshTagsFunc()
	}
}

func (a *App) NavigateToIndex(index int) {
	if a.Navigation != nil {
		a.Navigation.NavigateToIndex(index)
	}
}

func (a *App) GetImageFullPath() string {
	item := a.imageState.GetCurrentItem()
	if item == nil {
		return ""
	}
	return item.Path
}

func (a *App) GetSlideshowManager() *slideshow.SlideshowManager {
	return a.slideshowManager
}

func (a *App) GetViewportItems(centerIndex, windowSize int) ([]custom_widgets.ViewportItem, int) {
	items, newCenter := a.imageState.GetViewportItems(centerIndex, windowSize)
	// Convert ui.ViewportItem to custom_widgets.ViewportItem
	customItems := make([]custom_widgets.ViewportItem, len(items))
	for i, item := range items {
		customItems[i] = custom_widgets.ViewportItem{
			Path:      item.Item.Path,
			ViewIndex: item.ViewIndex,
		}
	}
	return customItems, newCenter
}

func (a *App) GetCurrentIndex() int {
	return a.imageState.GetCurrentIndex()
}

func (a *App) GetThumbnail(path string, onComplete func(fyne.Resource)) fyne.Resource {
	return a.thumbnailManager.GetThumbnail(path, onComplete)
}

func (a *App) NavigateToImageIndex(index int) {
	a.Navigation.NavigateToImageIndex(index)
}

func (a *App) ListAllTags() ([]tagging.TagWithCount, error) {
	return a.Service.ListAllTags()
}

func (a *App) GetWindow() fyne.Window {
	return a.UI.MainWin
}

func (a *App) ApplyFilter(tags []string) {
	if a.Tagging != nil {
		a.Tagging.ApplyFilter(tags)
	}
}

func (a *App) RemoveTagGlobally(tag string) error {
	return a.Tagging.RemoveTagGlobally(tag)
}

func (a *App) IsSlideshowPaused() bool {
	if a.slideshowManager == nil {
		return true
	}
	return a.slideshowManager.IsPaused()
}

func (a *App) ToggleSlideshow() {
	a.TogglePlay()
}
