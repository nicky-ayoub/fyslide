// Package ui  Setup for the FySlide Application
package ui

import (
	"container/list"
	"flag"
	"fmt"
	"fyslide/internal/history"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/slideshow"
	"fyslide/internal/tagging"
	"image"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	//"fyne.io/fyne/v2/data/binding"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	// DefaultSkipCount is the default number of images to skip with PageUp/PageDown.
	DefaultSkipCount = 20
)

// Img struct
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

	//fileTree binding.URITree

	images         scan.FileItems // The original, full list of images
	filteredImages scan.FileItems // The list when a filter is active
	index          int
	img            Img
	zoomPanArea    *ZoomPanArea

	historyManager      *history.HistoryManager // Manages navigation history
	isNavigatingHistory bool                    // True if DisplayImage is called from a history action

	slideshowManager *slideshow.SlideshowManager // NEW: Use SlideshowManager
	thumbnailHistory *list.List                  // Cache for random mode thumbnail paths
	navigationQueue  *NavigationQueue

	random bool

	tagDB *tagging.TagDB // Add the tag database instance

	isFiltered       bool   // NEW: Flag to indicate if filtering is active
	currentFilterTag string // NEW: The tag currently being filtered by

	refreshTagsFunc func() // This will hold the function returned by buildTagsTab

	skipCount        int // NEW: Configurable skip count for PageUp/PageDown
	maxLogMessages   int // Maximum number of log messages to store, initialized from DefaultMaxLogMessages
	logUIManager     *LogUIManager
	Service          *service.Service
	thumbnailManager *ThumbnailManager
	ImageService     *service.ImageService
}

// getCurrentList returns the active image list (filtered or full)
func (a *App) getCurrentList() scan.FileItems {
	if a.isFiltered {
		return a.filteredImages
	}
	return a.images
}

// getCurrentImageCount returns the count of the active image list
func (a *App) getCurrentImageCount() int {
	return len(a.getCurrentList())
}

// ternaryString is a helper, assuming it's defined elsewhere or should be local.
// If it's the one from app.go, ensure it's accessible or duplicate if needed.
func ternaryString(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

// formatNumberWithCommas takes an integer and returns a string representation
// with commas as thousands separators.
func formatNumberWithCommas(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:] // Temporarily remove sign for processing
	}
	length := len(s)
	if length <= 3 {
		if n < 0 {
			return "-" + s
		}
		return s
	}
	// Calculate number of commas needed
	commas := (length - 1) / 3
	result := make([]byte, length+commas)
	for i, j, k := length-1, len(result)-1, 0; ; i, j = i-1, j-1 {
		result[j] = s[i]
		if i == 0 {
			if n < 0 {
				return "-" + string(result)
			}
			return string(result)
		}
		k++
		if k%3 == 0 {
			j--
			result[j] = ','
		}
	}
}

// getCurrentItem returns the FileItem for the current index, or nil if invalid
func (a *App) getCurrentItem() *scan.FileItem {
	currentList := a.getCurrentList()
	count := len(currentList)
	if a.index < 0 || a.index >= count {
		return nil
	}
	return &currentList[a.index]
}

// updateStatusBar updates the text of the status bar.
func (a *App) updateStatusBar() {
	if a.UI.statusPathLabel == nil {
		return
	}
	currentItem := a.getCurrentItem()
	statusText := "Ready"

	if currentItem != nil {
		// If currentItem is not nil, a.index is valid and currentItem.Path can be used.
		// Using currentItem.Path is safer than calling GetImageFullPath() here,
		// as GetImageFullPath() might panic if a.index is somehow out of sync.
		statusText = fmt.Sprintf("%s  |  Image %d / %d", currentItem.Path, a.index+1, a.getCurrentImageCount())
		if a.isFiltered {
			statusText += fmt.Sprintf(" (Filtered: %s)", a.currentFilterTag)
		}
	}
	if a.slideshowManager.IsPaused() {
		statusText += " | Paused"
	} else {
		statusText += " | Playing"
	}
	a.UI.statusPathLabel.SetText(statusText) // Update only the path label
}

// addLogMessage adds a message to the UI log display.
func (a *App) addLogMessage(message string) {
	// Optional: Keep console logging here if desired, or move to LogUIManager
	// log.Printf("App->LogUIManager: %s", message)

	if a.logUIManager != nil {
		a.logUIManager.AddLogMessage(message)
	} else {
		// Fallback if LogUIManager is not yet initialized (should ideally not happen in normal flow)
		log.Printf("LogUIManager not ready, console log: %s", message)
		return
	}
}

func (a *App) updateInfoText(info *service.ImageInfo) {
	if a.img.Path == "" {
		a.UI.infoText.ParseMarkdown("# Info\n---\nNo image loaded.")
		return
	}

	if info == nil { // Called when image info isn't available (e.g. load error)
		a.UI.infoText.ParseMarkdown("# Info\n---\nImage metadata not available.")
		return
	}

	// --- Get Tags ---
	currentTags, err := a.Service.ListTagsForImage(a.img.Path) // Use service layer
	tagsString := "(none)"                                     // Default if no tags or error occurred
	// Only join if no error occurred and tags exist
	if err == nil && len(currentTags) > 0 {
		tagsString = strings.Join(currentTags, ", ")
	}

	exifString := "(not available)"

	if len(info.EXIFData) > 0 {
		// Get keys and sort them to ensure a consistent order
		keys := make([]string, 0, len(info.EXIFData))
		for k := range info.EXIFData {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var builder strings.Builder
		for _, k := range keys {
			builder.WriteString(fmt.Sprintf("- **%s**: %s\n\n", k, info.EXIFData[k]))
		}
		exifString = builder.String()
	}
	filterStatus := ""
	if a.isFiltered {
		filterStatus = fmt.Sprintf("\n**Filter Active:** %s\n", a.currentFilterTag)
	}
	md := fmt.Sprintf(`## Stats
%s
**Num:** %s

**Total:** %s

**Size:**   %s bytes

**Width:**   %d px

**Height:**  %d px

**Last modified:** %s

---
## Tags
%s

---
## EXIF Data
%s
`, // Added separator and tags section
		filterStatus,                           // Add filter status
		formatNumberWithCommas(int64(a.index)), // Display current index
		formatNumberWithCommas(int64(a.getCurrentImageCount())), // Use current count
		formatNumberWithCommas(info.Size),                       // Format size
		info.Width,                                              // Reverted
		info.Height,                                             // Reverted
		info.ModTime.Format("2006-01-02 15:04:05"),
		tagsString, // Add the formatted tags string here
		exifString, // Add the formatted EXIF string
	)

	a.UI.infoText.ParseMarkdown(md)
}

// handleImageDisplayError sets the UI state when an image fails to load or decode.
// formatName is optional and only used if errorType is "Decoding".
func (a *App) handleImageDisplayError(imagePath, errorType string, originalError error, formatName string) {
	a.img = Img{Path: imagePath, EXIFData: make(map[string]string)} // Keep path, clear EXIF
	a.zoomPanArea.SetImage(nil)
	a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - Error %s %s", errorType, filepath.Base(imagePath)))
	a.updateInfoText(nil)
	if errorType == "Decoding" && formatName != "" {
		msg := fmt.Sprintf("Error %s %s (format: %s): %v", errorType, filepath.Base(imagePath), formatName, originalError)
		a.addLogMessage(msg)
	} else {
		msg := fmt.Sprintf("Error %s %s: %v", errorType, filepath.Base(imagePath), originalError)
		a.addLogMessage(msg)
	}
}
func (a *App) GetImageFullPath() string {
	currentList := a.getCurrentList() // Use helper
	imagePath := currentList[a.index].Path
	return imagePath
}

// loadAndDisplayCurrentImage loads the image at the current index in the active list
// in a background goroutine and updates the UI on the main Fyne thread.
func (a *App) loadAndDisplayCurrentImage() {
	count := a.getCurrentImageCount()
	// Handle empty list (either full or filtered)

	if count == 0 { // Handle empty list (either full or filtered)
		a.zoomPanArea.SetImage(nil)
		a.img = Img{EXIFData: make(map[string]string)} // Clear EXIF
		a.UI.MainWin.SetTitle("FySlide")
		a.updateStatusBar()
		a.updateInfoText(nil)
		a.addLogMessage("No images available.")
		return // Exit the function, no image to load
	}

	imagePath := a.GetImageFullPath() // Get the full path of the current image

	// Check index bounds again after potential random selection or if not random
	if a.index < 0 || a.index >= count { // Use current count
		// This might happen if images were deleted; try to reset index or handle error
		a.index = 0     // Reset to first image
		if count == 0 { // Double check after reset attempt
			// Already handled above, but defensive check
			// This path should ideally not be hit if the initial count == 0 check is robust.
			// For safety, ensure UI reflects no images.
			fyne.Do(func() {
				a.zoomPanArea.SetImage(nil)                    // Clear the image display
				a.img = Img{EXIFData: make(map[string]string)} // Clear EXIF
				a.UI.MainWin.SetTitle("FySlide")
				a.updateStatusBar()
				a.updateInfoText(nil)
				a.addLogMessage("No images available after index reset.")
			})
			return
		}
		// If count > 0 after reset, update imagePath as index changed
		imagePath = a.GetImageFullPath()
	}

	isHistoryNav := a.isNavigatingHistory // Capture the flag state

	// Launch goroutine for loading and decoding
	go func(path string, historyNav bool) {
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

			// Update Title, Status Bar, and Info Text
			if a.random && !historyNav {
				// When navigating forward in random mode, add to history.
				// If we went back and then forward again, the path might already exist. Remove it first to move it to the back.
				for e := a.thumbnailHistory.Front(); e != nil; e = e.Next() {
					if e.Value.(string) == path {
						a.thumbnailHistory.Remove(e)
						break
					}
				}
				a.thumbnailHistory.PushBack(path)
				// Trim history to keep it from growing indefinitely
				if a.thumbnailHistory.Len() > MaxVisibleThumbnails*2 { // Keep a bit more than visible
					a.thumbnailHistory.Remove(a.thumbnailHistory.Front())
				}
			}
			// Update Title, Status Bar, and Info Text (pass the loaded imgInfo)
			a.updateStatusBar()
			a.updateInfoText(imgInfo)
			a.refreshThumbnailStrip() // Update the thumbnail strip

			// History Update (only if not navigating history)
			if a.historyManager != nil && !historyNav {
				a.historyManager.RecordNavigation(a.img.Path)
			}
			// a.updateShowFullSizeButtonVisibility() // This is now handled by the onZoomPanChange callback
		})
	}(imagePath, isHistoryNav) // Pass the path and flag to the goroutine
}

// showFilterDialog displays a dialog to select a tag for filtering.
func (a *App) showFilterDialog() {
	//allTagsWithCounts, err := a.tagDB.GetAllTags() // This now returns []tagging.TagWithCount
	allTagsWithCounts, err := a.Service.ListAllTags()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get tags for filtering: %w", err), a.UI.MainWin)
		return
	}

	if len(allTagsWithCounts) == 0 {
		dialog.ShowInformation("Filter by Tag", "No tags found in the database to filter by.", a.UI.MainWin)
		return
	}

	// Extract just the tag names for the dialog options
	tagNames := make([]string, len(allTagsWithCounts))
	for i, tagInfo := range allTagsWithCounts {
		tagNames[i] = tagInfo.Name
	}

	// Add option to clear filter
	options := append([]string{"(Show All / Clear Filter)"}, tagNames...)

	var selectedOption string
	filterSelector := widget.NewSelect(options, func(selected string) {
		selectedOption = selected
	})
	// Set initial selection based on current filter
	if a.isFiltered {
		filterSelector.SetSelected(a.currentFilterTag)
		selectedOption = a.currentFilterTag
	} else {
		filterSelector.SetSelected(options[0]) // Default to "Show All"
		selectedOption = options[0]
	}

	dialog.ShowForm("Filter by Tag", "Apply", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Select Tag", filterSelector),
	}, func(confirm bool) {
		if !confirm {
			return
		}

		if selectedOption == options[0] { // "(Show All / Clear Filter)"
			a.clearFilter()
		} else {
			a.applyFilter(selectedOption)
		}
	}, a.UI.MainWin)
}

// handleShowFullSizeBtn is called when the "Show Full Size" toolbar action is triggered.
func (a *App) handleShowFullSizeBtn() {
	if a.zoomPanArea != nil {
		a.slideshowManager.Pause(true) // Pause slideshow when user interacts with zoom
		a.zoomPanArea.ShowFullSize()
		// The onZoomPanChange callback, which is updateShowFullSizeButtonVisibility,
		// will be triggered by ShowFullSize, updating the button's state.
	}
}

// updateShowFullSizeButtonVisibility enables or disables the "Show Full Size" toolbar action
// based on the current image's zoom state and original size relative to the view.
func (a *App) updateShowFullSizeButtonVisibility() {
	if a.UI.showFullSizeAction == nil || a.zoomPanArea == nil || a.zoomPanArea.originalImg == nil {
		if a.UI.showFullSizeAction != nil {
			a.UI.showFullSizeAction.Disable()
			if a.UI.toolBar != nil {
				a.UI.toolBar.Refresh()
			}
		}
		return
	}

	currentZoom := a.zoomPanArea.CurrentZoom()
	epsilon := float32(0.001) // Tolerance for float comparison

	shouldBeEnabled := (currentZoom < (1.0 - epsilon)) || (currentZoom > (1.0 + epsilon))

	if shouldBeEnabled {
		a.UI.showFullSizeAction.Enable()
	} else {
		a.UI.showFullSizeAction.Disable()
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
}

// applyFilter filters the image list based on the selected tag.
func (a *App) applyFilter(tag string) {
	a.addLogMessage(fmt.Sprintf("Applying filter for tag: %s", tag))
	//tagImagesPaths, err := a.tagDB.GetImages(tag)
	tagImagesPaths, err := a.Service.ListImagesForTag(tag)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tag, err), a.UI.MainWin)
		a.clearFilter() // Revert if error occurs
		return
	}

	if len(tagImagesPaths) == 0 {
		dialog.ShowInformation("Filter Results", fmt.Sprintf("No images found with the tag '%s'.", tag), a.UI.MainWin)
		a.addLogMessage(fmt.Sprintf("No images found with tag '%s'.", tag))
		// Decide whether to clear filter or keep showing nothing - clearing is probably better UX
		a.clearFilter()
		return
	}

	// Build the filtered list
	var newFilteredImages scan.FileItems
	// Create a map for quick path lookup
	pathMap := make(map[string]bool)
	for _, path := range tagImagesPaths {
		pathMap[path] = true
	}

	// Iterate through the original full list to maintain FileItem structure
	for _, item := range a.images {
		if _, found := pathMap[item.Path]; found {
			newFilteredImages = append(newFilteredImages, item)
		}
	}

	if len(newFilteredImages) == 0 {
		// This might happen if tagged images were deleted/moved from the original scan
		dialog.ShowInformation("Filter Results", fmt.Sprintf("No currently loaded images match the tag '%s'.", tag), a.UI.MainWin)
		a.addLogMessage(fmt.Sprintf("No loaded images match tag '%s'.", tag))
		a.clearFilter()
		return
	}

	a.filteredImages = newFilteredImages
	a.isFiltered = true
	a.currentFilterTag = tag
	a.index = 0 // Reset index to the start of the filtered list
	a.thumbnailHistory.Init()
	a.navigationQueue.ResetAndFill(a.index)
	a.addLogMessage(fmt.Sprintf("Filter active: %d images with tag '%s'.", len(a.filteredImages), tag))

	a.isNavigatingHistory = false  // Applying a filter is a new view, not history navigation
	a.loadAndDisplayCurrentImage() // Display the first image in the filtered set
	a.refreshThumbnailStrip()      // Update the thumbnail strip
	//a.updateInfoText(nil)          // Update info panel immediately
	//a.updateStatusBar()
}

// clearFilter removes any active tag filter.
func (a *App) clearFilter() {
	if !a.isFiltered {
		return // Nothing to clear
	}
	a.addLogMessage("Filter cleared. Showing all images.")
	a.isFiltered = false
	a.currentFilterTag = ""
	a.filteredImages = nil // Clear the filtered list
	a.index = 0            // Reset index to the start of the full list
	a.thumbnailHistory.Init()
	a.navigationQueue.ResetAndFill(a.index)

	a.isNavigatingHistory = false  // Clearing a filter is a new view state
	a.loadAndDisplayCurrentImage() // Display the first image in the full set
	a.refreshThumbnailStrip()      // Update the thumbnail strip
	//a.updateInfoText()             // Update info panel immediately
	//a.updateStatusBar()
}

func (a *App) firstImage() {
	a.isNavigatingHistory = false
	if a.getCurrentImageCount() == 0 {
		return
	} // Add check
	a.index = 0
	a.navigationQueue.ResetAndFill(a.index)
	a.loadAndDisplayCurrentImage()
}

func (a *App) lastImage() {
	a.isNavigatingHistory = false
	count := a.getCurrentImageCount() // Use helper
	if count == 0 {
		return
	} // Add check
	a.index = count - 1
	a.navigationQueue.ResetAndFill(a.index)
	a.loadAndDisplayCurrentImage()
}

// navigate moves the current image by a given offset.
// A positive offset moves forward, a negative offset moves backward sequentially.
// offset=1 handles special "forward in history" logic.
func (a *App) navigate(offset int) {
	count := a.getCurrentImageCount() // Use helper
	if count == 0 {
		return
	} // Add check

	// Special case: If we were navigating history, "next" (offset=1) should try to move forward in history.
	if offset == 1 && a.isNavigatingHistory {
		if a.ShowNextImageFromHistory() {
			return // Successfully moved forward in history
		}
		// If not, fall through to normal navigation.
	}

	a.isNavigatingHistory = false // Any other navigation is a new action.

	if offset > 0 { // Forward navigation
		newIndex := a.index
		// For a single step, just pop once. For a skip, loop.
		for i := 0; i < offset; i++ {
			popped := a.navigationQueue.PopAndAdvance()
			if popped == -1 {
				break // Queue is exhausted
			}
			newIndex = popped
		}
		a.index = newIndex
	} else { // Backward navigation (offset is negative or zero)
		// Backward navigation is always sequential. If in random mode, turn it off.
		if a.random {
			a.toggleRandom()
		}
		a.index += offset
		if a.index < 0 {
			a.index = 0
		}
		// Since we did a sequential jump, the queue must be reset.
		a.navigationQueue.ResetAndFill(a.index)
	}

	a.loadAndDisplayCurrentImage()
}

// ShowNextImageFromHistory attempts to move forward in the history stack.
// Returns true if successful, false otherwise (e.g., at end of history or history disabled).
func (a *App) ShowNextImageFromHistory() bool {
	imagePathFromHistory, ok := a.historyManager.NavigateForward()
	if !ok {
		return false
	}

	a.isNavigatingHistory = true // Signal DisplayImage not to add to history stack for this action

	// Check if the historical image is in the current filtered list if filtering is active.
	// Unlike 'back', for 'forward' it might be better to just skip if it's not in the filter,
	// or clear the filter. Clearing the filter seems more consistent with the 'back' behavior.
	mustClearFilter := false
	if a.isFiltered {
		foundInFilter := false
		for _, item := range a.filteredImages {
			if item.Path == imagePathFromHistory {
				foundInFilter = true
				break
			}
		}
		if !foundInFilter {
			mustClearFilter = true
		}
	}

	if mustClearFilter {
		a.addLogMessage(fmt.Sprintf("Image %s from history not in current filter. Clearing filter state for forward navigation.", filepath.Base(imagePathFromHistory)))
		// Directly modify filter state without calling a.clearFilter() to avoid its DisplayImage call
		a.isFiltered = false
		a.currentFilterTag = ""
		a.filteredImages = nil
		// The info text will be updated by the DisplayImage call later.
	}

	// Find the index of the historical image in the *current* active list (full or filtered)
	activeList := a.getCurrentList()
	foundIndexInActiveList := -1
	for i, item := range activeList {
		if item.Path == imagePathFromHistory {
			foundIndexInActiveList = i
			break
		}
	}

	if foundIndexInActiveList == -1 {
		a.addLogMessage(fmt.Sprintf("Error: Image from history (%s) not found in current active list during forward navigation. Removing from history.", filepath.Base(imagePathFromHistory)))
		// Image might have been deleted or is otherwise inaccessible.
		a.historyManager.RemovePath(imagePathFromHistory) // Remove problematic path
		dialog.ShowInformation("History Navigation", "A previously viewed image is no longer available and was removed from history.", a.UI.MainWin)
		a.isNavigatingHistory = false // Reset flag as this specific navigation failed
		return false                  // Failed to show the historical image
	}

	a.index = foundIndexInActiveList
	a.navigationQueue.ResetAndFill(a.index)

	a.loadAndDisplayCurrentImage() // loadAndDisplayCurrentImage will respect a.isNavigatingHistory
	// Error handling is now internal to loadAndDisplayCurrentImage
	a.isNavigatingHistory = false // Reset flag after the operation is complete
	return true                   // Successfully displayed historical image
}

// ShowPreviousImage handles the "back" button logic using history.
func (a *App) ShowPreviousImage() {

	// --- Turn off random mode if active ---
	if a.random {
		a.toggleRandom() // This function handles icon update and state change
	}

	// --- Pause slideshow if it's playing (user is navigating back) ---
	if !a.slideshowManager.IsPaused() {
		a.togglePlay() // This effectively pauses it via user action
	}

	imagePathFromHistory, ok := a.historyManager.NavigateBack()
	if !ok {
		a.addLogMessage("No previous image in history.")
		return
	}
	a.isNavigatingHistory = true // Signal DisplayImage not to add to history stack for this action

	mustClearFilter := false
	if a.isFiltered {
		foundInFilter := false
		for _, item := range a.filteredImages {
			if item.Path == imagePathFromHistory {
				foundInFilter = true
				break
			}
		}
		if !foundInFilter {
			mustClearFilter = true
		}
	}

	if mustClearFilter {
		a.addLogMessage(fmt.Sprintf("Image %s from history not in current filter. Clearing filter state.", filepath.Base(imagePathFromHistory)))
		// Directly modify filter state without calling a.clearFilter() to avoid its DisplayImage call
		a.isFiltered = false
		a.currentFilterTag = ""
		a.filteredImages = nil
		// The info text will be updated by the DisplayImage call later.
	}

	activeList := a.getCurrentList() // Get the list that's now active (could be a.images if filter was cleared)
	foundIndexInActiveList := -1
	for i, item := range activeList {
		if item.Path == imagePathFromHistory {
			foundIndexInActiveList = i
			break
		}
	}

	if foundIndexInActiveList == -1 {
		a.addLogMessage(fmt.Sprintf("Error: Image from history (%s) not found in current active list. Removing from history.", filepath.Base(imagePathFromHistory)))
		a.historyManager.RemovePath(imagePathFromHistory) // Remove problematic path
		dialog.ShowInformation("History Navigation", "A previously viewed image is no longer available and was removed from history.", a.UI.MainWin)
		a.isNavigatingHistory = false // Reset flag as this specific navigation failed
		return
	}

	a.index = foundIndexInActiveList
	a.navigationQueue.ResetAndFill(a.index)

	a.loadAndDisplayCurrentImage() // loadAndDisplayCurrentImage will respect a.isNavigatingHistory
	// Error handling is now internal to loadAndDisplayCurrentImage
	a.isNavigatingHistory = false // Reset flag after the operation is complete
}

// Delete file

func (a *App) deleteFileCheck() {
	dialog.ShowConfirm("Delete file!", "Are you sure?\n This action can't be undone.", func(b bool) {
		if b {
			a.deleteFile()
		}
	}, a.UI.MainWin)
}

func (a *App) deleteFile() {
	deletedPath := a.img.Path
	if deletedPath == "" {
		return
	} // No image loaded

	err := a.Service.DeleteImageFile(deletedPath)
	if err != nil {
		a.addLogMessage(fmt.Sprintf("Error deleting file and tags: %v", err))
		// If the service layer couldn't delete the file (and its tags),
		// it might be best to not alter the UI lists further.
		dialog.ShowError(err, a.UI.MainWin)
		return
	}

	// 3. Remove from the main image list (a.images)
	originalIndex := -1
	newImages := a.images[:0]
	for i, item := range a.images {
		if item.Path == deletedPath {
			originalIndex = i // Keep track of original index if needed
		} else {
			newImages = append(newImages, item)
		}
	}
	if originalIndex != -1 {
		a.addLogMessage(fmt.Sprintf("Removed %s from image list.", filepath.Base(deletedPath)))
	} else {
		a.addLogMessage(fmt.Sprintf("Warning: Image %s not found in main list during deletion.", deletedPath))
	}
	a.images = newImages

	// 3.5. Remove from historyStack
	a.removeFromThumbnailHistory(deletedPath)
	if a.historyManager != nil {
		a.historyManager.RemovePath(deletedPath)
	}

	// 4. Remove from the filtered list (a.filteredImages) if filtering is active
	if a.isFiltered {
		newFiltered := a.filteredImages[:0]
		for _, item := range a.filteredImages {
			if item.Path != deletedPath {
				newFiltered = append(newFiltered, item)
			}
		}
		a.filteredImages = newFiltered
		// If the filtered list becomes empty, clear the filter
		if len(a.filteredImages) == 0 {
			a.addLogMessage("Filtered list empty after deletion, clearing filter.")
			a.clearFilter() // This will reset index and display
			return          // clearFilter calls DisplayImage
		}
	}

	// 5. Adjust index and display the next image
	count := a.getCurrentImageCount()
	if count == 0 {
		// No images left at all (or in filter)
		a.index = -1 // Indicate no valid index
	} else {
		// Adjust index carefully
		if a.index >= count { // If we deleted the last item
			a.index = count - 1
		}
		// Ensure index is within bounds [0, count-1]
		if a.index < 0 {
			a.index = 0
		}
	}
	// After the image lists are updated and the new index is calculated,
	// reset the navigation queue to reflect the new state. This ensures
	// the thumbnail strip will be correct.
	a.navigationQueue.ResetAndFill(a.index)
	// Common call after index adjustment
	a.loadAndDisplayCurrentImage()
	a.refreshThumbnailStrip() // Update the thumbnail strip
}

// removeFromThumbnailHistory removes a given path from the random-mode thumbnail cache.
func (a *App) removeFromThumbnailHistory(path string) {
	if a.thumbnailHistory == nil {
		return
	}
	for e := a.thumbnailHistory.Front(); e != nil; e = e.Next() {
		if e.Value.(string) == path {
			a.thumbnailHistory.Remove(e)
			break // Assume path is unique
		}
	}
}

// func pathToURI(path string) (fyne.URI, error) {
// 	absPath, _ := filepath.Abs(path)
// 	fileURI := storage.NewFileURI(absPath)
// 	return fileURI, nil
// }

func (a *App) loadImages(root string) {
	a.images = nil // Clear previous images or a.images = a.images[:0]

	// Define a logger function that matches scan.LoggerFunc
	// and uses the app's logUIManager.
	scanLogger := func(message string) {
		// fyne.Do is important if scan.Run's logger calls happen from a non-main goroutine
		// and a.addLogMessage directly updates UI. a.addLogMessage itself uses logUIManager.
		fyne.Do(func() { a.addLogMessage(message) })
	}
	//imageChan := scan.Run(root, scanLogger) // Pass the logger
	imageChan := a.Service.FileScan.Run(root, scanLogger)
	for item := range imageChan { // Loop until the channel is closed
		a.images = append(a.images, item)
		// Optionally, you could update a progress indicator here
		// if the GUI needs to show loading progress.
	}
	msg := fmt.Sprintf("Loaded %d images from %s", len(a.images), root)
	fyne.Do(func() {
		a.addLogMessage(msg)
		a.refreshThumbnailStrip() // Update the thumbnail strip
	})
}

func (a *App) imageCount() int {
	return len(a.images)
}

func (a *App) init(historyCap int, slideshowIntervalSec float64, skipNum int) {
	a.img = Img{EXIFData: make(map[string]string)} // Initialize EXIFData
	a.historyManager = history.NewHistoryManager(historyCap)

	// Define a logger function for SlideshowManager
	// This closure captures 'a' (the App instance).
	slideshowLogger := func(message string) {
		// Ensure UI updates from logs happen on the Fyne goroutine.
		// a.addLogMessage itself uses a.logUIManager which updates UI.
		fyne.Do(func() { a.addLogMessage(fmt.Sprintf("Slideshow: %s", message)) })
	}

	a.skipCount = skipNum
	a.slideshowManager = slideshow.NewSlideshowManager(time.Duration(slideshowIntervalSec*1000)*time.Millisecond, slideshowLogger) //nolint:durationcheck
	a.isNavigatingHistory = false
	a.maxLogMessages = DefaultMaxLogMessages

	// SlideshowManager's constructor handles default interval if slideshowIntervalSec is invalid
	// So, no need for a separate check here for slideshowIntervalSec.

	if a.skipCount <= 0 {
		fyne.LogError(fmt.Sprintf("Skip count must be positive. Defaulting to %d. Got: %d", DefaultSkipCount, skipNum), nil)
		a.skipCount = DefaultSkipCount
	}
}

// Handle toggles
func (a *App) togglePlay() {
	a.slideshowManager.TogglePlayPause()
	if a.slideshowManager.IsPaused() { // Toggle state using the manager
		if a.UI.pauseAction != nil { // Check if pauseAction is initialized
			a.UI.pauseAction.SetIcon(theme.MediaPlayIcon())
		}
	} else {
		// Now playing (not paused), so button should offer to pause
		if a.UI.pauseAction != nil { // Check if pauseAction is initialized
			a.UI.pauseAction.SetIcon(theme.MediaPauseIcon())
		}
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
	a.updateStatusBar()
}

func (a *App) toggleRandom() {
	a.random = !a.random // Toggle state first
	a.navigationQueue.SetMode(a.random)
	if a.random {
		// Random is ON, show active dice
		if a.UI.randomAction != nil {
			if a.thumbnailHistory.Len() == 0 && a.img.Path != "" {
				a.thumbnailHistory.PushBack(a.img.Path)
			}
			a.UI.randomAction.SetIcon(resourceDice24Png)
		}
	} else {
		// Random is OFF, show disabled dice
		if a.UI.randomAction != nil {
			a.UI.randomAction.SetIcon(resourceDiceDisabled24Png)
		}
		a.thumbnailHistory.Init() // Clear the history when leaving random mode
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
}

// Command-line flags
var historySizeFlag = flag.Int("history-size", 10, "Number of last viewed images to remember (0 to disable). Min: 0.")
var slideshowIntervalFlag = flag.Float64("slideshow-interval", 3.0, "Slideshow image display interval in seconds. Min: 0.1.")
var skipCountFlag = flag.Int("skip-count", 20, "Number of images to skip with PageUp/PageDown. Min: 1.")

// CreateApplication is the GUI entrypoint
func CreateApplication() {
	flag.Parse() // Parse command-line flags
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("error while opening the directory : %v\n", err)
		return
	}
	if len(os.Args) > 1 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Printf("error while opening the directory '%s': %v\n", file.Name(), err)
			return
		}
		s, _ := file.Stat()
		if s.IsDir() {
			dir = s.Name()
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return
	}

	a := app.NewWithID("com.github.nicky-ayoub/fyslide")
	a.SetIcon(resourceIconPng)

	currentTheme := a.Settings().Theme()
	a.Settings().SetTheme(NewSmallTabsTheme(currentTheme)) //nolint:staticcheck

	ui := &App{app: a}

	// Define the logger function that TagDB will use.
	// This closure captures the 'ui' variable (*App instance).
	appLoggerFunc := func(message string) {
		if ui.logUIManager != nil { // Check if logUIManager has been initialized
			ui.logUIManager.AddLogMessage(message)
		} else {
			// Fallback to console log if logUIManager is not yet ready
			// This might happen for logs from NewTagDB before buildMainUI completes.
			log.Printf("EarlyTagDBLog: %s", message)
		}
	}

	ui.tagDB, err = tagging.NewTagDB("", appLoggerFunc) // Pass the logger function
	if err != nil {
		log.Fatalf("Failed to initialize tag database: %v", err)
	}
	// --- Service Layer Integration ---
	fileScanner := scan.FileScannerImpl{} // You may need to implement this as shown earlier
	ui.Service = service.NewService(ui.tagDB, &fileScanner, appLoggerFunc)
	ui.thumbnailManager = NewThumbnailManager(ui)
	ui.ImageService = service.NewImageService()
	// Initialize UI components that need the app instance
	ui.UI.MainWin = a.NewWindow("FySlide")
	ui.UI.MainWin.SetCloseIntercept(func() {
		log.Println("Closing tag database...")
		if err := ui.tagDB.Close(); err != nil {
			log.Printf("Error closing tag database: %v", err)
		}
		ui.UI.MainWin.Close() // Proceed with closing the window
	})

	ui.UI.MainWin.SetIcon(resourceIconPng)
	ui.init(*historySizeFlag, *slideshowIntervalFlag, *skipCountFlag) // Pass parsed flags to init
	ui.random = true
	ui.thumbnailHistory = list.New()
	ui.navigationQueue = NewNavigationQueue(ui)
	ui.navigationQueue.SetMode(ui.random) // Synchronize the queue's mode with the app's state at startup

	ui.UI.clockLabel = widget.NewLabel("Time: ")
	ui.UI.infoText = widget.NewRichTextFromMarkdown("# Info\n---\n")

	// Status bar will be initialized in buildMainUI
	ui.UI.MainWin.SetContent(ui.buildMainUI())

	go ui.loadImages(dir)

	ui.UI.MainWin.CenterOnScreen()
	ui.UI.MainWin.SetFullScreen(true)

	// Wait for initial scan
	startTime := time.Now()
	for ui.imageCount() < 1 {
		if time.Since(startTime) > 10*time.Second { // Timeout
			ui.addLogMessage("Timeout waiting for images to load. Please check the directory.")
			// No images loaded, so the UI will reflect this.
			break
		}
		time.Sleep(100 * time.Millisecond) // Slightly longer sleep
	}

	// Check if images were actually loaded
	if ui.imageCount() > 0 {
		// If in random mode, pick a random starting image. Otherwise, start at 0.
		if ui.random {
			ui.index = rand.Intn(ui.imageCount())
			if ui.imageCount() > 0 {
				ui.thumbnailHistory.PushBack(ui.images[ui.index].Path)
			}
		}
		ui.navigationQueue.ResetAndFill(ui.index) // Initialize the queue from the chosen start index
		ticker := time.NewTicker(ui.slideshowManager.Interval())
		ui.isNavigatingHistory = false // Initial display is not from history
		go ui.pauser(ticker)           // pauser will call loadAndDisplayCurrentImage via fyne.Do
		go ui.updateTimer()
		ui.loadAndDisplayCurrentImage()
	} else {
		// This case is also hit on timeout if no images loaded.
		ui.updateStatusBar() // Will show "No images available" or similar.
		ui.updateInfoText(nil)
	}

	ui.UI.MainWin.ShowAndRun()
}

func (a *App) updateTimer() {
	for range time.Tick(time.Second) {
		// ???
		if a.UI.MainWin == nil || a.UI.clockLabel == nil { // Check if UI elements are still valid
			return // Exit goroutine if window is closed
		}
		fyne.Do(func() {
			formatted := time.Now().Format("Time: 03:04:05")
			a.UI.clockLabel.SetText(formatted)
		})
	}
}

func (a *App) pauser(ticker *time.Ticker) {
	for range ticker.C {
		if a.UI.MainWin == nil { // Check if window is still valid
			ticker.Stop() // Stop the ticker
			return        // Exit goroutine
		}
		if !a.slideshowManager.IsPaused() {
			fyne.Do(func() {
				a.isNavigatingHistory = false // Standard navigation is not history navigation
				a.navigate(1)
			})
		}
	}
}

func (a *App) removeTagGlobally(tag string) error {
	if tag == "" {
		return nil
	}
	a.addLogMessage(fmt.Sprintf("Global removal for tag '%s' started.", tag))
	successes, errors, err := a.Service.RemoveTagGlobally(tag)
	a.addLogMessage(fmt.Sprintf("Global removal for '%s': %d successes, %d errors.", tag, successes, errors))
	return err
}

// postOperationUpdate handles UI updates after a tag operation.
func (a *App) postOperationUpdate(errOp error, statusMessage string, filesAffectedCount int, wasCurrentFileAffected bool) {
	if errOp != nil {
		dialog.ShowError(errOp, a.UI.MainWin)
		a.addLogMessage(fmt.Sprintf("Error during tag operation: %v", errOp))
	} else {
		if statusMessage != "" {
			a.addLogMessage(fmt.Sprintf("Tag Operation Status: %s", statusMessage))
		}
	}

	if filesAffectedCount > 0 {
		// If any files were affected, refresh the global tags list view
		if a.refreshTagsFunc != nil {
			a.refreshTagsFunc()
		}

		// If the currently displayed image was one of those affected, update its info panel
		if wasCurrentFileAffected {
			imgInfo, _, err := a.ImageService.GetImageInfo(a.img.Path)
			if err == nil && imgInfo != nil {
				a.updateInfoText(imgInfo)
			} else {
				a.addLogMessage(fmt.Sprintf("Error reloading info for current image after tag op: %v", err))
			}
		}
	}
}

// handleTagOperation provides a generic framework for creating a tag operation dialog.
func (a *App) handleTagOperation(
	title string,
	verb string,
	formItems []*widget.FormItem,
	focusableWidget fyne.Focusable,
	preDialogCheck func() bool,
	execute func(confirm bool),
) {
	if a.img.Path == "" {
		dialog.ShowInformation(title, "No image loaded to "+strings.ToLower(verb)+" tags.", a.UI.MainWin)
		return
	}

	if preDialogCheck != nil && !preDialogCheck() {
		return
	}

	a.slideshowManager.Pause(true)
	if a.slideshowManager.IsPaused() {
		a.addLogMessage(fmt.Sprintf("Slideshow paused for tag %s.", strings.ToLower(verb)))
	}

	dialogCallback := func(confirm bool) {
		defer func() {
			a.slideshowManager.ResumeAfterOperation()
			if !a.slideshowManager.IsPaused() {
				a.addLogMessage("Slideshow resumed.")
			}
		}()

		if !confirm {
			return
		}
		execute(confirm)
	}

	formDialog := dialog.NewForm(title, verb, "Cancel", formItems, dialogCallback, a.UI.MainWin)

	// If the focusable widget is an entry, set its OnSubmitted behavior
	if entry, ok := focusableWidget.(*widget.Entry); ok {
		entry.OnSubmitted = func(text string) {
			if text != "" {
				a.addLogMessage(fmt.Sprintf("Submitting %s for processing: %s", strings.ToLower(title), text))
				formDialog.Submit()
			}
		}
	}

	formDialog.Show()
	if focusableWidget != nil {
		a.UI.MainWin.Canvas().Focus(focusableWidget)
	}
}

// tagOperationFunc defines a function that performs a tag operation on a single image path with a set of tags.
type tagOperationFunc func(imagePath string, tags []string) error

// _processTagsForDirectory handles batch tag operations (add/remove) for all images in a directory.
// It uses goroutines for concurrent database operations.
func (a *App) _processTagsForDirectory(
	currentDir string,
	tags []string,
	operation tagOperationFunc,
	operationVerb string, // e.g., "tagging" or "untagging"
) (successfulImages, erroredImages, imagesProcessed int, firstError error, filesAffected map[string]bool) {

	a.addLogMessage(fmt.Sprintf("Batch %s directory: %s with [%s]", operationVerb, filepath.Base(currentDir), strings.Join(tags, ", ")))

	type result struct {
		path string
		err  error
	}

	var imagesToProcess []string
	for _, imageItem := range a.images {
		if filepath.Dir(imageItem.Path) == currentDir {
			imagesToProcess = append(imagesToProcess, imageItem.Path)
		}
	}

	if len(imagesToProcess) == 0 {
		return 0, 0, 0, nil, make(map[string]bool)
	}

	resultsChan := make(chan result, len(imagesToProcess))
	var wg sync.WaitGroup

	for _, path := range imagesToProcess {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			err := operation(p, tags)
			resultsChan <- result{path: p, err: err}
		}(path)
	}

	wg.Wait()
	close(resultsChan)

	filesAffected = make(map[string]bool)
	for res := range resultsChan {
		imagesProcessed++
		if res.err != nil {
			erroredImages++
			if firstError == nil {
				firstError = fmt.Errorf("failed to %s %s: %w", operationVerb, filepath.Base(res.path), res.err)
			}
		} else {
			successfulImages++
			filesAffected[res.path] = true
		}
	}

	a.addLogMessage(fmt.Sprintf("Batch %s for [%s] in '%s' complete. Images processed: %d, Successes: %d, Errors: %d.",
		operationVerb, strings.Join(tags, ", "), filepath.Base(currentDir), imagesProcessed, successfulImages, erroredImages))
	return
}

// addTag shows a dialog to add a new tag to the current image
func (a *App) addTag() {
	currentTags, err := a.Service.ListTagsForImage(a.img.Path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
		return
	}

	tagEntry := widget.NewEntry()
	tagEntry.SetPlaceHolder("Enter tag(s) separated by commas...")

	currentTagsText := "Current tags: (none)"
	if len(currentTags) > 0 {
		currentTagsText = fmt.Sprintf("Current tags: %s", strings.Join(currentTags, ", "))
	}
	currentTagsLabel := widget.NewLabel(currentTagsText)

	applyToAllCheck := widget.NewCheck("Apply tag(s) to all images in this directory", nil)
	applyToAllCheck.SetChecked(true)

	formItems := []*widget.FormItem{
		widget.NewFormItem("", currentTagsLabel), // Display current tags
		widget.NewFormItem("New Tag(s)", tagEntry),
		widget.NewFormItem("", applyToAllCheck),
	}

	execute := func(confirm bool) {
		rawInput := tagEntry.Text
		potentialTags := strings.Split(rawInput, ",")
		var tagsToAdd []string
		uniqueTags := make(map[string]bool)
		for _, pt := range potentialTags {
			tag := strings.ToLower(strings.TrimSpace(pt))
			if tag != "" && !uniqueTags[tag] {
				tagsToAdd = append(tagsToAdd, tag)
				uniqueTags[tag] = true
			}
		}

		if len(tagsToAdd) == 0 {
			dialog.ShowInformation("Add Tags", "No valid tags entered.", a.UI.MainWin)
			return
		}

		applyToAll := applyToAllCheck.Checked
		var errAddOp error
		var statusMessage string
		filesAffected := make(map[string]bool)
		var successfulAdditions, errorsEncountered int

		if applyToAll {
			currentDir := filepath.Dir(a.img.Path)
			var successfulImages, erroredImages int
			// Use '=' to assign to the outer errAddOp and filesAffected variables
			successfulImages, erroredImages, _, errAddOp, filesAffected = a._processTagsForDirectory(currentDir, tagsToAdd, a.Service.AddTagsToImage, "tagging")
			successfulAdditions = successfulImages * len(tagsToAdd)
			errorsEncountered = erroredImages * len(tagsToAdd)

			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Partial success adding tags to %d images. %d errors occurred.", len(filesAffected), errorsEncountered)
			} else if successfulAdditions > 0 {
				statusMessage = fmt.Sprintf("Added tag(s) to %d images in %s.", len(filesAffected), filepath.Base(currentDir))
			}
		} else {
			errAddOp = a.Service.AddTagsToImage(a.img.Path, tagsToAdd)
			if errAddOp == nil {
				successfulAdditions = len(tagsToAdd)
				filesAffected[a.img.Path] = true
			} else {
				errorsEncountered = len(tagsToAdd)
			}
			a.addLogMessage(fmt.Sprintf("Add to %s: %d successes, %d errors.", filepath.Base(a.img.Path), successfulAdditions, errorsEncountered))
			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Partial success adding tags. %d errors occurred.", errorsEncountered)
			} else if successfulAdditions > 0 {
				statusMessage = fmt.Sprintf("Added %d tag(s) to current image.", len(tagsToAdd))
			}
		}
		a.postOperationUpdate(errAddOp, statusMessage, len(filesAffected), filesAffected[a.img.Path])
	}

	a.handleTagOperation("Add Tag", "Add", formItems, tagEntry, nil, execute)
}

// removeTag shows a dialog to remove an existing tag from the current image,
// with an option to remove it from all images in the same directory.
func (a *App) removeTag() {
	currentTags, err := a.Service.ListTagsForImage(a.img.Path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
		return
	}

	preCheck := func() bool {
		if len(currentTags) == 0 {
			dialog.ShowInformation("Remove Tag", "This image has no tags to remove.", a.UI.MainWin)
			return false
		}
		return true
	}

	var selectedTag string
	tagSelector := widget.NewSelect(currentTags, func(s string) { selectedTag = s })
	tagSelector.SetSelected(currentTags[0])
	selectedTag = currentTags[0]

	removeFromAllCheck := widget.NewCheck("Remove tag(s) from all images in this directory", nil)

	formItems := []*widget.FormItem{
		widget.NewFormItem("Select Tag to Remove", tagSelector),
		widget.NewFormItem("", removeFromAllCheck),
	}

	execute := func(confirm bool) {
		if selectedTag == "" {
			return
		}
		applyToAll := removeFromAllCheck.Checked
		var errRemoveOp error
		var statusMessage string
		var imagesUntaggedCount, errorsEncountered int
		filesAffected := make(map[string]bool)

		if applyToAll {
			currentDir := filepath.Dir(a.img.Path)
			op := func(path string, tags []string) error {
				return a.Service.RemoveTagsFromImage(path, tags)
			}
			imagesUntaggedCount, errorsEncountered, _, errRemoveOp, filesAffected = a._processTagsForDirectory(currentDir, []string{selectedTag}, op, "untagging")

			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Partial success removing tag. %d images untagged, %d errors.", imagesUntaggedCount, errorsEncountered)
			} else if imagesUntaggedCount > 0 {
				statusMessage = fmt.Sprintf("Tag '%s' removed from %d images in directory %s.", selectedTag, imagesUntaggedCount, filepath.Base(currentDir))
			}
		} else {
			errRemoveOp = a.Service.RemoveTagsFromImage(a.img.Path, []string{selectedTag})
			if errRemoveOp == nil {
				imagesUntaggedCount = 1
				filesAffected[a.img.Path] = true
				statusMessage = fmt.Sprintf("Tag '%s' removed from current image.", selectedTag)
			}
			a.addLogMessage(fmt.Sprintf("Remove from %s: %d successes, %d errors.", filepath.Base(a.img.Path), imagesUntaggedCount, errorsEncountered))
		}
		a.postOperationUpdate(errRemoveOp, statusMessage, len(filesAffected), filesAffected[a.img.Path])
	}

	a.handleTagOperation("Remove Tag", "Remove", formItems, nil, preCheck, execute)
}

// getThumbnailWindowPaths assembles the list of image paths for the thumbnail strip.
// In random mode, it constructs a "sliding window" based on navigation history.
// It returns nil if it should fall back to sequential logic.
func (a *App) getThumbnailWindowPaths() []string {
	if !a.random || a.thumbnailHistory.Len() == 0 || a.img.Path == "" {
		return nil // Use sequential logic
	}

	// 1. Find current image in our history cache
	var currentElement *list.Element
	for e := a.thumbnailHistory.Back(); e != nil; e = e.Prev() {
		if e.Value.(string) == a.img.Path {
			currentElement = e
			break
		}
	}

	if currentElement == nil {
		return nil // Not found, fallback to sequential
	}

	displayPaths := make([]string, 0, MaxVisibleThumbnails)

	// 2. Add previous images by walking backwards from currentElement
	numPrev := MaxVisibleThumbnails / 2
	e := currentElement
	for i := 0; i < numPrev; i++ {
		e = e.Prev()
		if e == nil {
			break
		}
		displayPaths = append([]string{e.Value.(string)}, displayPaths...) // Prepend
	}

	// 3. Add current image
	displayPaths = append(displayPaths, currentElement.Value.(string))

	// 4. Add next images from history cache first
	e = currentElement
	for len(displayPaths) < MaxVisibleThumbnails {
		e = e.Next()
		if e == nil {
			break
		}
		displayPaths = append(displayPaths, e.Value.(string))
	}

	// 5. Fill remaining slots from the navigationQueue
	if len(displayPaths) < MaxVisibleThumbnails {
		needed := MaxVisibleThumbnails - len(displayPaths)
		upcomingIndices := a.navigationQueue.GetUpcoming(needed + 1) // +1 for current
		if len(upcomingIndices) > 1 {
			currentList := a.getCurrentList()
			for _, idx := range upcomingIndices[1:] {
				if idx >= 0 && idx < len(currentList) {
					displayPaths = append(displayPaths, currentList[idx].Path)
				}
			}
		}
	}

	return displayPaths
}
