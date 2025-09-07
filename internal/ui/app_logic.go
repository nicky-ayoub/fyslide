// Package ui contains the core application logic and event handlers.
package ui

import (
	"fmt"
	"fyslide/internal/scan"
	"os"
	"path/filepath"
	"runtime"
	"sync"
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
		imgInfo, imgDecoded, err := a.ImageService.GetImageInfo(path) // This can be slow (disk I/O, decoding)

		if err != nil {
			fyne.Do(func() {
				a.handleImageDisplayError(path, "loading/decoding", err, "")
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
	// Get the index of the image we are about to delete.
	viewIndexToDelete := a.imageState.GetCurrentIndex()
	if viewIndexToDelete < 0 {
		return // No image is selected.
	}

	// Remove the image from the state first to get its path.
	deletedPath, listBecameEmpty := a.imageState.RemoveImageAtViewIndex(viewIndexToDelete)
	if deletedPath == "" {
		a.AddLogMessage("Could not delete image: not found in current view.")
		return // The image wasn't in the state, so nothing to do.
	}

	// Now, delete the file from disk and the database.
	err := a.Service.DeleteImageFile(deletedPath)
	if err != nil {
		a.AddLogMessage(fmt.Sprintf("Error deleting file and tags: %v", err))
		dialog.ShowError(err, a.UI.MainWin)
		// Note: The file is already removed from the UI state, so we just log the error.
		return
	}

	a.AddLogMessage(fmt.Sprintf("Deleted %s.", filepath.Base(deletedPath)))

	// If the list became empty and was filtered, clear the filter.
	if listBecameEmpty && a.imageState.IsFiltered() {
		a.AddLogMessage("Filtered list empty after deletion, clearing filter.")
		a.Tagging.clearFilter()
		return
	}

	a.LoadAndDisplayCurrentImage()
}

// loadImagesFromDB pre-populates the image list from the tag database.
// This version is optimized for large databases by using a worker pool
// to check for file existence concurrently.
func (a *App) loadImagesFromDB() {
	a.AddLogMessage("Pre-loading known image paths from database...")

	pathChan := a.Service.StreamAllImagePaths()

	// --- Worker Pool Setup ---
	// Use a number of workers based on CPU cores for I/O-bound tasks.
	// This provides a good balance without overwhelming the system.
	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup
	resultsChan := make(chan scan.FileItem, 100)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathChan {
				info, err := os.Stat(path)
				// If os.Stat fails, it's likely the file was moved or deleted, so we just ignore it.
				if err == nil && !info.IsDir() {
					resultsChan <- scan.NewFileItem(path, info)
				}
			}
		}()
	}

	// Goroutine to close the results channel once all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// --- Process Results ---
	const batchSize = 1000
	const batchTimeout = 100 * time.Millisecond

	batch := make(scan.FileItems, 0, batchSize)
	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	totalAdded := 0
	running := true
	for running {
		select {
		case item, ok := <-resultsChan:
			if !ok { // Channel is closed, all files processed.
				if len(batch) > 0 {
					a.imageState.AddImages(batch)
					totalAdded += len(batch)
				}
				running = false // Exit the loop
				break
			}
			batch = append(batch, item)
			if len(batch) >= batchSize {
				a.imageState.AddImages(batch)
				totalAdded += len(batch)
				batch = make(scan.FileItems, 0, batchSize)
			}
		case <-ticker.C:
			// On a timer, add whatever is in the batch to update the UI count.
			if len(batch) > 0 {
				a.imageState.AddImages(batch)
				totalAdded += len(batch)
				batch = make(scan.FileItems, 0, batchSize)
			}
		}
	}

	if totalAdded == 0 {
		a.AddLogMessage("No previously tagged images found on disk.")
	} else {
		a.AddLogMessage(fmt.Sprintf("Pre-loaded %d existing images from database.", totalAdded))
	}
}

// loadImages scans the given root directory for image files in a background goroutine
// and populates the main image list.
func (a *App) loadImages(root string) {
	// Signal completion when this function exits, no matter how.
	defer func() {
		select {
		case a.scanCompleteChan <- true:
		default:
		}
	}()

	imageChan := a.Service.FileScan.Run(root, a.AddLogMessage)

	const batchSize = 1000
	const batchTimeout = 100 * time.Millisecond

	batch := make(scan.FileItems, 0, batchSize)
	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	// Loop to process images from the channel
	for {
		select {
		case item, ok := <-imageChan:
			if !ok { // Channel is closed, scanner is done.
				// Add any remaining items in the final batch.
				if len(batch) > 0 {
					a.imageState.AddImages(batch)
				}
				// Finalize and exit the function.
				msg := fmt.Sprintf("Loaded %d images from %s", a.imageState.GetCurrentImageCount(), root)
				a.AddLogMessage(msg)
				fyne.Do(func() { a.UI.thumbnailBrowser.Refresh() })
				return // Exit the function, defer will signal completion.
			}

			batch = append(batch, item)
			if len(batch) >= batchSize {
				a.imageState.AddImages(batch)
				batch = make(scan.FileItems, 0, batchSize) // Reset batch, keeping capacity.
			}
		case <-ticker.C:
			// Timeout reached, add whatever is in the batch to update the UI count.
			if len(batch) > 0 {
				a.imageState.AddImages(batch)
				batch = make(scan.FileItems, 0, batchSize)
			}
		}
	}
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
