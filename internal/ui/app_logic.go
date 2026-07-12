// Package ui contains the core application logic and event handlers.
package ui

import (
	"context"
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/tagging"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
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

// showDuplicateResultsDialog displays the duplicate groups detected for the current image list.
func (a *App) showDuplicateResultsDialog(groups []tagging.DuplicateGroup) {
	if len(groups) == 0 {
		dialog.ShowInformation("Duplicate Scan", "No duplicates were found.", a.UI.MainWin)
		return
	}

	items := make([]fyne.CanvasObject, 0, len(groups))
	for _, group := range groups {
		g := group
		label := widget.NewLabel(fmt.Sprintf("%s group: %d member(s)", g.MatchType, len(g.Members)))
		openBtn := widget.NewButton("Open in Viewer", func() {
			a.openDuplicateGroup(g)
		})
		mergeBtn := widget.NewButton("Merge", func() {
			a.mergeDuplicateGroup(g)
		})
		row := container.NewHBox(label, openBtn, mergeBtn)
		items = append(items, row)
	}

	content := container.NewVBox(items...)
	scroll := container.NewVScroll(content)
	dialog.ShowCustom("Duplicate Scan Results", "Close", container.NewBorder(nil, nil, nil, nil, scroll), a.UI.MainWin)
}

func (a *App) openDuplicateGroup(group tagging.DuplicateGroup) {
	if len(group.Members) == 0 {
		return
	}

	items := make(scan.FileItems, 0, len(group.Members))
	allImages := a.imageState.GetAllImages()
	pathSet := make(map[string]struct{}, len(group.Members))
	for _, member := range group.Members {
		pathSet[member] = struct{}{}
	}
	for _, item := range allImages {
		if _, ok := pathSet[item.Path]; ok {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		dialog.ShowInformation("Duplicate Group", "No matching images were found in the current image set.", a.UI.MainWin)
		return
	}

	a.imageState.ApplyFilter(items, fmt.Sprintf("duplicate:%s", filepath.Base(group.RepresentativePath)))
	a.imageState.SetIndex(0)
	a.AddLogMessage(fmt.Sprintf("Showing duplicate group for %s", filepath.Base(group.RepresentativePath)))
	a.LoadAndDisplayCurrentImage()
}

func (a *App) mergeDuplicateGroup(group tagging.DuplicateGroup) {
	a.showDuplicateMergeDialog(group)
}

func (a *App) showDuplicateMergeDialog(group tagging.DuplicateGroup) {
	if len(group.Members) == 0 {
		return
	}

	members := append([]string(nil), group.Members...)
	sort.Strings(members)

	selectedRepresentative := group.RepresentativePath
	if selectedRepresentative == "" || !containsString(members, selectedRepresentative) {
		selectedRepresentative = members[0]
	}

	previewLabel := widget.NewLabel("")
	previewLabel.Wrapping = fyne.TextWrapWord
	warningLabel := widget.NewLabel("")
	warningLabel.Wrapping = fyne.TextWrapWord

	refreshPreview := func(selected string) {
		selectedRepresentative = selected
		previewLabel.SetText(fmt.Sprintf("Representative file: %s", selectedRepresentative))
		warnings := a.getDuplicateMergeWarnings(selectedRepresentative, members)
		if len(warnings) > 0 {
			warningLabel.SetText("Warning:\n" + strings.Join(warnings, "\n"))
		} else {
			warningLabel.SetText("All selected files are on the same filesystem, so hard links can be created safely.")
		}
	}

	radio := widget.NewRadioGroup(members, func(selected string) {
		refreshPreview(selected)
	})
	radio.SetSelected(selectedRepresentative)

	refreshPreview(selectedRepresentative)

	var dlg *dialog.CustomDialog
	mergeButton := widget.NewButton("Merge", func() {
		dlg.Hide()
		a.performMergeDuplicateGroup(group, selectedRepresentative)
	})
	cancelButton := widget.NewButton("Cancel", func() {
		dlg.Hide()
	})

	content := container.NewVBox(
		widget.NewLabel("Choose the file that should remain as the representative."),
		previewLabel,
		radio,
		widget.NewLabel("The other files will be hard-linked to the representative and their tags will be merged."),
		warningLabel,
		container.NewHBox(cancelButton, mergeButton),
	)
	contentContainer := container.NewVScroll(content)

	dlg = dialog.NewCustom("Merge duplicate group", "Cancel", contentContainer, a.UI.MainWin)
	dlg.Show()
}

func (a *App) performMergeDuplicateGroup(group tagging.DuplicateGroup, representativePath string) {
	if a.Service == nil {
		dialog.ShowError(fmt.Errorf("service is not initialized"), a.UI.MainWin)
		return
	}
	mergeGroup := group
	if representativePath != "" {
		mergeGroup.RepresentativePath = representativePath
	}
	result, err := a.Service.MergeDuplicateGroup(mergeGroup)
	if err != nil {
		a.AddLogMessage(fmt.Sprintf("Duplicate merge failed: %v", err))
		dialog.ShowError(err, a.UI.MainWin)
		return
	}
	message := fmt.Sprintf("Merged %d file(s) into %s", len(result.MergedPaths), filepath.Base(result.RepresentativePath))
	a.AddLogMessage(message)
	dialog.ShowInformation("Duplicate Merge", message, a.UI.MainWin)
}

func (a *App) getDuplicateMergeWarnings(representativePath string, members []string) []string {
	if representativePath == "" || len(members) == 0 {
		return nil
	}

	representativeInfo, err := os.Stat(representativePath)
	if err != nil {
		return []string{fmt.Sprintf("unable to inspect representative: %v", err)}
	}
	representativeStat, ok := representativeInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return []string{"unable to inspect filesystem metadata for the representative"}
	}

	warnings := make([]string, 0)
	for _, member := range members {
		if member == "" || member == representativePath {
			continue
		}
		memberInfo, err := os.Stat(member)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", filepath.Base(member), err))
			continue
		}
		memberStat, ok := memberInfo.Sys().(*syscall.Stat_t)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("%s: unable to inspect filesystem metadata", filepath.Base(member)))
			continue
		}
		if representativeStat.Dev != memberStat.Dev {
			warnings = append(warnings, fmt.Sprintf("%s is on a different filesystem and cannot be hard-linked", filepath.Base(member)))
		}
	}
	return warnings
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// runDuplicateDetection scans the current image set and reports duplicate groups.
func (a *App) runDuplicateDetection() {
	if a.Service == nil {
		dialog.ShowError(fmt.Errorf("service is not initialized"), a.UI.MainWin)
		return
	}

	paths := make([]string, 0)
	for _, item := range a.imageState.GetAllImages() {
		if item.Path != "" {
			paths = append(paths, item.Path)
		}
	}
	if len(paths) == 0 {
		dialog.ShowInformation("Duplicate Scan", "No images are currently loaded.", a.UI.MainWin)
		return
	}

	a.AddLogMessage(fmt.Sprintf("Scanning %d images for duplicates...", len(paths)))
	go func() {
		groups, err := a.Service.FindDuplicates(context.Background(), paths)
		if err != nil {
			fyne.Do(func() {
				a.AddLogMessage(fmt.Sprintf("Duplicate scan failed: %v", err))
				dialog.ShowError(err, a.UI.MainWin)
			})
			return
		}
		fyne.Do(func() {
			a.AddLogMessage(fmt.Sprintf("Duplicate scan complete: %d group(s) found.", len(groups)))
			a.showDuplicateResultsDialog(groups)
		})
	}()
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
