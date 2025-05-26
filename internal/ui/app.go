// Package ui  Setup for the FySlide Application
package ui

import (
	"flag"
	"fmt"
	"fyslide/internal/scan"
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

	// --- New History Fields ---
	historyStack        []string // Stores paths of viewed images
	currentHistoryIndex int      // Points to the current image in historyStack (-1 if empty or before first)
	historyCapacity     int      // Max number of images to remember
	isNavigatingHistory bool     // True if DisplayImage is called from a history action

	paused    bool
	direction int

	random bool

	tagDB *tagging.TagDB // Add the tag database instance

	isFiltered       bool   // NEW: Flag to indicate if filtering is active
	currentFilterTag string // NEW: The tag currently being filtered by

	refreshTagsFunc func() // This will hold the function returned by buildTagsTab
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
		log.Printf("updateInfoText: Warning - FileInfo missing for %s", a.img.Path)
		// Optionally try os.Stat as a fallback, or show an error
		var err error
		fileInfo, err = os.Stat(a.img.Path)
		if err != nil {
			log.Printf("updateInfoText: Fallback os.Stat failed for %s: %v", a.img.Path, err)
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
		log.Printf("Error getting tags for %s: %v", a.img.Path, errTags)
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
		log.Printf("Error %s image '%s' (format detected: %s): %v", errorType, imagePath, formatName, originalError)
	} else {
		log.Printf("Error %s image '%s': %v", errorType, imagePath, originalError)
	}
}

// DisplayImage displays the image on the canvas at the current index
func (a *App) DisplayImage() error {
	// decode and update the image + get image path
	var err error
	currentList := a.getCurrentList() // Use helper
	count := a.getCurrentImageCount() // Use helper

	if count == 0 { // Handle empty list (either full or filtered)
		a.image.Image = nil
		a.img = Img{}
		a.image.Refresh()
		a.UI.MainWin.SetTitle("FySlide")
		a.updateInfoText()
		// a.UI.tagBtn.Disable()
		// a.UI.removeTagBtn.Disable()
		return fmt.Errorf("no images available in the current list")
	}

	if a.random && !a.isNavigatingHistory {
		if count == 1 {
			a.index = 0
		} else if count > 1 { // count is already guaranteed > 0 here
			randomNumber := rand.Intn(count)
			a.index = randomNumber
		}
	}

	// Check index bounds again after potential random selection or if not random
	if a.index < 0 || a.index >= count { // Use current count
		// This might happen if images were deleted; try to reset index or handle error
		a.index = 0     // Reset to first image
		if count == 0 { // Double check after reset attempt
			// Already handled above, but defensive check
			return fmt.Errorf("image index out of bounds and no images available")
		}
	}

	imagePath := currentList[a.index].Path // Get path from current list

	file, err := os.Open(imagePath) // Use imagePath
	if err != nil {
		a.handleImageDisplayError(imagePath, "Loading", err, "") // No formatName for loading errors
		// Keep buttons enabled to allow navigation away from the error
		// a.UI.tagBtn.Enable() // Can still tag/untag even if load failed
		// a.UI.removeTagBtn.Enable()
		return fmt.Errorf("unable to open image '%s': %w", imagePath, err)
	}
	defer file.Close()
	imageDecoded, formatName, err := image.Decode(file)
	if err != nil {
		a.handleImageDisplayError(file.Name(), "Decoding", err, formatName)
		// a.UI.tagBtn.Enable()
		// a.UI.removeTagBtn.Enable()
		return fmt.Errorf("unable to decode image %s: %w", file.Name(), err)
	}

	// Successfully decoded image
	a.img.OriginalImage = imageDecoded
	a.img.Path = file.Name()
	a.image.Image = a.img.OriginalImage
	a.image.Refresh()

	// --- Update Title, Status Bar, and Info Text ---
	a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - %v", filepath.Base(a.img.Path)))
	// Update status label based on filter state

	a.updateInfoText() // Call the function to update the info panel

	// --- Ensure buttons are enabled (if count > 0) ---
	// a.UI.tagBtn.Enable()
	// a.UI.removeTagBtn.Enable()
	// --- History Update ---
	if a.historyCapacity > 0 && !a.isNavigatingHistory {
		// If currentHistoryIndex is not at the end of the stack (e.g., user went back, then chose a new path),
		// truncate the "future" part of history.
		if a.currentHistoryIndex != -1 && a.currentHistoryIndex < len(a.historyStack)-1 {
			a.historyStack = a.historyStack[:a.currentHistoryIndex+1]
		}

		// Add current image to history.
		// Avoid adding if it's the exact same path as the last entry AND we are at the end of history.
		// This prevents duplicates if DisplayImage is called for a refresh without actual navigation.
		// If we branched (currentHistoryIndex < len(historyStack)-1), we always add.
		addToHistory := true
		if len(a.historyStack) > 0 && a.currentHistoryIndex == len(a.historyStack)-1 && a.historyStack[a.currentHistoryIndex] == a.img.Path {
			addToHistory = false
		}

		if addToHistory {
			a.historyStack = append(a.historyStack, a.img.Path)
		}

		// Trim history if it exceeds capacity (remove from the beginning)
		if len(a.historyStack) > a.historyCapacity {
			a.historyStack = a.historyStack[len(a.historyStack)-a.historyCapacity:]
		}
		// After adding/trimming, the current history index points to the last item.
		a.currentHistoryIndex = len(a.historyStack) - 1
	}
	return nil
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
	log.Printf("Applying filter for tag: %s", tag)
	tagImagesPaths, err := a.tagDB.GetImages(tag)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tag, err), a.UI.MainWin)
		a.clearFilter() // Revert if error occurs
		return
	}

	if len(tagImagesPaths) == 0 {
		dialog.ShowInformation("Filter Results", fmt.Sprintf("No images found with the tag '%s'.", tag), a.UI.MainWin)
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
		a.clearFilter()
		return
	}

	a.filteredImages = newFilteredImages
	a.isFiltered = true
	a.currentFilterTag = tag
	a.index = 0     // Reset index to the start of the filtered list
	a.direction = 1 // Default direction
	log.Printf("Filter applied. %d images match tag '%s'.", len(a.filteredImages), tag)

	a.isNavigatingHistory = false // Applying a filter is a new view, not history navigation
	a.DisplayImage()              // Display the first image in the filtered set
	a.updateInfoText()            // Update info panel immediately
}

// clearFilter removes any active tag filter.
func (a *App) clearFilter() {
	if !a.isFiltered {
		return // Nothing to clear
	}
	log.Println("Clearing filter.")
	a.isFiltered = false
	a.currentFilterTag = ""
	a.filteredImages = nil // Clear the filtered list
	a.index = 0            // Reset index to the start of the full list
	a.direction = 1

	a.isNavigatingHistory = false // Clearing a filter is a new view state
	a.DisplayImage()              // Display the first image in the full set
	a.updateInfoText()            // Update info panel immediately
}

func (a *App) firstImage() {
	a.isNavigatingHistory = false
	if a.getCurrentImageCount() == 0 {
		return
	} // Add check
	a.index = 0
	a.DisplayImage()
	a.direction = 1
}

func (a *App) lastImage() {
	a.isNavigatingHistory = false
	count := a.getCurrentImageCount() // Use helper
	if count == 0 {
		return
	} // Add check
	a.index = count - 1
	a.DisplayImage()
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

	a.DisplayImage() // Display the image at the calculated index

}

// ShowNextImageFromHistory attempts to move forward in the history stack.
// Returns true if successful, false otherwise (e.g., at end of history or history disabled).
func (a *App) ShowNextImageFromHistory() bool {
	if a.historyCapacity == 0 || a.currentHistoryIndex == -1 || a.currentHistoryIndex >= len(a.historyStack)-1 {
		// Cannot go forward in history
		return false
	}

	targetHistoryIndex := a.currentHistoryIndex + 1
	// Bounds check (should be <= len(a.historyStack)-1)
	if targetHistoryIndex >= len(a.historyStack) {
		log.Println("Already at the newest image in history (targetHistoryIndex out of bounds).")
		return false
	}
	imagePathFromHistory := a.historyStack[targetHistoryIndex]

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
		log.Printf("Image %s from history not in current filter. Clearing filter state for forward navigation.", filepath.Base(imagePathFromHistory))
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
		log.Printf("Error: Image from history (%s) not found in current active list during forward navigation. Removing from history.", imagePathFromHistory)
		// Image might have been deleted or is otherwise inaccessible.
		// Remove the problematic item from history.
		if targetHistoryIndex >= 0 && targetHistoryIndex < len(a.historyStack) {
			a.historyStack = append(a.historyStack[:targetHistoryIndex], a.historyStack[targetHistoryIndex+1:]...)
			// currentHistoryIndex stays the same, effectively now pointing to the item that was after the removed one.
			// If the removed item was the last one, currentHistoryIndex will now be len(historyStack), which is handled by the check at the start of this function.
		}
		dialog.ShowInformation("History Navigation", "A previously viewed image is no longer available and was removed from history.", a.UI.MainWin)
		a.isNavigatingHistory = false // Reset flag as this specific navigation failed
		return false                  // Failed to show the historical image
	}

	a.index = foundIndexInActiveList
	a.currentHistoryIndex = targetHistoryIndex // Update to the index of the image we are now showing

	err := a.DisplayImage() // DisplayImage will respect a.isNavigatingHistory
	if err != nil {
		log.Printf("Error displaying image %s from history during forward navigation: %v", imagePathFromHistory, err)
		// Consider how to handle display errors for historical items (e.g., remove from history)
		a.isNavigatingHistory = false // Reset flag on error
		return false                  // Failed to display
	}

	a.isNavigatingHistory = false // Reset flag after the operation is complete
	return true                   // Successfully displayed historical image
}

// ShowPreviousImage handles the "back" button logic using history.
func (a *App) ShowPreviousImage() {
	if a.historyCapacity == 0 {
		log.Println("History is disabled.")
		// Optionally, could fall back to old random/previous behavior:
		// a.direction = -1
		// a.isNavigatingHistory = false // Ensure this is set if calling nextImage
		// a.nextImage()
		return
	}

	// --- Turn off random mode if active ---
	if a.random {
		a.toggleRandom() // This function handles icon update and state change
		log.Println("Random mode turned off due to back button press.")
	}

	// --- Pause slideshow if it's playing ---
	if !a.paused {
		a.togglePlay() // This function handles icon update and state change
		log.Println("Slideshow paused due to back button press.")
	}

	if a.currentHistoryIndex <= 0 || len(a.historyStack) < 2 { // Need at least 2 items to go "back" to a different one
		log.Println("No more images in history to go back to, or history is too short.")
		// Optionally, show a brief message to the user via a dialog or status bar update
		return
	}

	targetHistoryIndex := a.currentHistoryIndex - 1
	// Bounds check for targetHistoryIndex (should be >= 0)
	// This check is somewhat redundant due to `a.currentHistoryIndex <= 0` above, but good for safety.
	if targetHistoryIndex < 0 {
		log.Println("Already at the oldest image in history (targetHistoryIndex < 0).")
		return
	}
	imagePathFromHistory := a.historyStack[targetHistoryIndex]

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
		log.Printf("Image %s from history not in current filter. Clearing filter state.", filepath.Base(imagePathFromHistory))
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
		log.Printf("Error: Image from history (%s) not found in current active list. Removing from history.", imagePathFromHistory)
		if targetHistoryIndex >= 0 && targetHistoryIndex < len(a.historyStack) {
			a.historyStack = append(a.historyStack[:targetHistoryIndex], a.historyStack[targetHistoryIndex+1:]...)
			a.currentHistoryIndex-- // Adjust pointer to currently displayed image
			// Ensure index is valid after removal
			if a.currentHistoryIndex >= len(a.historyStack) {
				a.currentHistoryIndex = len(a.historyStack) - 1
			}
			if a.currentHistoryIndex < 0 {
				a.currentHistoryIndex = -1
			}
		}
		dialog.ShowInformation("History Navigation", "A previously viewed image is no longer available and was removed from history.", a.UI.MainWin)
		a.isNavigatingHistory = false // Reset flag as this specific navigation failed
		return
	}

	a.index = foundIndexInActiveList
	a.currentHistoryIndex = targetHistoryIndex // Update to the index of the image we are now showing

	err := a.DisplayImage() // DisplayImage will respect a.isNavigatingHistory
	if err != nil {
		log.Printf("Error displaying image %s from history: %v", imagePathFromHistory, err)
	}
	a.isNavigatingHistory = false // Reset flag after the operation is complete
}
func (a *App) updateTime() {
	formatted := time.Now().Format("Time: 03:04:05")
	a.UI.clockLabel.SetText(formatted)
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
	log.Printf("Deleted file: %s", deletedPath)

	// 2. Remove tags associated with this file from DB
	err := a.tagDB.RemoveAllTagsForImage(deletedPath)
	if err != nil {
		log.Printf("Warning: Failed to remove all tags for deleted file %s: %v", deletedPath, err)
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
		log.Printf("Removed image from list: %s", deletedPath)
	} else {
		log.Printf("Warning: Image not found in list: %s", deletedPath)
	}
	a.images = newImages

	// 3.5. Remove from historyStack
	if a.historyCapacity > 0 {
		newHistoryStack := make([]string, 0, len(a.historyStack))
		deletedFromHistoryIndex := -1
		for i, p := range a.historyStack {
			if p == deletedPath {
				if i <= a.currentHistoryIndex {
					// If the deleted item was at or before the current history pointer,
					// the pointer needs to shift left.
					deletedFromHistoryIndex = i // Mark that a relevant history item was deleted
				}
			} else {
				newHistoryStack = append(newHistoryStack, p)
			}
		}
		a.historyStack = newHistoryStack
		if deletedFromHistoryIndex != -1 && a.currentHistoryIndex >= deletedFromHistoryIndex {
			a.currentHistoryIndex-- // Adjust index
		}
		if a.currentHistoryIndex < 0 && len(a.historyStack) > 0 {
			a.currentHistoryIndex = 0
		}
		if len(a.historyStack) == 0 {
			a.currentHistoryIndex = -1
		}
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
			log.Println("Filtered list empty after deletion, clearing filter.")
			a.clearFilter() // This will reset index and display
			return          // clearFilter calls DisplayImage
		}
	}

	// 5. Adjust index and display the next image
	count := a.getCurrentImageCount()
	if count == 0 {
		// No images left at all (or in filter)
		a.index = -1     // Indicate no valid index
		a.DisplayImage() // Will show "No images" state
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

		a.DisplayImage() // Display the image at the (potentially adjusted) index
	}
	a.updateInfoText() // Update counts etc.
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
	log.Printf("Finished loading %d images from %s", len(a.images), root)
	// The existing wait loop in CreateApplication for imageCount() > 0 will work as before.
}

func (a *App) imageCount() int {
	return len(a.images)
}

func (a *App) init(historyCap int) { // Added historyCap parameter
	a.img = Img{}
	a.historyCapacity = historyCap
	if a.historyCapacity < 0 { // Ensure non-negative
		log.Println("Warning: History capacity cannot be negative. Setting to 0 (disabled).")
		a.historyCapacity = 0
	}
	if a.historyCapacity > 0 {
		a.historyStack = make([]string, 0, a.historyCapacity) // Initialize with capacity
	}
	a.currentHistoryIndex = -1    // Indicates history is empty or pointer is before the first item
	a.isNavigatingHistory = false // Default state
}

// Handle toggles
func (a *App) togglePlay() {
	a.paused = !a.paused // Toggle state first
	if a.paused {
		// Now paused, so button should offer to play
		if a.UI.pauseAction != nil { // Check if pauseAction is initialized
			a.UI.pauseAction.SetIcon(theme.MediaPlayIcon())
		}
	} else {
		// Now playing, so button should offer to pause
		if a.UI.pauseAction != nil { // Check if pauseAction is initialized
			a.UI.pauseAction.SetIcon(theme.MediaPauseIcon())
		}
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
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
		// Or show a dialog and exit gracefully
		// dialog.ShowError(err, ui.UI.MainWin)
		// return
	}

	ui.UI.MainWin = a.NewWindow("FySlide")
	ui.UI.MainWin.SetCloseIntercept(func() {
		log.Println("Closing tag database...")
		if err := ui.tagDB.Close(); err != nil {
			log.Printf("Error closing tag database: %v", err)
		}
		ui.UI.MainWin.Close() // Proceed with closing the window
	})

	ui.UI.MainWin.SetIcon(resourceIconPng)
	ui.init(*historySizeFlag) // Pass parsed flag to init
	ui.random = true

	ui.UI.clockLabel = widget.NewLabel("Time: ")
	ui.UI.infoText = widget.NewRichTextFromMarkdown("# Info\n---\n")

	ui.UI.MainWin.SetContent(ui.buildMainUI())

	go ui.loadImages(dir)

	ui.UI.MainWin.CenterOnScreen()
	ui.UI.MainWin.SetFullScreen(true)

	// Wait for initial scan
	startTime := time.Now()
	for ui.imageCount() < 1 {
		if time.Since(startTime) > 10*time.Second { // Timeout
			log.Println("Timeout waiting for images to load.")
			// Optionally show an error dialog
			break
		}
		time.Sleep(100 * time.Millisecond) // Slightly longer sleep
	}

	// Check if images were actually loaded
	if ui.imageCount() > 0 {
		ticker := time.NewTicker(2 * time.Second)
		ui.isNavigatingHistory = false // Initial display is not from history
		go ui.pauser(ticker)
		go ui.updateTimer()
		ui.DisplayImage()
	} else {
		// Handle case where no images were found/loaded
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
		a.updateTime()
	}
}

func (a *App) pauser(ticker *time.Ticker) {
	for range ticker.C {
		if a.UI.MainWin == nil { // Check if window is still valid
			ticker.Stop() // Stop the ticker
			return        // Exit goroutine
		}
		if !a.paused {
			a.isNavigatingHistory = false // Standard "next" is not history navigation
			a.nextImage()
		}
	}
}

// removeTagGlobally removes a specific tag from all images in the database.
func (a *App) removeTagGlobally(tag string) error {
	if tag == "" {
		return fmt.Errorf("cannot remove an empty tag")
	}
	log.Printf("Starting global removal process for tag: '%s'", tag)

	// 1. Get all images associated with this tag
	imagePaths, err := a.tagDB.GetImages(tag)
	if err != nil {
		// Log the error, but maybe the tag just doesn't exist (which is fine for removal)
		log.Printf("Error getting images for tag '%s' during global removal (maybe tag doesn't exist?): %v", tag, err)
		// Check if it's a "not found" type error if your DB layer provides it.
		// If it's just not found, we can consider it a success (nothing to remove).
		// For BoltDB, GetImages returns an empty list if the tag key doesn't exist, not an error.
		// So, an error here is likely a real DB issue.
		return fmt.Errorf("database error while getting images for tag '%s': %w", tag, err)
	}

	if len(imagePaths) == 0 {
		log.Printf("Tag '%s' not found or no images associated with it. Global removal considered complete.", tag)
		// It's important to still try and remove the tag key itself in case of orphaned data
		// The RemoveTag function should handle deleting the tag key if the image list becomes empty.
		// We can call RemoveTag with a dummy path just to trigger the tag key cleanup if needed,
		// but let's rely on the loop below (which won't run if len=0) and the TagDB logic.
		// A more robust TagDB might have a specific DeleteTagKey function.
		return nil // No images had this tag, so removal is effectively done.
	}

	log.Printf("Found %d images associated with tag '%s'. Proceeding with removal...", len(imagePaths), tag)

	// 2. Iterate and remove the tag from each image
	var firstError error
	errorsEncountered := 0
	successfulRemovals := 0

	for _, path := range imagePaths {
		// RemoveTag handles both Image->Tag and Tag->Image mappings.
		// It should also delete the Tag key if the image list becomes empty.
		errRemove := a.tagDB.RemoveTag(path, tag)
		if errRemove != nil {
			log.Printf("Error removing tag '%s' from image '%s': %v", tag, path, errRemove)
			errorsEncountered++
			if firstError == nil {
				firstError = fmt.Errorf("failed removing tag '%s' from %s: %w", tag, filepath.Base(path), errRemove)
			}
		} else {
			successfulRemovals++
		}
	}

	log.Printf("Global removal attempt for tag '%s' finished. Successes: %d, Errors: %d", tag, successfulRemovals, errorsEncountered)

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
			log.Printf("Current image %s was affected by global tag removal. Updating info panel.", currentItem.Path)
			a.updateInfoText() // Refresh the info panel to show updated tags
		}
	}

	// 4. Return the first error encountered, if any
	return firstError
}

// removeTag shows a dialog to remove an existing tag from the current image,
// with an option to remove it from all images in the same directory.
func (a *App) removeTag() {
	if a.img.Path == "" {
		dialog.ShowInformation("Remove Tag", "No image loaded to remove tags from.", a.UI.MainWin)
		return
	}
	wasPaused := a.paused // Store the original pause state
	if !wasPaused {
		a.togglePlay() // Pause the slideshow if it was running
		log.Println("Slideshow paused for tag removal.")
	}
	// 1. Get current tags for the image to populate the selector
	currentTags, err := a.tagDB.GetTags(a.img.Path)
	if err != nil {
		// If we paused, make sure to resume before showing the info and returning
		if !wasPaused {
			a.togglePlay()
			log.Println("Slideshow resumed after tag removal info (no tags).")
		}
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
		return
	}

	// 2. Check if there are any tags to remove
	if len(currentTags) == 0 {
		// If we paused, make sure to resume before showing the info and returning
		if !wasPaused {
			a.togglePlay()
			log.Println("Slideshow resumed after tag removal info (no tags).")
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
		// This will run when the callback function exits
		defer func() {
			if !wasPaused {
				a.togglePlay() // Resume slideshow ONLY if it was running before
				log.Println("Slideshow resumed after tag removal.")
			}
		}()
		if !confirm || selectedTag == "" {
			return // User cancelled or somehow didn't select a tag
		}

		removeFromAll := removeFromAllCheck.Checked // --- NEW: Get checkbox state ---

		var err error        // Use a local error variable
		var firstError error // Store the first error encountered in batch mode
		var successMessage string
		var logMessage string
		imagesUntaggedCount := 0
		errorsEncountered := 0

		if removeFromAll {
			// --- NEW: Logic to remove tag from all images in the directory ---
			currentDir := filepath.Dir(a.img.Path)
			log.Printf("Attempting to remove tag '%s' from all images in directory: %s", selectedTag, currentDir)

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
							log.Printf("Error removing tag '%s' from %s: %v", tagToRemove, itemPath, errRemove)
							errorsEncountered++
							if firstError == nil {
								firstError = fmt.Errorf("failed to untag %s: %w", filepath.Base(itemPath), errRemove)
							}
						} else {
							imagesUntaggedCount++
						}
					}()
				}
			}
			wg.Wait() // Wait for all goroutines to finish

			err = firstError // Use firstError for the main error status check later

			if imagesUntaggedCount > 0 || errorsEncountered > 0 { // Only log if something was attempted
				logMessage = fmt.Sprintf("Attempted removal of tag '%s'. Images successfully untagged: %d in %s. Errors: %d.",
					selectedTag, imagesUntaggedCount, currentDir, errorsEncountered)
			} else {
				logMessage = fmt.Sprintf("No images in directory %s found requiring removal of tag '%s'.", currentDir, selectedTag)
			}

			if errorsEncountered > 0 {
				successMessage = fmt.Sprintf("Tag '%s' removal attempted. %d images untagged.\n%d errors occurred (see logs).", selectedTag, imagesUntaggedCount, errorsEncountered)
				log.Println(successMessage)
			}
		} else {
			// Original logic: Remove only from the current image
			err = a.tagDB.RemoveTag(a.img.Path, selectedTag)
			if err == nil {
				imagesUntaggedCount = 1 // Only one attempt
				logMessage = fmt.Sprintf("Removed tag '%s' from %s", selectedTag, a.img.Path)
			}
		}

		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to remove tag '%s': %w", selectedTag, err), a.UI.MainWin)
		} else {
			if successMessage != "" { // Show partial success/error summary if there is one
				dialog.ShowInformation("Tag Removal Status", successMessage, a.UI.MainWin)
			}
			log.Println(logMessage)
			a.updateInfoText()
		}
	}, a.UI.MainWin)
}

// addTag shows a dialog to add a new tag to the current image
func (a *App) addTag() {
	if a.img.Path == "" {
		dialog.ShowInformation("Add Tag", "No image loaded to tag.", a.UI.MainWin) // Updated title
		return
	}
	// --- Pause Slideshow Logic ---
	wasPaused := a.paused // Store the original pause state
	if !wasPaused {
		a.togglePlay() // Pause the slideshow if it was running
		log.Println("Slideshow paused for tagging.")
	}

	currentTags, err := a.tagDB.GetTags(a.img.Path)
	if err != nil {
		// If we paused, make sure to resume before showing the error and returning
		if !wasPaused {
			a.togglePlay()
			log.Println("Slideshow resumed after tagging error.")
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

		// This will run when the callback function exits (after confirm/cancel/return)
		defer func() {
			if !wasPaused {
				a.togglePlay() // Resume slideshow ONLY if it was running before
				log.Println("Slideshow resumed after tagging.")
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
			tag := strings.TrimSpace(pt)
			if tag != "" && !uniqueTags[tag] { // Only add non-empty, unique tags
				tagsToAdd = append(tagsToAdd, tag)
				uniqueTags[tag] = true
			}
		}

		if len(tagsToAdd) == 0 {
			dialog.ShowInformation("Add Tag(s)", "No valid tags entered.", a.UI.MainWin)
			return // No valid tags, defer handles resume
		}

		applyToAll := applyToAllCheck.Checked // --- NEW: Get checkbox state ---

		var firstError error // Store the first error encountered
		var successMessage string
		showMessage := false
		var logMessage string
		totalTagsAttempted := 0
		successfulAdditions := 0
		errorsEncountered := 0

		// --- It correctly iterates through the 'tagsToAdd' slice ---
		if applyToAll {
			currentDir := filepath.Dir(a.img.Path)
			log.Printf("Attempting to apply %d tag(s) [%s] to all images in directory: %s", len(tagsToAdd), strings.Join(tagsToAdd, ", "), currentDir)

			var wg sync.WaitGroup
			var mu sync.Mutex // Mutex to protect shared variables
			imagesProcessed := 0

			for _, imageItem := range a.images { // Iterate through the original full list
				// Capture loop variables for the goroutine
				itemPath := imageItem.Path    // Capture path
				currentTagsToAdd := tagsToAdd // Capture tags to add for this goroutine

				itemDir := filepath.Dir(itemPath) // Use captured itemPath or imageItem.Path
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
								log.Printf("Error adding tag '%s' to %s: %v", tag, itemPath, errAdd)
								localErrorsOnThisImage++
								if localFirstErrorForThisImage == nil {
									localFirstErrorForThisImage = fmt.Errorf("failed to tag %s with '%s': %w", filepath.Base(itemPath), tag, errAdd)
								}
							} else {
								localSuccessfulAdditionsOnThisImage++
							}
						}

						mu.Lock()
						imagesProcessed++ // This image's processing is complete
						totalTagsAttempted += localTagsAttemptedOnThisImage
						successfulAdditions += localSuccessfulAdditionsOnThisImage
						errorsEncountered += localErrorsOnThisImage
						if localFirstErrorForThisImage != nil && firstError == nil {
							firstError = localFirstErrorForThisImage
						}
						mu.Unlock()
					}()
				}
			}
			wg.Wait() // Wait for all goroutines to finish

			logMessage = fmt.Sprintf("Attempted to apply %d tag(s) to %d images in %s. Successes: %d, Errors: %d", len(tagsToAdd), imagesProcessed, currentDir, successfulAdditions, errorsEncountered)
			if errorsEncountered > 0 {
				showMessage = true
				successMessage = fmt.Sprintf("%d tag(s) applied partially across %d images.\n%d errors occurred (see logs).", len(tagsToAdd), imagesProcessed, errorsEncountered)
			}
		} else {
			// Apply tags only to the current image
			log.Printf("Attempting to apply %d tag(s) [%s] to %s", len(tagsToAdd), strings.Join(tagsToAdd, ", "), a.img.Path)
			for _, tag := range tagsToAdd {
				totalTagsAttempted++
				errAdd := a.tagDB.AddTag(a.img.Path, tag)
				if errAdd != nil {
					log.Printf("Error adding tag '%s' to %s: %v", tag, a.img.Path, errAdd)
					errorsEncountered++
					if firstError == nil {
						firstError = fmt.Errorf("failed to add tag '%s': %w", tag, errAdd)
					}
				} else {
					successfulAdditions++
				}
			}
			logMessage = fmt.Sprintf("Attempted to apply %d tag(s) to %s. Successes: %d, Errors: %d", len(tagsToAdd), a.img.Path, successfulAdditions, errorsEncountered)
			if errorsEncountered > 0 {
				successMessage = fmt.Sprintf("%d tag(s) applied partially.\n%d errors occurred (see logs).", len(tagsToAdd), errorsEncountered)
				showMessage = true // Show message for partial success on single image too
			}
		}

		// Use firstError for the main dialog feedback
		err = firstError

		// --- Common Post-Processing ---
		if err != nil {
			// Show the first error encountered
			dialog.ShowError(err, a.UI.MainWin) // Simplified error message
		} else {
			log.Println(logMessage)
			a.updateInfoText() // Update info panel for the current image
			if a.refreshTagsFunc != nil {
				log.Println("Calling Tags tab refresh function.")
				a.refreshTagsFunc()
			} else {
				log.Println("Tags tab refresh function not set.")
			}
			if showMessage {
				dialog.ShowInformation("Success", successMessage, a.UI.MainWin)
			}
		}
	}, a.UI.MainWin)
}
