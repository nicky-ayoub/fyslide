// Package ui contains the core application logic and event handlers.
package ui

import (
	"context"
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

	// If no images, clear UI and return.
	if count == 0 {
		if fyne.CurrentApp() != nil {
			fyne.Do(func() {
				if a.zoomPanArea != nil {
					a.zoomPanArea.SetImage(nil)
				}
				a.SetImg(Img{EXIFData: make(map[string]string)})
				if a.UI.MainWin != nil {
					a.UI.MainWin.SetTitle("FySlide")
				}
				a.updateStatusBar()
				a.UpdateInfoText(nil)
				a.AddLogMessage("No images available.")
			})
		} else {
			if a.zoomPanArea != nil {
				a.zoomPanArea.SetImage(nil)
			}
			a.SetImg(Img{EXIFData: make(map[string]string)})
			if a.UI.MainWin != nil {
				a.UI.MainWin.SetTitle("FySlide")
			}
			a.updateStatusBar()
			a.UpdateInfoText(nil)
			a.AddLogMessage("No images available.")
		}
		return
	}

	imagePath := a.GetImageFullPath()

	// Check index bounds again after potential random selection or if not random
	if a.imageState.GetCurrentIndex() < 0 || a.imageState.GetCurrentIndex() >= count {
		a.imageState.SetIndex(0)
		if a.imageState.GetCurrentImageCount() == 0 {
			if fyne.CurrentApp() != nil {
				fyne.Do(func() {
					if a.zoomPanArea != nil {
						a.zoomPanArea.SetImage(nil)
					}
					a.SetImg(Img{EXIFData: make(map[string]string)})
					if a.UI.MainWin != nil {
						a.UI.MainWin.SetTitle("FySlide")
					}
					a.updateStatusBar()
					a.UpdateInfoText(nil)
					a.AddLogMessage("No images available after index reset.")
				})
			} else {
				if a.zoomPanArea != nil {
					a.zoomPanArea.SetImage(nil)
				}
				a.img = Img{EXIFData: make(map[string]string)}
				if a.UI.MainWin != nil {
					a.UI.MainWin.SetTitle("FySlide")
				}
				a.updateStatusBar()
				a.UpdateInfoText(nil)
				a.AddLogMessage("No images available after index reset.")
			}
			return
		}
		imagePath = a.GetImageFullPath()
	}

	// Cancel any previous per-image load, then create a new cancellable context
	a.loadMu.Lock()
	if a.loadCancel != nil {
		a.loadCancel()
	}
	parent := a.appCtx
	if parent == nil {
		parent = context.Background()
	}
	loadCtx, loadCancel := context.WithCancel(parent)
	a.loadCancel = loadCancel
	a.loadMu.Unlock()

	// Launch goroutine for loading and decoding. The decode itself cannot be cancelled
	// because the standard library does not support cancellable image decode, but
	// we use the per-load context to avoid applying stale results and to signal intent.
	go func(ctx context.Context, path string) {
		// Load all image info at once, including the decoded image
		imgInfo, imgDecoded, err := a.ImageService.GetImageInfo(path) // This can be slow (disk I/O, decoding)

		// If the load was cancelled, drop the result
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err != nil {
			if fyne.CurrentApp() != nil {
				fyne.Do(func() {
					if a.GetImageFullPath() == path {
						a.handleImageDisplayError(path, "loading/decoding", err, "")
					}
				})
			} else {
				if a.GetImageFullPath() == path {
					a.handleImageDisplayError(path, "loading/decoding", err, "")
				}
			}
			return
		}

		// Successfully decoded image - perform UI updates on the Fyne thread
		if fyne.CurrentApp() != nil {
			fyne.Do(func() {
				// Final check: Has the user navigated away while this image was loading?
				if a.GetImageFullPath() != path {
					return // Discard stale image load.
				}

				newImg := Img{
					OriginalImage: imgDecoded,
					Path:          path,
					EXIFData:      imgInfo.EXIFData,
				}
				a.SetImg(newImg)
				if a.zoomPanArea != nil {
					a.zoomPanArea.SetImage(newImg.OriginalImage) // This will also call Reset and Refresh
				}

				// Update Title, Status Bar, and Info Text (pass the loaded imgInfo)
				a.updateStatusBar()
				if a.UI.infoText != nil && a.Service != nil {
					a.UpdateInfoText(imgInfo)
				}
				if a.UI.thumbnailBrowser != nil {
					a.UI.thumbnailBrowser.Refresh() // Update the thumbnail strip
				}
			})
		} else {
			// Non-Fyne test environment: apply synchronously but only if still current
			if a.GetImageFullPath() != path {
				return
			}
			newImg := Img{
				OriginalImage: imgDecoded,
				Path:          path,
				EXIFData:      imgInfo.EXIFData,
			}
			a.SetImg(newImg)
			if a.zoomPanArea != nil {
				a.zoomPanArea.SetImage(newImg.OriginalImage)
			}
			a.updateStatusBar()
			if a.UI.infoText != nil && a.Service != nil {
				a.UpdateInfoText(imgInfo)
			}
			if a.UI.thumbnailBrowser != nil {
				a.UI.thumbnailBrowser.Refresh()
			}
		}
	}(loadCtx, imagePath) // Pass the path and per-load ctx to the goroutine
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
	// Get the path and index of the image to be deleted.
	viewIndexToDelete := a.imageState.GetCurrentIndex()
	itemToDelete := a.imageState.GetCurrentItem()
	if viewIndexToDelete < 0 || itemToDelete == nil {
		return // No image is selected.
	}
	pathToDelete := itemToDelete.Path

	// First, attempt the destructive operation on the backend (disk and DB).
	err := a.Service.DeleteImageFile(pathToDelete)
	if err != nil {
		a.AddLogMessage(fmt.Sprintf("Error deleting file and tags: %v", err))
		dialog.ShowError(err, a.UI.MainWin)
		// Do not proceed, as the file was not deleted. The UI state remains consistent.
		return
	}

	// Deletion was successful, now update the UI state.
	_, listBecameEmpty := a.imageState.RemoveImageAtViewIndex(viewIndexToDelete)

	a.AddLogMessage(fmt.Sprintf("Deleted %s.", filepath.Base(pathToDelete)))

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
func (a *App) loadImagesFromDB(ctx context.Context) {
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
			for {
				select {
				case <-ctx.Done():
					return
				case path, ok := <-pathChan:
					if !ok {
						return
					}
					info, err := os.Stat(path)
					// If os.Stat fails, it's likely the file was moved or deleted, so we just ignore it.
					if err == nil && !info.IsDir() {
						select {
						case <-ctx.Done():
							return
						case resultsChan <- scan.NewFileItem(path, info):
						}
					}
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
func (a *App) loadImages(ctx context.Context, root string) {
	// Signal completion when this function exits, no matter how.
	defer func() {
		select {
		case a.scanCompleteChan <- true:
		default:
		}
	}()

	imageChan := a.Service.FileScan.Run(ctx, root, a.AddLogMessage)

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
func (a *App) updateTimer(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.UI.MainWin == nil || a.UI.clockLabel == nil {
				return
			}
			formatted := time.Now().Format("Time: 03:04:05")
			fyne.Do(func() { a.UI.clockLabel.SetText(formatted) })
		}
	}
}

// slideshowAdvancer advances the slideshow based on a ticker.
func (a *App) slideshowAdvancer(ctx context.Context, ticker *time.Ticker) {
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.UI.MainWin == nil {
				return
			}
			if !a.slideshowManager.IsPaused() {
				fyne.Do(func() {
					a.Navigation.Navigate(1)
				})
			}
		}
	}
}
