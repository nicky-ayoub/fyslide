// Package ui  Setup for the FySlide Application
package ui

import (
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

	statusBar *fyne.Container

	quit         *widget.Button
	firstBtn     *widget.Button
	previousBtn  *widget.Button
	pauseBtn     *widget.Button
	nextBtn      *widget.Button
	lastBtn      *widget.Button
	deleteBtn    *widget.Button
	removeTagBtn *widget.Button
	tagBtn       *widget.Button
	randomBtn    *widget.Button
	randomAction *widget.ToolbarAction
	pauseAction  *widget.ToolbarAction

	statusLabel *widget.Label
	toolbar     *widget.Toolbar

	tabs *container.AppTabs
	//explorer *widget.Accordion

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
		a.UI.statusLabel.SetText("No images to display")
		if a.isFiltered {
			a.UI.statusLabel.SetText(fmt.Sprintf("No images match filter: %s", a.currentFilterTag))
		}
		a.updateInfoText()
		// Disable buttons
		a.UI.previousBtn.Disable()
		a.UI.nextBtn.Disable()
		a.UI.firstBtn.Disable()
		a.UI.lastBtn.Disable()
		a.UI.tagBtn.Disable()
		a.UI.removeTagBtn.Disable()
		a.UI.deleteBtn.Disable()
		return fmt.Errorf("no images available in the current list")
	}

	if a.random {
		// rand.Intn panics if n <= 0, handle the case of 1 image
		if count == 1 {
			a.index = 0
		} else {
			randomNumber := rand.Intn(count) // Use current count
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
		log.Printf("Unable to open image '%s': %v", imagePath, err)
		// Error handling: Show placeholder, maybe try next?
		// For simplicity, just show error state for this image
		a.image.Image = nil
		a.img = Img{Path: imagePath} // Keep path for context
		a.image.Refresh()
		a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - Error Loading %v", filepath.Base(a.img.Path)))
		a.UI.statusLabel.SetText(fmt.Sprintf("Error loading: %s", a.img.Path))
		a.updateInfoText()
		// Keep buttons enabled to allow navigation away from the error
		a.UI.previousBtn.Enable()
		a.UI.nextBtn.Enable()
		a.UI.firstBtn.Enable()
		a.UI.lastBtn.Enable()
		a.UI.tagBtn.Enable() // Can still tag/untag even if load failed
		a.UI.removeTagBtn.Enable()
		a.UI.deleteBtn.Enable()
		return fmt.Errorf("unable to open image '%s': %w", imagePath, err)
	}
	defer file.Close()

	imageDecoded, formatName, err := image.Decode(file)
	if err != nil {
		log.Printf("Unable image.Decode(%q) of format %q: %v", file.Name(), formatName, err)
		// Similar error handling
		a.image.Image = nil
		a.img = Img{Path: file.Name()}
		a.image.Refresh()
		a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - Error Decoding %v", filepath.Base(a.img.Path)))
		a.UI.statusLabel.SetText(fmt.Sprintf("Error decoding: %s", a.img.Path))
		a.updateInfoText()
		// Keep buttons enabled
		a.UI.previousBtn.Enable()
		a.UI.nextBtn.Enable()
		a.UI.firstBtn.Enable()
		a.UI.lastBtn.Enable()
		a.UI.tagBtn.Enable()
		a.UI.removeTagBtn.Enable()
		a.UI.deleteBtn.Enable()
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
	statusText := a.img.Path
	if a.isFiltered {
		statusText = fmt.Sprintf("[Filtered: %s] %s", a.currentFilterTag, a.img.Path)
	}
	a.UI.statusLabel.SetText(statusText)

	a.updateInfoText() // Call the function to update the info panel

	// --- Ensure buttons are enabled (if count > 0) ---
	a.UI.previousBtn.Enable()
	a.UI.nextBtn.Enable()
	a.UI.firstBtn.Enable()
	a.UI.lastBtn.Enable()
	a.UI.tagBtn.Enable()
	a.UI.removeTagBtn.Enable()
	a.UI.deleteBtn.Enable()

	return nil
}

// showFilterDialog displays a dialog to select a tag for filtering.
func (a *App) showFilterDialog() {
	allTags, err := a.tagDB.GetAllTags()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get tags for filtering: %w", err), a.UI.MainWin)
		return
	}

	if len(allTags) == 0 {
		dialog.ShowInformation("Filter by Tag", "No tags found in the database to filter by.", a.UI.MainWin)
		return
	}

	// Add option to clear filter
	options := append([]string{"(Show All / Clear Filter)"}, allTags...)

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

	a.DisplayImage()   // Display the first image in the filtered set
	a.updateInfoText() // Update info panel immediately
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

	a.DisplayImage()   // Display the first image in the full set
	a.updateInfoText() // Update info panel immediately
}

func (a *App) firstImage() {
	if a.getCurrentImageCount() == 0 {
		return
	} // Add check
	a.index = 0
	a.DisplayImage()
	a.direction = 1
}

func (a *App) lastImage() {
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

	a.index += a.direction
	if a.index < 0 {
		a.direction = 1
		a.index = 0
	} else if a.index >= count { // Use current count
		a.direction = -1
		a.index = count - 1 // Use current count
	}
	a.DisplayImage()
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
	// Get tags first, then remove image from each tag's list, then remove image key
	tags, err := a.tagDB.GetTags(deletedPath)
	if err != nil {
		log.Printf("Warning: Failed to get tags for deleted file %s: %v", deletedPath, err)
		// Continue deletion from lists anyway
	} else {
		for _, tag := range tags {
			errRemove := a.tagDB.RemoveTag(deletedPath, tag) // This removes both ways
			if errRemove != nil {
				log.Printf("Warning: Failed to remove tag '%s' for deleted file %s: %v", tag, deletedPath, errRemove)
			}
		}
		// Explicitly delete the image key from ImagesToTags just in case RemoveTag didn't clear it
		// (Though it should if the tag list becomes empty)
		// errDelImgKey := a.tagDB.db.Update(func(tx *bolt.Tx) error { ... delete key ... })
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
	scan.Run(root, &a.images)
}

func (a *App) imageCount() int {
	return len(a.images)
}

func (a *App) init() {
	a.img = Img{}
}

// Handle toggles
func (a *App) togglePlay() {
	if a.paused {
		a.UI.pauseBtn.SetIcon(theme.MediaPauseIcon())
		a.UI.pauseAction.SetIcon(theme.MediaPauseIcon())
	} else {
		a.UI.pauseBtn.SetIcon(theme.ContentRedoIcon())
		a.UI.pauseAction.SetIcon(theme.ContentRedoIcon())
	}
	a.paused = !a.paused
}

func (a *App) toggleRandom() {
	if a.random {
		a.UI.randomBtn.SetIcon(resourceDiceDisabled24Png)
		a.UI.randomAction.SetIcon(resourceDiceDisabled24Png)
	} else {
		a.UI.randomBtn.SetIcon(resourceDice24Png)
		a.UI.randomAction.SetIcon(resourceDice24Png)
	}
	a.random = !a.random
}

// CreateApplication is the GUI entrypoint
func CreateApplication() {
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
	ui.init()
	ui.random = true
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
		go ui.pauser(ticker)
		go ui.updateTimer()
		ui.DisplayImage()
	} else {
		// Handle case where no images were found/loaded
		ui.UI.statusLabel.SetText("No images found in directory.")
		ui.updateInfoText()
		// Disable buttons
		ui.UI.previousBtn.Disable()
		ui.UI.nextBtn.Disable()
		// ... disable others ...
	}

	ui.UI.MainWin.ShowAndRun()
}

func (a *App) updateTimer() {
	for range time.Tick(time.Second) {
		a.updateTime()
	}
}

func (a *App) pauser(ticker *time.Ticker) {
	for range ticker.C {
		if !a.paused {
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
	var firstError error = nil
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

		var err error              // Use a local error variable
		var firstError error = nil // Store the first error encountered in batch mode
		var successMessage string
		var logMessage string
		imagesUntaggedCount := 0
		errorsEncountered := 0

		if removeFromAll {
			// --- NEW: Logic to remove tag from all images in the directory ---
			currentDir := filepath.Dir(a.img.Path)
			log.Printf("Attempting to remove tag '%s' from all images in directory: %s", selectedTag, currentDir)

			for _, item := range a.images { // Iterate through the original full list
				itemDir := filepath.Dir(item.Path)
				if itemDir == currentDir {
					errRemove := a.tagDB.RemoveTag(item.Path, selectedTag)
					if errRemove != nil {
						log.Printf("Error removing tag '%s' from %s: %v", selectedTag, item.Path, errRemove)
						errorsEncountered++
						if firstError == nil {
							firstError = fmt.Errorf("failed to untag %s: %w", filepath.Base(item.Path), errRemove)
						}
					} else {
						imagesUntaggedCount++
					}
				}
			}

			err = firstError // Use firstError for the main error status check later

			logMessage = fmt.Sprintf("Attempted removal of tag '%s' for %d images in %s.", selectedTag, imagesUntaggedCount, currentDir)
			if errorsEncountered > 0 {
				successMessage = fmt.Sprintf("Tag '%s' removal attempted for %d images.\n%d errors occurred (see logs).", selectedTag, imagesUntaggedCount, errorsEncountered)
			} else {
				successMessage = fmt.Sprintf("Tag '%s' removed successfully from matching images in the directory.", selectedTag)
			}
		} else {
			// Original logic: Remove only from the current image
			err = a.tagDB.RemoveTag(a.img.Path, selectedTag)
			if err == nil {
				imagesUntaggedCount = 1 // Only one attempt
				logMessage = fmt.Sprintf("Removed tag '%s' from %s", selectedTag, a.img.Path)
				successMessage = fmt.Sprintf("Tag '%s' removed successfully.", selectedTag)
			}
		}

		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to remove tag '%s': %w", selectedTag, err), a.UI.MainWin)
		} else {
			log.Println(logMessage)
			a.updateInfoText()
			dialog.ShowInformation("Success", successMessage, a.UI.MainWin)
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

		var firstError error = nil // Store the first error encountered
		var successMessage string
		var logMessage string
		totalTagsAttempted := 0
		successfulAdditions := 0
		errorsEncountered := 0

		// --- The rest of the logic for applying tags remains the same ---
		// --- It correctly iterates through the 'tagsToAdd' slice ---
		if applyToAll {
			currentDir := filepath.Dir(a.img.Path)
			log.Printf("Attempting to apply %d tag(s) [%s] to all images in directory: %s", len(tagsToAdd), strings.Join(tagsToAdd, ", "), currentDir)

			imagesProcessed := 0
			for _, item := range a.images {
				itemDir := filepath.Dir(item.Path)
				if itemDir == currentDir {
					imagesProcessed++
					for _, tag := range tagsToAdd { // Loop through tags for each image
						totalTagsAttempted++
						errAdd := a.tagDB.AddTag(item.Path, tag)
						if errAdd != nil {
							log.Printf("Error adding tag '%s' to %s: %v", tag, item.Path, errAdd)
							errorsEncountered++
							if firstError == nil {
								firstError = fmt.Errorf("failed to tag %s with '%s': %w", filepath.Base(item.Path), tag, errAdd)
							}
						} else {
							successfulAdditions++
						}
					}
				}
			}
			logMessage = fmt.Sprintf("Attempted to apply %d tag(s) to %d images in %s. Successes: %d, Errors: %d", len(tagsToAdd), imagesProcessed, currentDir, successfulAdditions, errorsEncountered)
			if errorsEncountered > 0 {
				successMessage = fmt.Sprintf("%d tag(s) applied partially across %d images.\n%d errors occurred (see logs).", len(tagsToAdd), imagesProcessed, errorsEncountered)
			} else {
				successMessage = fmt.Sprintf("%d tag(s) applied successfully to %d images in the directory.", len(tagsToAdd), imagesProcessed)
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
			} else {
				successMessage = fmt.Sprintf("%d tag(s) added successfully.", len(tagsToAdd))
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
			dialog.ShowInformation("Success", successMessage, a.UI.MainWin)
		}
	}, a.UI.MainWin)
}
