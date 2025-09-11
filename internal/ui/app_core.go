// Package ui contains the core application state and host interface implementations.
package ui

import (
	"fyslide/internal/customwidgets"
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

	slideshowManager *slideshow.Manager

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

// GetService returns the application's core service layer.
func (a *App) GetService() *service.Service {
	return a.Service
}

// GetImageService returns the application's image service.
func (a *App) GetImageService() *service.ImageService {
	return a.ImageService
}

// GetMainWindow returns the main application window.
func (a *App) GetMainWindow() fyne.Window {
	return a.UI.MainWin
}

// RefreshTags triggers a refresh of the tags view.
func (a *App) RefreshTags() {
	if a.RefreshTagsFunc != nil {
		a.RefreshTagsFunc()
	}
}

// NavigateToIndex navigates the view to a specific image index.
func (a *App) NavigateToIndex(index int) {
	if a.Navigation != nil {
		a.Navigation.NavigateToIndex(index)
	}
}

// GetImageFullPath returns the full path of the currently displayed image.
func (a *App) GetImageFullPath() string {
	item := a.imageState.GetCurrentItem()
	if item == nil {
		return ""
	}
	return item.Path
}

// GetSlideshowManager returns the application's slideshow manager.
func (a *App) GetSlideshowManager() *slideshow.Manager {
	return a.slideshowManager
}

// isImageViewActive checks if the main image view is currently visible and ready for interaction.
func (a *App) isImageViewActive() bool {
	if a.zoomPanArea == nil || a.UI.contentStack == nil {
		return false
	}
	// Check if the image view is the visible one in the stack
	return a.UI.contentStack.Objects[customwidgets.ImageViewIndex].Visible()
}

// GetViewportItems returns a slice of items for the thumbnail browser's viewport.
// It satisfies the ThumbnailHost interface.
func (a *App) GetViewportItems(centerIndex, windowSize int) ([]customwidgets.ViewportItem, int) {
	items, newCenter := a.imageState.GetViewportItems(centerIndex, windowSize)
	// Convert ui.ViewportItem to customwidgets.ViewportItem
	customItems := make([]customwidgets.ViewportItem, len(items))
	for i, item := range items {
		customItems[i] = customwidgets.ViewportItem{
			// Path is used by the thumbnail browser to request a thumbnail.
			Path: item.Item.Path,
			// ViewIndex is used to navigate to the image when the thumbnail is clicked.
			ViewIndex: item.ViewIndex,
		}
	}
	return customItems, newCenter
}

// GetCurrentIndex returns the index of the current image in the active view.
// It satisfies the ThumbnailHost interface.
func (a *App) GetCurrentIndex() int {
	return a.imageState.GetCurrentIndex()
}

// GetThumbnail retrieves or generates a thumbnail for the given image path.
// It satisfies the ThumbnailHost interface.
func (a *App) GetThumbnail(path string, onComplete func(fyne.Resource)) fyne.Resource {
	return a.thumbnailManager.GetThumbnail(path, onComplete)
}

// NavigateToImageIndex navigates directly to an image by its view index.
// It satisfies the ThumbnailHost interface.
func (a *App) NavigateToImageIndex(index int) {
	a.Navigation.NavigateToImageIndex(index)
}

// ListAllTags returns all unique tags from the database with their counts.
// It satisfies the TagsViewHost interface.
func (a *App) ListAllTags() ([]tagging.TagWithCount, error) {
	return a.Service.ListAllTags()
}

// GetWindow returns the main application window.
// It satisfies the TagsViewHost and NavigationHost interfaces.
func (a *App) GetWindow() fyne.Window {
	return a.UI.MainWin
}

// ApplyFilter applies a tag-based filter to the image list.
// It satisfies the TagsViewHost interface.
func (a *App) ApplyFilter(tags []string) {
	if a.Tagging != nil {
		a.Tagging.ApplyFilter(tags)
	}
}

// RemoveTagGlobally removes a tag from all images in the database.
// It satisfies the TagsViewHost interface.
func (a *App) RemoveTagGlobally(tag string) error {
	return a.Tagging.RemoveTagGlobally(tag)
}

// IsSlideshowPaused returns true if the slideshow is currently paused.
// It satisfies the ThumbnailHost interface.
func (a *App) IsSlideshowPaused() bool {
	if a.slideshowManager == nil {
		return true
	}
	return a.slideshowManager.IsPaused()
}

// ToggleSlideshow toggles the play/pause state of the slideshow.
// It satisfies the ThumbnailHost interface.
func (a *App) ToggleSlideshow() {
	a.TogglePlay()
}
