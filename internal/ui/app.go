// Package ui  Setup for the FySlide Application
package ui

import (
	"flag"
	"fmt"
	"fyslide/internal/history"
	"fyslide/internal/scan"
	"fyslide/internal/slideshow" // Import the new package
	"fyslide/internal/tagging"
	"image"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	//"fyne.io/fyne/v2/data/binding"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Img struct
type Img struct {
	OriginalImage image.Image
	EditedImage   *image.RGBA
	Path          string
	Directory     string
}

// UI struct
type UI struct {
	MainWin    fyne.Window
	mainModKey fyne.KeyModifier

	split      *container.Split
	clockLabel *widget.Label
	infoText   *widget.RichText

	//ribbonBar *fyne.Container
	// pauseBtn     *widget.Button
	// removeTagBtn *widget.Button
	// tagBtn       *widget.Button
	// randomBtn    *widget.Button

	toolBar      *widget.Toolbar
	randomAction *widget.ToolbarAction
	pauseAction  *widget.ToolbarAction

	contentStack     *fyne.Container   // ADDED: To hold the main views
	imageContentView fyne.CanvasObject // ADDED: Holds the image view (split)
	tagsContentView  fyne.CanvasObject // ADDED: Holds the tags view content
	// --- Status Bar Elements ---
	statusBar        *fyne.Container // Changed from *widget.Label to *fyne.Container
	statusPathLabel  *widget.Label   // For file path and image count
	statusLogLabel   *widget.Label   // For log messages
	statusLogUpBtn   *widget.Button
	statusLogDownBtn *widget.Button
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
	image          *canvas.Image

	historyManager      *history.HistoryManager // Manages navigation history
	isNavigatingHistory bool                    // True if DisplayImage is called from a history action

	slideshowManager *slideshow.SlideshowManager // NEW: Use SlideshowManager
	direction        int

	random bool

	tagDB *tagging.TagDB // Add the tag database instance

	isFiltered       bool   // NEW: Flag to indicate if filtering is active
	currentFilterTag string // NEW: The tag currently being filtered by

	refreshTagsFunc func() // This will hold the function returned by buildTagsTab

	skipCount int // NEW: Configurable skip count for PageUp/PageDown

	// --- Log Bar ---
	logMessages     []string // To store log messages for the status bar
	currentLogIndex int      // Index of the currently displayed log message (-1 if empty, 0 to len-1 otherwise)
	maxLogMessages  int      // Maximum number of log messages to store

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

func parseURL(urlStr string) *url.URL {
	link, err := url.Parse(urlStr)
	if err != nil {
		fyne.LogError("Could not parse URL", err)
	}
	return link

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
	path := a.GetImageFullPath()
	statusText := "Ready"
	if currentItem != nil && path != "" {
		statusText = fmt.Sprintf("%s  |  Image %d / %d", path, a.index+1, a.getCurrentImageCount()) // Shorter path
		if a.isFiltered {
			statusText += fmt.Sprintf(" (Filtered: %s)", a.currentFilterTag)
		}
	}
	statusText += ternaryString(a.slideshowManager.IsPaused(), " | Paused", " | Playing")
	a.UI.statusPathLabel.SetText(statusText) // Update only the path label
}

// addLogMessage adds a message to the UI log display.
func (a *App) addLogMessage(message string) {
	// Also log to console for development/debugging purposes
	//log.Printf("UI_LOG: %s", message)

	if a.UI.statusLogLabel == nil { // UI not fully initialized
		return
	}

	a.logMessages = append(a.logMessages, message)
	if len(a.logMessages) > a.maxLogMessages {
		// Keep only the latest maxLogMessages
		a.logMessages = a.logMessages[len(a.logMessages)-a.maxLogMessages:]
	}
	a.currentLogIndex = len(a.logMessages) - 1 // Point to the newest message
	a.updateLogDisplay()
}

// updateLogDisplay refreshes the status bar log label and button states.
func (a *App) updateLogDisplay() {
	if a.UI.statusLogLabel == nil || a.UI.statusLogUpBtn == nil || a.UI.statusLogDownBtn == nil {
		return // UI not ready
	}

	if len(a.logMessages) == 0 {
		a.UI.statusLogLabel.SetText("")
		a.UI.statusLogUpBtn.Disable()
		a.UI.statusLogDownBtn.Disable()
		return
	}

	// Ensure currentLogIndex is valid
	if a.currentLogIndex < 0 {
		a.currentLogIndex = 0
	} else if a.currentLogIndex >= len(a.logMessages) {
		a.currentLogIndex = len(a.logMessages) - 1
	}

	a.UI.statusLogLabel.SetText(fmt.Sprintf("[%d/%d] %s", a.currentLogIndex+1, len(a.logMessages), a.logMessages[a.currentLogIndex]))
	if a.currentLogIndex <= 0 {
		a.UI.statusLogUpBtn.Disable()
	} else {
		a.UI.statusLogUpBtn.Enable()
	}
	if a.currentLogIndex >= len(a.logMessages)-1 {
		a.UI.statusLogDownBtn.Disable()
	} else {
		a.UI.statusLogDownBtn.Enable()
	}

}

// updateInfoText fetches current image info and tags, then updates the infoText widget.
func (a *App) updateInfoText() {
	currentItem := a.getCurrentItem() // Use helper to get current item safely

	if currentItem == nil || a.img.Path == "" { // Check if item exists and path is set
		a.UI.infoText.ParseMarkdown("# Info\n---\nNo image loaded.")
		return
	}

	count := a.getCurrentImageCount() // Use helper

	// --- Use FileInfo from the scanned item ---
	fileInfo := currentItem.Info // OPTIMIZATION: Use existing FileInfo
	if fileInfo == nil {
		// Fallback or handle error if FileInfo wasn't stored during scan
		a.addLogMessage(fmt.Sprintf("updateInfoText: Warning - FileInfo missing for %s", a.img.Path))
		// Optionally try os.Stat as a fallback, or show an error
		var err error
		fileInfo, err = os.Stat(a.img.Path)
		if err != nil {
			a.addLogMessage(fmt.Sprintf("updateInfoText: Fallback os.Stat failed for %s: %v", a.img.Path, err))
			a.UI.infoText.ParseMarkdown(fmt.Sprintf("## Error\nCould not get file stats for %s", a.img.Path))
			return
		}
	}
	// --- End Optimization ---

	// Get image dimensions (assuming a.img.OriginalImage is still valid from DisplayImage)
	imgWidth := 0
	imgHeight := 0
	if a.img.OriginalImage != nil {
		imgWidth = a.img.OriginalImage.Bounds().Max.X
		imgHeight = a.img.OriginalImage.Bounds().Max.Y
	}

	// --- Get Tags ---
	currentTags, errTags := a.tagDB.GetTags(a.img.Path)
	if errTags != nil {
		// Log the error, but continue to display other info
		a.addLogMessage(fmt.Sprintf("Error getting tags for %s: %v", a.img.Path, errTags))
	}
	tagsString := "(none)" // Default if no tags or error occurred
	// Only join if no error occurred and tags exist
	if errTags == nil && len(currentTags) > 0 {
		tagsString = strings.Join(currentTags, ", ")
	}

	// --- Build Markdown ---
	filterStatus := ""
	if a.isFiltered {
		filterStatus = fmt.Sprintf("\n**Filter Active:** %s\n", a.currentFilterTag)
	}

	md := fmt.Sprintf(`## Stats
%s
**Num:** %d

**Total:** %d

**Size:**   %d bytes

**Width:**   %dpx

**Height:**  %dpx

**Last modified:** %s

---
## Tags
%s
`, // Added separator and tags section
		filterStatus, // Add filter status
		a.index+1,    // Display 1-based index
		count,        // Use current count
		fileInfo.Size(), imgWidth, imgHeight, fileInfo.ModTime().Format("2006-01-02"),
		tagsString, // Add the formatted tags string here
	)

	// --- Update Widget ---
	a.UI.infoText.ParseMarkdown(md)
	// Optional: Scroll to top if content is long
	// if scroller, ok := a.UI.infoText.Parent().(*container.Scroll); ok {
	//     scroller.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -10000}}) // Scroll up significantly
	// }
}

// handleImageDisplayError is a helper to set the UI state when an image fails to load or decode.
// formatName is optional and only used if errorType is "Decoding".
func (a *App) handleImageDisplayError(imagePath, errorType string, originalError error, formatName string) {
	a.image.Image = nil
	a.img = Img{Path: imagePath} // Keep path for context
	a.image.Refresh()
	a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - Error %s %s", errorType, filepath.Base(imagePath)))
	a.updateInfoText()
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
		a.image.Image = nil
		a.img = Img{}
		a.image.Refresh()
		a.UI.MainWin.SetTitle("FySlide")
		a.updateStatusBar()
		a.updateInfoText()
		a.addLogMessage("No images available.")
		return // Exit the function, no image to load
	}

	if a.random && !a.isNavigatingHistory {
		if count == 1 {
			a.index = 0
		} else if count > 1 { // count is already guaranteed > 0 here
			randomNumber := rand.Intn(count)
			a.index = randomNumber
		}
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
				a.image.Image = nil
				a.img = Img{}
				a.image.Refresh()
				a.UI.MainWin.SetTitle("FySlide")
				a.updateStatusBar()
				a.updateInfoText()
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
		file, err := os.Open(path)
		if err != nil {
			fyne.Do(func() {
				a.handleImageDisplayError(path, "Loading", err, "")
			})
			return // Exit goroutine
		}
		defer file.Close()

		imageDecoded, formatName, err := image.Decode(file)
		if err != nil {
			fyne.Do(func() {
				a.handleImageDisplayError(file.Name(), "Decoding", err, formatName)
			})
			return // Exit goroutine
		}

		// Successfully decoded image - perform UI updates on the Fyne thread
		fyne.Do(func() {
			a.img.OriginalImage = imageDecoded
			a.img.Path = file.Name() // Update the path in the Img struct
			a.image.Image = a.img.OriginalImage
			a.image.Refresh()

			// Update Title, Status Bar, and Info Text
			a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - %v", a.img.Path))
			a.updateStatusBar()
			a.updateInfoText()

			// History Update (only if not navigating history)
			if a.historyManager != nil && !historyNav {
				a.historyManager.RecordNavigation(a.img.Path)
			}
		})
	}(imagePath, isHistoryNav) // Pass the path and flag to the goroutine
}

// showFilterDialog displays a dialog to select a tag for filtering.
func (a *App) showFilterDialog() {
	allTagsWithCounts, err := a.tagDB.GetAllTags() // This now returns []tagging.TagWithCount
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

// applyFilter filters the image list based on the selected tag.
func (a *App) applyFilter(tag string) {
	a.addLogMessage(fmt.Sprintf("Applying filter for tag: %s", tag))
	tagImagesPaths, err := a.tagDB.GetImages(tag)
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
	a.index = 0     // Reset index to the start of the filtered list
	a.direction = 1 // Default direction
	a.addLogMessage(fmt.Sprintf("Filter active: %d images with tag '%s'.", len(a.filteredImages), tag))

	a.isNavigatingHistory = false  // Applying a filter is a new view, not history navigation
	a.loadAndDisplayCurrentImage() // Display the first image in the filtered set
	a.updateInfoText()             // Update info panel immediately
	a.updateStatusBar()
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
	a.direction = 1

	a.isNavigatingHistory = false  // Clearing a filter is a new view state
	a.loadAndDisplayCurrentImage() // Display the first image in the full set
	a.updateInfoText()             // Update info panel immediately
	a.updateStatusBar()
}

func (a *App) firstImage() {
	a.isNavigatingHistory = false
	if a.getCurrentImageCount() == 0 {
		return
	} // Add check
	a.index = 0
	a.loadAndDisplayCurrentImage()
	a.direction = 1
}

func (a *App) lastImage() {
	a.isNavigatingHistory = false
	count := a.getCurrentImageCount() // Use helper
	if count == 0 {
		return
	} // Add check
	a.index = count - 1
	a.loadAndDisplayCurrentImage()
	a.direction = -1
}

func (a *App) nextImage() {
	count := a.getCurrentImageCount() // Use helper
	if count == 0 {
		return
	} // Add check
	// --- History Navigation Logic ---
	if a.isNavigatingHistory {
		// If currently navigating history (came here from a 'back' action),
		// try to go forward in history first.
		if a.ShowNextImageFromHistory() {
			return // Successfully moved forward in history
		}
		// If ShowNextImageFromHistory returned false, it means we are at the end
		// of the history stack. Fall through to standard next image logic.
	}
	// --- Standard Next Image Logic ---
	a.isNavigatingHistory = false // Ensure this is false for standard navigation

	// Calculate next index based on direction (original logic)
	a.index += a.direction              // This might go out of bounds
	a.index = (a.index + count) % count // Wrap around using modulo

	a.loadAndDisplayCurrentImage() // Display the image at the calculated index

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

	a.loadAndDisplayCurrentImage() // loadAndDisplayCurrentImage will respect a.isNavigatingHistory
	// Error handling is now internal to loadAndDisplayCurrentImage
	a.isNavigatingHistory = false // Reset flag after the operation is complete
}

func (a *App) showPreviousLogMessage() {
	if len(a.logMessages) == 0 || a.currentLogIndex <= 0 {
		return
	}
	a.currentLogIndex--
	a.updateLogDisplay()
}

func (a *App) showNextLogMessage() {
	if len(a.logMessages) == 0 || a.currentLogIndex >= len(a.logMessages)-1 {
		return
	}
	a.currentLogIndex++
	a.updateLogDisplay()
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

	// 1. Remove from OS
	if err := os.Remove(deletedPath); err != nil {
		dialog.ShowError(err, a.UI.MainWin)
		return
	}
	a.addLogMessage(fmt.Sprintf("Deleted file: %s", deletedPath))

	// 2. Remove tags associated with this file from DB
	err := a.tagDB.RemoveAllTagsForImage(deletedPath)
	if err != nil {
		a.addLogMessage(fmt.Sprintf("Warn: Failed to remove tags for deleted file %s: %v", deletedPath, err))
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
		a.index = -1                   // Indicate no valid index
		a.loadAndDisplayCurrentImage() // Will show "No images" state
	} else {
		// Adjust index carefully
		if a.index >= count { // If we deleted the last item
			a.index = count - 1
		}
		// If we deleted an item before the current index, the index is now implicitly correct
		// If we deleted the item *at* the current index, the next item shifts into this index
		// So, just ensure index is within bounds [0, count-1]
		if a.index < 0 {
			a.index = 0
		}

		a.loadAndDisplayCurrentImage() // Display the image at the (potentially adjusted) index
	}
	a.updateInfoText() // Update counts etc.
	a.updateStatusBar()
}

// func pathToURI(path string) (fyne.URI, error) {
// 	absPath, _ := filepath.Abs(path)
// 	fileURI := storage.NewFileURI(absPath)
// 	return fileURI, nil
// }

func (a *App) loadImages(root string) {
	a.images = nil // Clear previous images or a.images = a.images[:0]

	imageChan := scan.Run(root)   // scan.Run now returns a channel
	for item := range imageChan { // Loop until the channel is closed
		a.images = append(a.images, item)
		// Optionally, you could update a progress indicator here
		// if the GUI needs to show loading progress.
	}
	msg := fmt.Sprintf("Loaded %d images from %s", len(a.images), root)
	fyne.Do(func() {
		a.addLogMessage(msg)
	})
}

func (a *App) imageCount() int {
	return len(a.images)
}

func (a *App) init(historyCap int, slideshowIntervalSec float64, skipNum int) {
	a.img = Img{}
	a.historyManager = history.NewHistoryManager(historyCap)
	a.skipCount = skipNum
	a.slideshowManager = slideshow.NewSlideshowManager(time.Duration(slideshowIntervalSec*1000) * time.Millisecond)
	a.isNavigatingHistory = false
	a.maxLogMessages = 100 // Max number of log messages to store in UI
	a.logMessages = make([]string, 0, a.maxLogMessages)
	a.currentLogIndex = -1 // No logs initially

	// SlideshowManager's constructor handles default interval if slideshowIntervalSec is invalid
	// So, no need for a separate check here for a.slideshowInterval.

	if a.skipCount <= 0 {
		log.Printf("Warning: Skip count must be positive. Defaulting to 20. Got: %d", skipNum)
		a.skipCount = 20
	}
}

// Handle toggles
func (a *App) togglePlay() {
	a.slideshowManager.TogglePlayPause() // Toggle state using the manager
	if a.slideshowManager.IsPaused() {
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
	if a.random {
		// Random is ON, show active dice
		if a.UI.randomAction != nil {
			a.UI.randomAction.SetIcon(resourceDice24Png)
		}
	} else {
		// Random is OFF, show disabled dice
		if a.UI.randomAction != nil {
			a.UI.randomAction.SetIcon(resourceDiceDisabled24Png)
		}
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
}

// Command-line flags
var historySizeFlag = flag.Int("history-size", 10, "Number of last viewed images to remember (0 to disable). Min: 0.")
var slideshowIntervalFlag = flag.Float64("slideshow-interval", 2.0, "Slideshow image display interval in seconds. Min: 0.1.")
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
	a.Settings().SetTheme(NewSmallTabsTheme(currentTheme))

	ui := &App{app: a, direction: 1}

	ui.tagDB, err = tagging.NewTagDB("")
	if err != nil {
		log.Fatalf("Failed to initialize tag database: %v", err)
		// For a fatal error like this, log.Fatalf is appropriate as the app can't continue.
		// If we wanted to show a dialog, we'd need the main window, which isn't fully up yet.
	}

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
		ticker := time.NewTicker(ui.slideshowManager.Interval())
		ui.isNavigatingHistory = false // Initial display is not from history
		go ui.pauser(ticker)           // pauser will call loadAndDisplayCurrentImage via fyne.Do
		go ui.updateTimer()
		ui.loadAndDisplayCurrentImage()
	} else {
		// This case is also hit on timeout if no images loaded.
		ui.updateStatusBar() // Will show "No images available" or similar.
		ui.updateInfoText()
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
				a.isNavigatingHistory = false // Standard "next" is not history navigation
				a.nextImage()
			})
		}
	}
}

// removeTagGlobally removes a specific tag from all images in the database.
func (a *App) removeTagGlobally(tag string) error {
	if tag == "" {
		return fmt.Errorf("cannot remove an empty tag")
	}
	a.addLogMessage(fmt.Sprintf("Global removal for tag '%s' started.", tag))

	// 1. Get all images associated with this tag
	imagePaths, err := a.tagDB.GetImages(tag)
	if err != nil {
		// Log the error, but maybe the tag just doesn't exist (which is fine for removal)
		a.addLogMessage(fmt.Sprintf("Error getting images for tag '%s' during global removal: %v", tag, err))
		// Check if it's a "not found" type error if your DB layer provides it.
		// If it's just not found, we can consider it a success (nothing to remove).
		// For BoltDB, GetImages returns an empty list if the tag key doesn't exist, not an error.
		// So, an error here is likely a real DB issue.
		return fmt.Errorf("database error while getting images for tag '%s': %w", tag, err)
	}

	if len(imagePaths) == 0 {
		a.addLogMessage(fmt.Sprintf("Tag '%s' not found or no images associated with it. Global removal complete.", tag))
		return nil // No images had this tag, so removal is effectively done.
	}

	a.addLogMessage(fmt.Sprintf("Found %d images with tag '%s'. Removing...", len(imagePaths), tag))

	// 2. Iterate and remove the tag from each image
	var firstError error
	errorsEncountered := 0
	successfulRemovals := 0

	for _, path := range imagePaths {
		// RemoveTag handles both Image->Tag and Tag->Image mappings.
		// It should also delete the Tag key if the image list becomes empty.
		errRemove := a.tagDB.RemoveTag(path, tag)
		if errRemove != nil {
			a.addLogMessage(fmt.Sprintf("Error removing tag '%s' from image '%s': %v", tag, filepath.Base(path), errRemove))
			errorsEncountered++
			if firstError == nil {
				firstError = fmt.Errorf("failed removing tag '%s' from %s: %w", tag, filepath.Base(path), errRemove)
			}
		} else {
			successfulRemovals++
		}
	}

	a.addLogMessage(fmt.Sprintf("Global removal for '%s': %d successes, %d errors.", tag, successfulRemovals, errorsEncountered))

	// 3. Update UI if the currently displayed image was affected
	// Check if the current image *had* the tag that was just removed
	currentItem := a.getCurrentItem()
	if currentItem != nil {
		// Check if the current item's path was in the list we just processed
		wasAffected := false
		for _, path := range imagePaths {
			if currentItem.Path == path {
				wasAffected = true
				break
			}
		}
		if wasAffected {
			a.addLogMessage(fmt.Sprintf("Current image %s affected by global tag removal.", filepath.Base(currentItem.Path)))
			a.updateInfoText() // Refresh the info panel to show updated tags
		}
	}

	// 4. Return the first error encountered, if any
	return firstError
}

// _addTagsToDirectory is a helper to apply a list of tags to all images in a given directory.
// It uses goroutines for concurrent database operations.
func (a *App) _addTagsToDirectory(tagsToAdd []string, currentDir string,
	wg *sync.WaitGroup, mu *sync.Mutex, firstError *error,
	totalTagsAttempted *int, successfulAdditions *int, errorsEncountered *int, imagesProcessed *int, filesAffected map[string]bool) {

	a.addLogMessage(fmt.Sprintf("Batch tagging directory: %s with [%s]", filepath.Base(currentDir), strings.Join(tagsToAdd, ", ")))

	for _, imageItem := range a.images { // Iterate through the original full list
		// Capture loop variables for the goroutine
		itemPath := imageItem.Path
		currentTagsToAdd := tagsToAdd // Capture for goroutine

		itemDir := filepath.Dir(itemPath)
		if itemDir == currentDir {
			wg.Add(1)
			go func() {
				defer wg.Done()

				localTagsAttemptedOnThisImage := 0
				localSuccessfulAdditionsOnThisImage := 0
				localErrorsOnThisImage := 0
				var localFirstErrorForThisImage error

				for _, tag := range currentTagsToAdd {
					localTagsAttemptedOnThisImage++
					errAdd := a.tagDB.AddTag(itemPath, tag)
					if errAdd != nil {
						// Logged via addLogMessage by the calling function's summary
						localErrorsOnThisImage++
						if localFirstErrorForThisImage == nil {
							localFirstErrorForThisImage = fmt.Errorf("failed to tag %s with '%s': %w", filepath.Base(itemPath), tag, errAdd)
						}
					} else {
						localSuccessfulAdditionsOnThisImage++
						filesAffected[itemPath] = true
					}
				}

				mu.Lock()
				(*imagesProcessed)++
				*totalTagsAttempted += localTagsAttemptedOnThisImage
				*successfulAdditions += localSuccessfulAdditionsOnThisImage
				*errorsEncountered += localErrorsOnThisImage
				if localFirstErrorForThisImage != nil && *firstError == nil {
					*firstError = localFirstErrorForThisImage
				}
				mu.Unlock()
			}()
		}
	}
	wg.Wait() // Wait for all goroutines to finish

	a.addLogMessage(fmt.Sprintf("Batch tagging for [%s] in '%s' complete. Images processed: %d. Attempts: %d, Successes: %d, Errors: %d.",
		strings.Join(tagsToAdd, ", "), filepath.Base(currentDir), *imagesProcessed, *totalTagsAttempted, *successfulAdditions, *errorsEncountered))
}

// addTag shows a dialog to add a new tag to the current image
func (a *App) addTag() {
	if a.img.Path == "" {
		dialog.ShowInformation("Add Tag", "No image loaded to tag.", a.UI.MainWin) // Updated title
		return
	}

	a.slideshowManager.Pause(true)     // Pause for the tagging operation
	if a.slideshowManager.IsPaused() { // Check if it's actually paused now
		a.addLogMessage("Slideshow paused for tagging.")
	}

	currentTags, err := a.tagDB.GetTags(a.img.Path)
	if err != nil {
		a.slideshowManager.ResumeAfterOperation() // Ensure resume on error
		if !a.slideshowManager.IsPaused() {
			a.addLogMessage("Slideshow resumed.")
		}
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
		return
	}

	tagEntry := widget.NewEntry()
	tagEntry.SetPlaceHolder("Enter tag(s) separated by commas...")

	currentTagsLabel := widget.NewLabel(fmt.Sprintf("Current tags: %s", strings.Join(currentTags, ", ")))
	if len(currentTags) == 0 {
		currentTagsLabel.SetText("Current tags: (none)")
	}

	applyToAllCheck := widget.NewCheck("Apply tag(s) to all images in this directory", nil)
	applyToAllCheck.SetChecked(true)

	// Keep the rest of the addTag (formerly tagFile) function body the same...
	dialog.ShowForm("Add Tag", "Add", "Cancel", []*widget.FormItem{
		widget.NewFormItem("", currentTagsLabel), // Display current tags
		widget.NewFormItem("New Tag(s) (comma-separated)", tagEntry),
		widget.NewFormItem("", applyToAllCheck), // --- NEW: Add checkbox to form ---
	}, func(confirm bool) {

		defer func() {
			a.slideshowManager.ResumeAfterOperation()
			if !a.slideshowManager.IsPaused() {
				a.addLogMessage("Slideshow resumed.")
			}
		}()

		if !confirm {
			return
		}

		rawInput := tagEntry.Text
		potentialTags := strings.Split(rawInput, ",")
		var tagsToAdd []string
		uniqueTags := make(map[string]bool) // Use a map to handle duplicates in input
		for _, pt := range potentialTags {
			tag := strings.ToLower(strings.TrimSpace(pt)) // Normalize to lowercase
			if tag != "" && !uniqueTags[tag] {            // Only add non-empty, unique tags
				tagsToAdd = append(tagsToAdd, tag)
				uniqueTags[tag] = true
			}
		}

		if len(tagsToAdd) == 0 {
			dialog.ShowInformation("Add Tag(s)", "No valid tags entered.", a.UI.MainWin)
			return // No valid tags, defer handles resume
		}

		applyToAll := applyToAllCheck.Checked // --- NEW: Get checkbox state ---

		var errAddOp error // Store the first error encountered for the operation
		var statusMessage string
		showMessage := false
		var logMessage string
		filesAffected := make(map[string]bool) // Track files that had tags successfully added
		totalTagsAttempted := 0
		successfulAdditions := 0
		errorsEncountered := 0

		// --- It correctly iterates through the 'tagsToAdd' slice ---
		if applyToAll {
			currentDir := filepath.Dir(a.img.Path)
			var wg sync.WaitGroup
			var mu sync.Mutex // Mutex to protect shared variables
			imagesProcessed := 0
			a.addLogMessage(fmt.Sprintf("Adding tag(s) [%s] to all images in %s...", strings.Join(tagsToAdd, ", "), filepath.Base(currentDir)))
			a._addTagsToDirectory(tagsToAdd, currentDir, &wg, &mu, &errAddOp, &totalTagsAttempted, &successfulAdditions, &errorsEncountered, &imagesProcessed, filesAffected)

			logMessage = fmt.Sprintf("Added tag(s) [%s] to %d images in %s. Successes: %d, Errors: %d",
				strings.Join(tagsToAdd, ", "), imagesProcessed, filepath.Base(currentDir), successfulAdditions, errorsEncountered)

			if errorsEncountered > 0 {
				showMessage = true
				statusMessage = fmt.Sprintf("%d tag(s) applied partially across %d images.\n%d errors occurred (see logs).", len(tagsToAdd), imagesProcessed, errorsEncountered)
			} else if successfulAdditions > 0 { // Only show success if something was actually added
				showMessage = true
				statusMessage = fmt.Sprintf("%d tag(s) applied to %d images in %s.", len(tagsToAdd), len(filesAffected), filepath.Base(currentDir))
			}
		} else {
			// Apply tags only to the current image
			a.addLogMessage(fmt.Sprintf("Adding tag(s) [%s] to %s", strings.Join(tagsToAdd, ", "), filepath.Base(a.img.Path)))
			for _, tag := range tagsToAdd {
				totalTagsAttempted++
				errAdd := a.tagDB.AddTag(a.img.Path, tag)
				if errAdd != nil {
					// Error will be part of errAddOp and shown in dialog / logged
					errorsEncountered++
					if errAddOp == nil {
						errAddOp = fmt.Errorf("failed to add tag '%s': %w", tag, errAdd)
					}
				} else {
					successfulAdditions++
					filesAffected[a.img.Path] = true
				}
			}
			logMessage = fmt.Sprintf("Attempted to apply %d tag(s) to %s. Successes: %d, Errors: %d", len(tagsToAdd), a.img.Path, successfulAdditions, errorsEncountered)
			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("%d tag(s) applied partially.\n%d errors occurred (see logs).", len(tagsToAdd), errorsEncountered)
				showMessage = true // Show message for partial success on single image too
			}
			if successfulAdditions > 0 && errorsEncountered == 0 { // Only show success if something was actually added and no errors
				showMessage = true
				statusMessage = fmt.Sprintf("%d tag(s) applied to current image.", len(tagsToAdd))
			}
		}

		a.addLogMessage(logMessage)

		// --- Common Post-Processing ---
		if errAddOp != nil {
			// Show the first error encountered
			dialog.ShowError(errAddOp, a.UI.MainWin) // Simplified error message
			a.addLogMessage(fmt.Sprintf("Error adding tags: %v", errAddOp))
		} else {
			// No critical error, logMessage already added by a.addLogMessage
		}
		if len(filesAffected) > 0 { // Only update info/refresh if something actually changed
			a.updateInfoText() // Update info panel for the current image
			if a.refreshTagsFunc != nil {
				// log.Println("Calling Tags tab refresh function.") // Developer log, not for UI
				a.refreshTagsFunc()
			} else {
				a.addLogMessage("Warning: Tags tab refresh function not set.")
			}
		}
		if showMessage { // This is for partial success messages or full success
			// dialog.ShowInformation("Tagging Status", statusMessage, a.UI.MainWin)
			// Replace dialog with status bar message
			a.addLogMessage(fmt.Sprintf("Tagging Status: %s", statusMessage))
		}
	}, a.UI.MainWin)
}

// removeTag shows a dialog to remove an existing tag from the current image,
// with an option to remove it from all images in the same directory.
func (a *App) removeTag() {
	if a.img.Path == "" {
		dialog.ShowInformation("Remove Tag", "No image loaded to remove tags from.", a.UI.MainWin)
		return
	}

	a.slideshowManager.Pause(true) // Pause for the operation
	if a.slideshowManager.IsPaused() {
		a.addLogMessage("Slideshow paused for tag removal.")
	}

	// 1. Get current tags for the image to populate the selector
	currentTags, err := a.tagDB.GetTags(a.img.Path)
	if err != nil {
		a.slideshowManager.ResumeAfterOperation()
		if !a.slideshowManager.IsPaused() {
			a.addLogMessage("Slideshow resumed.")
		}
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
		return
	}

	// 2. Check if there are any tags to remove
	if len(currentTags) == 0 {
		a.slideshowManager.ResumeAfterOperation()
		if !a.slideshowManager.IsPaused() {
			a.addLogMessage("Slideshow resumed.")
		}
		dialog.ShowInformation("Remove Tag", "This image has no tags to remove.", a.UI.MainWin)
		return
	}

	// 3. Prepare UI for tag selection
	var selectedTag string
	tagSelector := widget.NewSelect(currentTags, func(selected string) {
		selectedTag = selected
	})
	// Pre-select the first tag to avoid issues if the user confirms without selecting
	tagSelector.SetSelected(currentTags[0])
	selectedTag = currentTags[0] // Initialize selectedTag

	// --- NEW: Checkbox for removing from all in directory ---
	removeFromAllCheck := widget.NewCheck("Remove tag from all images in this directory", nil)
	// --- End NEW ---

	// 4. Show the removal dialog
	dialog.ShowForm("Remove Tag", "Remove", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Select Tag to Remove", tagSelector),
		widget.NewFormItem("", removeFromAllCheck), // --- NEW: Add checkbox to form ---
	}, func(confirm bool) {
		defer func() {
			a.slideshowManager.ResumeAfterOperation()
			if !a.slideshowManager.IsPaused() {
				a.addLogMessage("Slideshow resumed.")
			}
		}()
		if !confirm || selectedTag == "" {
			return // User cancelled or somehow didn't select a tag
		}

		removeFromAll := removeFromAllCheck.Checked // --- NEW: Get checkbox state ---

		var errRemoveOp error    // Use a local error variable for the operation
		var statusMessage string // For success or partial success
		var logMessage string
		imagesUntaggedCount := 0
		errorsEncountered := 0

		if removeFromAll {
			// --- NEW: Logic to remove tag from all images in the directory ---
			currentDir := filepath.Dir(a.img.Path)
			a.addLogMessage(fmt.Sprintf("Removing tag '%s' from all images in %s...", selectedTag, filepath.Base(currentDir)))

			var wg sync.WaitGroup
			var mu sync.Mutex

			for _, item := range a.images { // Iterate through the original full list
				// Capture loop variables for the goroutine
				itemPath := item.Path
				tagToRemove := selectedTag

				itemDir := filepath.Dir(item.Path)
				if itemDir == currentDir {
					wg.Add(1)
					go func() {
						defer wg.Done()
						errRemove := a.tagDB.RemoveTag(itemPath, tagToRemove)

						mu.Lock()
						defer mu.Unlock()

						if errRemove != nil {
							// Error will be part of errRemoveOp and shown in dialog / logged
							errorsEncountered++
							if errRemoveOp == nil {
								errRemoveOp = fmt.Errorf("failed to untag %s: %w", filepath.Base(itemPath), errRemove)
							}
						} else {
							imagesUntaggedCount++
						}
					}()
				}
			}
			wg.Wait() // Wait for all goroutines to finish

			if imagesUntaggedCount > 0 || errorsEncountered > 0 { // Only log if something was attempted
				logMessage = fmt.Sprintf("Removed tag '%s'. Images untagged: %d in %s. Errors: %d.",
					selectedTag, imagesUntaggedCount, filepath.Base(currentDir), errorsEncountered)
			} else {
				logMessage = fmt.Sprintf("No images in directory %s found requiring removal of tag '%s'.", filepath.Base(currentDir), selectedTag)
			}
			a.addLogMessage(logMessage)

			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Tag '%s' removal attempted. %d images untagged.\n%d errors occurred (see logs).", selectedTag, imagesUntaggedCount, errorsEncountered)
			} else if imagesUntaggedCount > 0 {
				statusMessage = fmt.Sprintf("Tag '%s' removed from %d images in directory %s.", selectedTag, imagesUntaggedCount, filepath.Base(currentDir))
			}
		} else {
			// Original logic: Remove only from the current image
			errRemoveOp = a.tagDB.RemoveTag(a.img.Path, selectedTag)
			if errRemoveOp == nil {
				imagesUntaggedCount = 1 // Only one attempt
				logMessage = fmt.Sprintf("Removed tag '%s' from %s", selectedTag, filepath.Base(a.img.Path))
				a.addLogMessage(logMessage)
				statusMessage = fmt.Sprintf("Tag '%s' removed from current image.", selectedTag)
			}
		}

		if errRemoveOp != nil {
			dialog.ShowError(fmt.Errorf("failed to remove tag '%s': %w", selectedTag, errRemoveOp), a.UI.MainWin)
			a.addLogMessage(fmt.Sprintf("Error removing tag '%s': %v", selectedTag, errRemoveOp))
		} else {
			if statusMessage != "" { // Show success or partial success summary
				// dialog.ShowInformation("Tag Removal Status", statusMessage, a.UI.MainWin)
				a.addLogMessage(fmt.Sprintf("Tag Removal Status: %s", statusMessage))
			}
			if a.refreshTagsFunc != nil && imagesUntaggedCount > 0 {
				a.refreshTagsFunc()
			}
			a.updateInfoText()
		}
	}, a.UI.MainWin)
}
