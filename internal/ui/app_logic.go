// package ui contains the core application logic and event handlers.
package ui

import (
	"fmt"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// LoadAndDisplayCurrentImage loads the image at the current index in the active list
// in a background goroutine and updates the UI on the main Fyne thread.
func (a *App) LoadAndDisplayCurrentImage() {
	count := a.imageState.GetCurrentImageCount()
	// Handle empty list (either full or filtered)

	if count == 0 { // Handle empty list (either full or filtered)
		a.zoomPanArea.SetImage(nil)
		a.img = Img{EXIFData: make(map[string]string)} // Clear EXIF
		a.UI.MainWin.SetTitle("FySlide")
		a.updateStatusBar()
		a.UpdateInfoText(nil)
		a.AddLogMessage("No images available.")
		return // Exit the function, no image to load
	}

	imagePath := a.GetImageFullPath() // Get the full path of the current image

	// Check index bounds again after potential random selection or if not random
	if a.imageState.GetCurrentIndex() < 0 || a.imageState.GetCurrentIndex() >= count { // Use current count
		// This might happen if images were deleted; try to reset index or handle error
		a.imageState.SetIndex(0)                      // Reset to first image
		if a.imageState.GetCurrentImageCount() == 0 { // Double check after reset attempt
			fyne.Do(func() {
				a.zoomPanArea.SetImage(nil)                    // Clear the image display
				a.img = Img{EXIFData: make(map[string]string)} // Clear EXIF
				a.UI.MainWin.SetTitle("FySlide")
				a.updateStatusBar()
				a.UpdateInfoText(nil)
				a.AddLogMessage("No images available after index reset.")
			})
			return
		}
		// If count > 0 after reset, update imagePath as index changed
		imagePath = a.GetImageFullPath()
	}

	// Launch goroutine for loading and decoding
	go func(path string) {
		// Load all image info at once, including the decoded image
		imgInfo, imgDecoded, err := a.ImageService.GetImageInfo(path)
		if err != nil {
			fyne.Do(func() {
				a.handleImageDisplayError(path, "loading/decoding", err, "") // formatName not directly available here
			})
			return
		}

		// Successfully decoded image - perform UI updates on the Fyne thread
		fyne.Do(func() {
			a.img = Img{
				OriginalImage: imgDecoded,
				Path:          path,
				EXIFData:      imgInfo.EXIFData,
			}
			a.zoomPanArea.SetImage(a.img.OriginalImage) // This will also call Reset and Refresh

			// Update Title, Status Bar, and Info Text (pass the loaded imgInfo)
			a.updateStatusBar()
			a.UpdateInfoText(imgInfo)
			a.UI.thumbnailBrowser.Refresh() // Update the thumbnail strip
		})
	}(imagePath) // Pass the path and flag to the goroutine
}

// handleShowFullSizeBtn is called when the "Show Full Size" toolbar action is triggered.
func (a *App) handleShowFullSizeBtn() {
	if a.zoomPanArea != nil {
		a.slideshowManager.Pause(true) // Pause slideshow when user interacts with zoom
		a.zoomPanArea.ShowFullSize()
	}
}

// deleteFileCheck shows a confirmation dialog before deleting a file.
func (a *App) deleteFileCheck() {
	dialog.ShowConfirm("Delete file!", "Are you sure?\n This action can't be undone.", func(b bool) {
		if b {
			a.deleteFile()
		}
	}, a.UI.MainWin)
}

// deleteFile performs the actual file deletion and UI update.
func (a *App) deleteFile() {
	deletedPath := a.img.Path
	if deletedPath == "" {
		return
	} // No image loaded

	err := a.Service.DeleteImageFile(deletedPath)
	if err != nil {
		a.AddLogMessage(fmt.Sprintf("Error deleting file and tags: %v", err))
		dialog.ShowError(err, a.UI.MainWin)
		return
	}

	a.imageState.RemoveImage(deletedPath)
	a.AddLogMessage(fmt.Sprintf("Removed %s from image list.", filepath.Base(deletedPath)))

	if a.imageState.IsFiltered() && a.imageState.GetCurrentImageCount() == 0 {
		a.AddLogMessage("Filtered list empty after deletion, clearing filter.")
		a.Tagging.clearFilter()
		return
	}

	a.LoadAndDisplayCurrentImage()
}

// loadImages scans the given root directory for image files in a background goroutine
// and populates the main image list.
func (a *App) loadImages(root string) {
	a.imageState.images = nil // Clear previous images or a.images = a.images[:0]

	imageChan := a.Service.FileScan.Run(root, a.AddLogMessage)
	for item := range imageChan {
		a.imageState.images = append(a.imageState.images, item)
	}
	msg := fmt.Sprintf("Loaded %d images from %s", a.imageState.GetCurrentImageCount(), root)
	a.AddLogMessage(msg)
	fyne.Do(func() { a.UI.thumbnailBrowser.Refresh() }) // Refresh UI on main thread
}

// updateTimer updates the clock in the UI.
func (a *App) updateTimer() {
	for range time.Tick(time.Second) {
		if a.UI.MainWin == nil || a.UI.clockLabel == nil { // Check if UI elements are still valid
			return // Exit goroutine if window is closed
		}
		formatted := time.Now().Format("Time: 03:04:05")
		fyne.Do(func() { a.UI.clockLabel.SetText(formatted) })
	}
}

// slideshowAdvancer advances the slideshow based on a ticker.
func (a *App) slideshowAdvancer(ticker *time.Ticker) {
	for range ticker.C {
		if a.UI.MainWin == nil { // Check if window is still valid
			ticker.Stop() // Stop the ticker
			return        // Exit goroutine
		}
		if !a.slideshowManager.IsPaused() {
			fyne.Do(func() {
				a.Navigation.Navigate(1)
			})
		}
	}
}
