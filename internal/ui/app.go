// Package ui  Setup for the FySlide Application
package ui

import (
	"flag"
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/slideshow"
	"fyslide/internal/tagging"
	"image"
	"log"
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

	images                     scan.FileItems           // The original, full list of images
	permutationManager         *scan.PermutationManager // Manages the original images
	filteredImages             scan.FileItems           // The list when a filter is active
	filteredPermutationManager *scan.PermutationManager
	index                      int

	isFiltered       bool   // NEW: Flag to indicate if filtering is active
	currentFilterTag string // NEW: The tag currently being filtered by

	img         Img
	zoomPanArea *ZoomPanArea

	slideshowManager *slideshow.SlideshowManager // NEW: Use SlideshowManager

	random bool

	tagDB *tagging.TagDB // Add the tag database instance

	isDarkTheme bool // NEW: track current theme

	// refreshTagsFunc holds the function returned by buildTagsTab, allowing other parts
	// of the app to trigger a refresh of the tag list view.
	refreshTagsFunc func()

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

// ternaryString is a helper function that returns one of two strings based on a boolean condition.
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
	item, err := a.getItemByViewIndex(a.index)
	if err != nil {
		// This is a common case (e.g., empty list), so logging might be too noisy.
		// The caller should handle the nil case gracefully.
		return nil
	}
	return item
}

// getItemByViewIndex retrieves a FileItem from the active view (sequential or random)
// using a specific view index. This is the core data retrieval logic.
func (a *App) getItemByViewIndex(viewIndex int) (*scan.FileItem, error) {
	// 1. Determine the active data sources based on the filter state.
	activeList := &a.images
	activeManager := a.permutationManager

	if a.isFiltered {
		activeManager = a.filteredPermutationManager
		activeList = &a.filteredImages
	}

	// 2. Check for an empty or uninitialized data source.
	if activeList == nil || len(*activeList) == 0 {
		return nil, fmt.Errorf("active list is empty or not initialized")
	}

	// 3. Retrieve the item based on the current mode (random or sequential).
	if a.random {
		if activeManager == nil {
			return nil, fmt.Errorf("random mode is on but PermutationManager is not initialized")
		}
		item, err := activeManager.GetDataByShuffledIndex(viewIndex)
		if err != nil {
			return nil, fmt.Errorf("error getting data for shuffled index %d: %w", viewIndex, err)
		}
		return &item, nil
	}

	// Default to sequential mode retrieval.
	if viewIndex < 0 || viewIndex >= len(*activeList) {
		return nil, fmt.Errorf("sequential index %d out of bounds", viewIndex)
	}
	return &(*activeList)[viewIndex], nil
}

// ViewportItem is a helper struct for the thumbnail strip, bundling an image
// with its index in the current view (shuffled or sequential).
type ViewportItem struct {
	Item      scan.FileItem
	ViewIndex int
}

// getViewportItems returns a slice of ViewportItems representing the current viewport
// for the thumbnail strip, along with the index of the central item within that slice.
func (a *App) getViewportItems(centerIndex int, windowSize int) ([]ViewportItem, int) {
	count := a.getCurrentImageCount()
	if count == 0 {
		return []ViewportItem{}, -1
	}

	halfWindow := windowSize / 2
	start := centerIndex - halfWindow
	end := centerIndex + halfWindow

	// Adjust viewport if it goes out of bounds.
	if start < 0 {
		end -= start // equivalent to end += abs(start)
		start = 0
	}
	if end >= count {
		start -= (end - (count - 1))
		end = count - 1
	}
	// Final check in case the list is smaller than the window.
	if start < 0 {
		start = 0
	}

	items := make([]ViewportItem, 0, end-start+1)
	for i := start; i <= end; i++ {
		item, err := a.getItemByViewIndex(i)
		if err == nil && item != nil {
			items = append(items, ViewportItem{Item: *item, ViewIndex: i})
		}
	}

	newCenterIndex := centerIndex - start
	return items, newCenterIndex
}

// updateStatusBar updates the text of the status bar.
func (a *App) updateStatusBar() {
	if a.UI.statusPathLabel == nil {
		return
	}
	currentItem := a.getCurrentItem()
	statusText := "Ready"

	if currentItem != nil {
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

// updateInfoText generates and displays the markdown-formatted metadata for the
// current image in the info panel, including stats, tags, and EXIF data.
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
	item := a.getCurrentItem()
	if item == nil {
		return ""
	}
	imagePath := item.Path
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
			a.updateInfoText(imgInfo)
			a.refreshThumbnailStrip() // Update the thumbnail strip
		})
	}(imagePath) // Pass the path and flag to the goroutine
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
			a.applyFilter([]string{selectedOption})
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
func (a *App) applyFilter(tags []string) { // Changed signature to accept multiple tags
	if len(tags) == 0 { // If no tags are provided, clear the filter
		a.clearFilter()
		return
	}
	a.addLogMessage(fmt.Sprintf("Applying filter for tags: %s", strings.Join(tags, ", ")))

	// Start with a map of image paths that have the first tag
	// map[imagePath]FileItem
	currentFilteredPaths := make(map[string]scan.FileItem)

	// Get images for the first tag
	firstTagImages, err := a.Service.ListImagesForTag(tags[0])
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tags[0], err), a.UI.MainWin)
		a.clearFilter() // Revert if error occurs
		return
	}
	if len(firstTagImages) == 0 {
		a.addLogMessage(fmt.Sprintf("No images found with tag '%s'. Clearing filter.", tags[0]))
		a.clearFilter()
		return
	}

	// Populate initial set with FileItems from a.images
	// This requires iterating through a.images to get the full FileItem objects
	// for the paths returned by the service.
	for _, item := range a.images {
		for _, path := range firstTagImages {
			if item.Path == path {
				currentFilteredPaths[item.Path] = item
				break
			}
		}
	}

	// Intersect with subsequent tags
	for i := 1; i < len(tags); i++ {
		tag := tags[i]
		nextTagImages, err := a.Service.ListImagesForTag(tag)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tag, err), a.UI.MainWin)
			a.clearFilter()
			return
		}

		// Create a new map for the intersection
		intersectedPaths := make(map[string]scan.FileItem)
		for _, path := range nextTagImages {
			if item, ok := currentFilteredPaths[path]; ok {
				intersectedPaths[path] = item
			}
		}
		currentFilteredPaths = intersectedPaths // Update for next iteration
		if len(currentFilteredPaths) == 0 {
			a.addLogMessage(fmt.Sprintf("No images found with all selected tags. Clearing filter. : %s", strings.Join(tags, ", ")))
			a.clearFilter()
			return
		}
	}

	// Convert the map of FileItems back to a slice (scan.FileItems)
	var newFilteredImages scan.FileItems
	// To maintain original order as much as possible, iterate through a.images
	// and pick only those present in currentFilteredPaths.
	for _, item := range a.images {
		if _, ok := currentFilteredPaths[item.Path]; ok {
			newFilteredImages = append(newFilteredImages, item)
		}
	}

	if len(newFilteredImages) == 0 {
		a.addLogMessage(fmt.Sprintf("No currently loaded images match all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
		a.clearFilter()
		return
	}

	a.filteredImages = newFilteredImages
	a.filteredPermutationManager = scan.NewPermutationManager(&a.filteredImages)
	a.isFiltered = true
	a.currentFilterTag = strings.Join(tags, ", ") // Store all tags for display
	a.index = 0
	a.addLogMessage(fmt.Sprintf("Filter active: %d images with tags '%s'.", len(a.filteredImages), a.currentFilterTag))

	a.loadAndDisplayCurrentImage() // Display the first image in the filtered set
	a.refreshThumbnailStrip()      // Update the thumbnail strip

}

// navigateToIndex sets the current image to a specific index, resets the navigation
// queue, and loads the image. It's a central helper for direct jumps.
func (a *App) navigateToIndex(newIndex int) {
	count := a.getCurrentImageCount()
	if count == 0 || newIndex < 0 || newIndex >= count {
		return // Do nothing if the list is empty or the index is out of bounds.
	}

	a.index = newIndex
	a.loadAndDisplayCurrentImage()
}

// navigateToImageIndex handles a direct jump to a specific image index,
// for example, from a thumbnail click. It preserves the navigation queue
// in random mode where possible by rotating it.
func (a *App) navigateToImageIndex(targetIndex int) {
	count := a.getCurrentImageCount()
	if count == 0 || targetIndex < 0 || targetIndex >= count {
		return // Invalid index
	}

	a.index = targetIndex

	a.loadAndDisplayCurrentImage()
}

// _clearFilterState resets the application's filter state variables without triggering
// a navigation. This is a helper for operations like history navigation that need
// to make an image visible without changing the current view.
func (a *App) _clearFilterState() {
	if !a.isFiltered {
		return
	}
	a.isFiltered = false
	a.currentFilterTag = ""
	a.filteredImages = nil
	a.filteredPermutationManager = nil
}

// clearFilter removes any active tag filter and navigates to the first image.
func (a *App) clearFilter() {
	if !a.isFiltered {
		return // Nothing to clear
	}
	a.addLogMessage("Filter cleared. Showing all images.")
	a._clearFilterState()
	a.navigateToIndex(0)
	a.refreshThumbnailStrip() // Update the thumbnail strip
}

func (a *App) firstImage() {
	if a.getCurrentImageCount() == 0 {
		return
	} // Add check
	a.index = 0
	a.loadAndDisplayCurrentImage()
}

func (a *App) lastImage() {
	a.navigateToIndex(a.getCurrentImageCount() - 1)
}

// navigate moves the current image by a given offset.
// A positive offset moves forward, a negative offset moves backward sequentially.
// It dispatches to more specific handlers based on the offset.
func (a *App) navigate(offset int) {
	count := a.getCurrentImageCount()
	if count == 0 {
		return
	}

	newIndex := a.index + offset

	// Handle wrapping around the end of the list
	if newIndex >= count {
		newIndex = 0 // Wrap to the start
	}
	// Handle wrapping around the beginning of the list
	if newIndex < 0 {
		newIndex = count - 1 // Wrap to the end
	}

	a.index = newIndex
	a.loadAndDisplayCurrentImage()
}

// ShowPreviousImage handles the "back" button logic.
// In random mode, it uses the viewing history.
// In sequential mode, it navigates to the previous image in the list.
func (a *App) ShowPreviousImage() {
	// --- Pause slideshow if it's playing (user is navigating back) ---
	if !a.slideshowManager.IsPaused() {
		a.togglePlay() // This effectively pauses it via user action
	}

	// In sequential mode, "Previous" simply means going to the prior image in the list.
	a.navigate(-1)
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
	// Rebuild the main index manager as the underlying data has changed.
	if a.permutationManager != nil {
		a.permutationManager = scan.NewPermutationManager(&a.images)
	}

	// 4. Remove from the filtered list (a.filteredImages) if filtering is active
	if a.isFiltered {
		newFiltered := a.filteredImages[:0]
		for _, item := range a.filteredImages {
			if item.Path != deletedPath {
				newFiltered = append(newFiltered, item)
			}
		}
		// Rebuild the filtered index manager.
		if a.filteredPermutationManager != nil {
			a.filteredPermutationManager = scan.NewPermutationManager(&newFiltered)
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
	// Common call after index adjustment
	a.loadAndDisplayCurrentImage()
	a.refreshThumbnailStrip() // Update the thumbnail strip
}

// func pathToURI(path string) (fyne.URI, error) {
// 	absPath, _ := filepath.Abs(path)
// 	fileURI := storage.NewFileURI(absPath)
// 	return fileURI, nil
// }

// loadImages scans the given root directory for image files in a background goroutine
// and populates the main image list.
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

// init initializes the application's core components, including the history manager,
// slideshow manager, and other configuration settings based on provided flags.
func (a *App) init(slideshowIntervalSec float64, skipNum int) {
	a.img = Img{EXIFData: make(map[string]string)} // Initialize EXIFData

	// Define a logger function for SlideshowManager
	// This closure captures 'a' (the App instance).
	slideshowLogger := func(message string) {
		// Ensure UI updates from logs happen on the Fyne goroutine.
		// a.addLogMessage itself uses a.logUIManager which updates UI.
		fyne.Do(func() { a.addLogMessage(fmt.Sprintf("Slideshow: %s", message)) })
	}

	a.skipCount = skipNum
	a.slideshowManager = slideshow.NewSlideshowManager(time.Duration(slideshowIntervalSec*1000)*time.Millisecond, slideshowLogger) //nolint:durationcheck
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
			a.UI.pauseAction.SetIcon(theme.MediaPlayIcon()) // Play icon for paused state
		}
	} else {
		// Now playing (not paused), so button should offer to pause
		if a.UI.pauseAction != nil { // Check if pauseAction is initialized
			a.UI.pauseAction.SetIcon(theme.MediaPauseIcon()) // Pause icon for playing state
		}
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
	a.updateStatusBar()
}

// getDiceIcon returns the appropriate dice icon resource based on random mode and current theme.
func (a *App) getDiceIcon() fyne.Resource {
	if a.random {
		if a.isDarkTheme {
			return resourceDiceDark24Png
		}
		return resourceDice24Png
	} else {
		if a.isDarkTheme {
			return resourceDiceDisabledDark24Png
		}
		return resourceDiceDisabled24Png
	}
}

func (a *App) toggleRandom() {
	// 1. Get the currently displayed item before changing mode.
	currentItem := a.getCurrentItem()

	// 2. Toggle the mode state.
	a.random = !a.random
	if a.UI.randomAction != nil {
		a.UI.randomAction.SetIcon(a.getDiceIcon())
	}

	// 3. If there's no current item, just reset the index and exit.
	if currentItem == nil {
		a.index = 0
	} else {
		// 4. Preserve the current image by finding its index in the new view.
		currentPath := currentItem.Path
		newIndex := -1
		activeList := a.getCurrentList() // This now reflects the list for the new mode.

		// Find the item's sequential index in the active list.
		sequentialIndexInList := -1
		for i, item := range activeList {
			if item.Path == currentPath {
				sequentialIndexInList = i
				break
			}
		}

		if sequentialIndexInList == -1 {
			// Fallback: if item not found (should be rare), reset to 0.
			a.addLogMessage(fmt.Sprintf("Could not find item %s in new view. Resetting.", filepath.Base(currentPath)))
			a.index = 0
		} else {
			if a.random { // Switched TO random mode
				var activeManager *scan.PermutationManager
				if a.isFiltered {
					activeManager = a.filteredPermutationManager
				} else {
					activeManager = a.permutationManager
				}

				if activeManager != nil {
					shuffledIndex, err := activeManager.GetShuffledIndex(sequentialIndexInList)
					if err == nil {
						newIndex = shuffledIndex
					}
				}
			} else { // Switched TO sequential mode
				newIndex = sequentialIndexInList
			}
			a.index = newIndex
		}
	}

	// 5. Refresh UI with the updated state.
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
	a.loadAndDisplayCurrentImage() // Reload to show the image at the new (or preserved) index.
	a.refreshThumbnailStrip()      // Re-evaluate and display thumbnails based on the new random state
}

// toggleTheme switches between the light and dark application themes.
func (a *App) toggleTheme() {
	a.isDarkTheme = !a.isDarkTheme
	if a.isDarkTheme {
		a.app.Settings().SetTheme(NewSmallTabsTheme(theme.DarkTheme())) // Apply dark theme
	} else {
		a.app.Settings().SetTheme(NewSmallTabsTheme(theme.LightTheme())) // Apply light theme
	}

	// Update theme-dependent icons
	if a.UI.randomAction != nil {
		a.UI.randomAction.SetIcon(a.getDiceIcon())
	}
	// No need to refresh toolbar here, SetTheme usually triggers a full UI refresh.
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

	ui := &App{app: a}

	// Set initial theme
	ui.isDarkTheme = true // Default to dark theme
	a.Settings().SetTheme(NewSmallTabsTheme(theme.DarkTheme()))
	// Ensure initial random icon matches initial theme
	if ui.UI.randomAction != nil { // This might be nil if called too early, but buildToolbar will set it.
		ui.UI.randomAction.SetIcon(ui.getDiceIcon())
	}

	// Define the logger function that TagDB will use.
	// This closure captures the 'ui' variable (*App instance).
	appLoggerFunc := func(message string) {
		if ui.logUIManager != nil {
			// Ensure UI updates are on the main Fyne thread.
			fyne.Do(func() {
				ui.logUIManager.AddLogMessage(message)
			})
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
	ui.init(*slideshowIntervalFlag, *skipCountFlag) // Pass parsed flags to init
	ui.random = false

	ui.UI.clockLabel = widget.NewLabel("Time: ")
	ui.UI.infoText = widget.NewRichTextFromMarkdown("# Info\n---\n")

	// Status bar will be initialized in buildMainUI
	ui.UI.MainWin.SetContent(ui.buildMainUI())

	go ui.loadImages(dir)

	ui.UI.MainWin.CenterOnScreen()
	ui.UI.MainWin.SetFullScreen(true)

	// Wait for initial scan
	startTime := time.Now()
	for ui.imageCount() < 1000 {
		if time.Since(startTime) > 10*time.Second { // Timeout
			ui.addLogMessage("Timeout waiting for images to load. Please check the directory.")
			// No images loaded, so the UI will reflect this.
			break
		}
		time.Sleep(time.Second) // Slightly longer sleep
	}

	// Check if images were actually loaded
	if ui.imageCount() > 0 {
		// Initialize the permutation manager (for random mode)
		ui.permutationManager = scan.NewPermutationManager(&ui.images)
		// Start at the beginning of the current view (sequential or random).
		ui.index = 0
		ticker := time.NewTicker(ui.slideshowManager.Interval())
		go ui.pauser(ticker) // pauser will call loadAndDisplayCurrentImage via fyne.Do
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
				a.navigate(1)
			})
		}
	}
}

// removeTagGlobally initiates the process of removing a specific tag from all
// images in the database.
func (a *App) removeTagGlobally(tag string) error {
	if tag == "" {
		return nil
	}
	a.addLogMessage(fmt.Sprintf("Global removal for tag '%s' started.", tag))
	successes, errors, err := a.Service.RemoveTagGlobally(tag)
	a.addLogMessage(fmt.Sprintf("Global removal for '%s': %d successes, %d errors.", tag, successes, errors))
	return err
}

// postOperationUpdate handles common UI feedback after a tag operation completes.
// It shows an error dialog if needed, logs status messages, and refreshes relevant UI parts
// like the tag list and the current image's info panel if it was affected.
// - errOp: The error returned from the operation, if any.
// - statusMessage: A message to be logged for the user.
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

// handleTagOperation provides a generic framework for creating and managing a tag operation dialog.
// It handles pausing/resuming the slideshow, setting up the dialog with custom form items,
// and executing a callback function. This reduces boilerplate for `addTag` and `removeTag`.
// - title, verb: Strings for the dialog window title and confirm button.
// - formItems: The custom widgets to display in the dialog's form.
// - focusableWidget: The widget that should receive focus when the dialog appears.
// - preDialogCheck: An optional function that can prevent the dialog from showing.
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

	preDialogCheck := func() bool {
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

	a.handleTagOperation("Remove Tag", "Remove", formItems, nil, preDialogCheck, execute)
}
