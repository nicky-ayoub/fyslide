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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	//"fyne.io/fyne/v2/data/binding"

	"fyne.io/fyne/v2/dialog"
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

	Navigation *NavigationController
	imageState *ImageState
	Tagging    *TaggingController

	img         Img
	zoomPanArea *ZoomPanArea

	slideshowManager *slideshow.SlideshowManager // NEW: Use SlideshowManager

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

// getCurrentItem returns the FileItem for the current index, or nil if invalid
func (a *App) getCurrentItem() *scan.FileItem {
	return a.imageState.GetCurrentItem()
}

// getItemByViewIndex retrieves a FileItem from the active view (sequential or random)
// using a specific view index. This is the core data retrieval logic.
func (a *App) getItemByViewIndex(viewIndex int) (*scan.FileItem, error) { //nolint:unused
	return a.imageState.GetItemByViewIndex(viewIndex)
}

// getViewportItems returns a slice of ViewportItems representing the current viewport
// for the thumbnail strip, along with the index of the central item within that slice.
func (a *App) getViewportItems(centerIndex int, windowSize int) ([]ViewportItem, int) { //nolint:unused
	count := a.imageState.GetCurrentImageCount()
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

func (a *App) GetImageFullPath() string {
	item := a.getCurrentItem()
	if item == nil {
		return ""
	}
	imagePath := item.Path
	return imagePath
}

// ListAllTags is a convenience method to satisfy the TagsViewHost interface.
func (a *App) ListAllTags() ([]tagging.TagWithCount, error) {
	return a.Service.ListAllTags()
}

// GetWindow is a convenience method to satisfy the TagsViewHost interface.
func (a *App) GetWindow() fyne.Window {
	return a.UI.MainWin
}

// ApplyFilter is a convenience method to satisfy the TagsViewHost interface.
// It delegates the call to the TaggingController.
func (a *App) ApplyFilter(tags []string) {
	if a.Tagging != nil {
		a.Tagging.ApplyFilter(tags)
	}
}

// RemoveTagGlobally is a convenience method to satisfy the TagsViewHost interface.
// It delegates the call to the TaggingController.
func (a *App) RemoveTagGlobally(tag string) error {
	return a.Tagging.RemoveTagGlobally(tag)
}

// loadAndDisplayCurrentImage loads the image at the current index in the active list
// in a background goroutine and updates the UI on the main Fyne thread.
func (a *App) loadAndDisplayCurrentImage() {
	count := a.imageState.GetCurrentImageCount()
	// Handle empty list (either full or filtered)

	if count == 0 { // Handle empty list (either full or filtered)
		a.zoomPanArea.SetImage(nil)
		a.img = Img{EXIFData: make(map[string]string)} // Clear EXIF
		a.UI.MainWin.SetTitle("FySlide")
		a.updateStatusBar()
		a.updateInfoText(nil)
		a.AddLogMessage("No images available.")
		return // Exit the function, no image to load
	}

	imagePath := a.GetImageFullPath() // Get the full path of the current image

	// Check index bounds again after potential random selection or if not random
	if a.imageState.GetCurrentIndex() < 0 || a.imageState.GetCurrentIndex() >= count { // Use current count
		// This might happen if images were deleted; try to reset index or handle error
		a.imageState.SetIndex(0) // Reset to first image
		if count == 0 {          // Double check after reset attempt
			// Already handled above, but defensive check
			// This path should ideally not be hit if the initial count == 0 check is robust.
			// For safety, ensure UI reflects no images.
			fyne.Do(func() {
				a.zoomPanArea.SetImage(nil)                    // Clear the image display
				a.img = Img{EXIFData: make(map[string]string)} // Clear EXIF
				a.UI.MainWin.SetTitle("FySlide")
				a.updateStatusBar()
				a.updateInfoText(nil)
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
			a.updateInfoText(imgInfo)
			a.refreshThumbnailStrip() // Update the thumbnail strip
		})
	}(imagePath) // Pass the path and flag to the goroutine
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
		a.AddLogMessage(fmt.Sprintf("Error deleting file and tags: %v", err))
		// If the service layer couldn't delete the file (and its tags),
		// it might be best to not alter the UI lists further.
		dialog.ShowError(err, a.UI.MainWin)
		return
	}

	// 3. Delegate state update to ImageState
	a.imageState.RemoveImage(deletedPath)
	a.AddLogMessage(fmt.Sprintf("Removed %s from image list.", filepath.Base(deletedPath)))

	// 4. Check if the filtered list became empty and needs clearing
	if a.imageState.IsFiltered() && a.imageState.GetCurrentImageCount() == 0 {
		a.AddLogMessage("Filtered list empty after deletion, clearing filter.")
		a.Tagging.clearFilter() // This will reset index and display, then load the new view
		return                  // clearFilter already triggers the necessary UI updates
	}

	// 5. Refresh the UI
	// The index was adjusted by RemoveImage. We just need to load the image at the new index.

	a.loadAndDisplayCurrentImage()
	a.refreshThumbnailStrip() // Update the thumbnail strip
}

// loadImages scans the given root directory for image files in a background goroutine
// and populates the main image list.
func (a *App) loadImages(root string) {
	a.imageState.images = nil // Clear previous images or a.images = a.images[:0]

	// Define a logger function that matches scan.LoggerFunc
	// and uses the app's logUIManager.
	scanLogger := func(message string) {
		// fyne.Do is important if scan.Run's logger calls happen from a non-main goroutine
		// and a.AddLogMessage directly updates UI. a.AddLogMessage itself uses logUIManager.
		fyne.Do(func() { a.AddLogMessage(message) })
	}
	//imageChan := scan.Run(root, scanLogger) // Pass the logger
	imageChan := a.Service.FileScan.Run(root, scanLogger)
	for item := range imageChan { // Loop until the channel is closed
		a.imageState.images = append(a.imageState.images, item)
		// Optionally, you could update a progress indicator here
		// if the GUI needs to show loading progress.
	}
	msg := fmt.Sprintf("Loaded %d images from %s", a.imageState.GetCurrentImageCount(), root)
	fyne.Do(func() {
		a.AddLogMessage(msg)
		a.refreshThumbnailStrip() // Update the thumbnail strip
	})
}

// init initializes the application's core components, including the history manager,
// slideshow manager, and other configuration settings based on provided flags.
func (a *App) init(slideshowIntervalSec float64, skipNum int) {
	a.img = Img{EXIFData: make(map[string]string)} // Initialize EXIFData

	// Define a logger function for SlideshowManager
	// This closure captures 'a' (the App instance).
	slideshowLogger := func(message string) {
		// Ensure UI updates from logs happen on the Fyne goroutine.
		// a.AddLogMessage itself uses a.logUIManager which updates UI.
		fyne.Do(func() { a.AddLogMessage(fmt.Sprintf("Slideshow: %s", message)) })
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

// initServices initializes the database and all backend services.
func (a *App) initServices() error {
	// Define the logger function that TagDB and other services will use.
	appLoggerFunc := func(message string) {
		if a.logUIManager != nil {
			// Ensure UI updates are on the main Fyne thread.
			fyne.Do(func() {
				a.logUIManager.AddLogMessage(message)
			})
		} else {
			// Fallback to console log if logUIManager is not yet ready
			log.Printf("EarlyLog: %s", message)
		}
	}

	var err error
	a.tagDB, err = tagging.NewTagDB("", appLoggerFunc)
	if err != nil {
		return fmt.Errorf("failed to initialize tag database: %w", err)
	}

	fileScanner := scan.FileScannerImpl{}
	a.Service = service.NewService(a.tagDB, &fileScanner, appLoggerFunc)
	a.ImageService = service.NewImageService()
	a.thumbnailManager = NewThumbnailManager(a)

	return nil
}

// initComponents initializes all the controller components of the application.
// It should be called after services are initialized.
func (a *App) initComponents(slideshowIntervalSec float64, skipNum int) {
	// Call the original init for basic setup (slideshow manager, skip count, etc.)
	a.init(slideshowIntervalSec, skipNum)

	// Now initialize controllers that depend on services and the app instance.
	a.Navigation = NewNavigationController(a, a.imageState)
	a.Tagging = NewTaggingController(a, a.Service, a.imageState)
}

// runInitialScanAndWait starts the background image scan and waits for it to
// find at least one image or times out.
func (a *App) runInitialScanAndWait(dir string) {
	go a.loadImages(dir)

	// Wait for the initial scan to find at least one image to display.
	startTime := time.Now()
	for a.imageState.GetCurrentImageCount() < 1 {
		if time.Since(startTime) > 20*time.Second { // Timeout
			a.AddLogMessage("Timeout waiting for images to load. Please check the directory.")
			break
		}
		time.Sleep(250 * time.Millisecond) // Poll for images
	}
}

// Command-line flags
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

	ui := &App{app: a, imageState: NewImageState()}

	// Set initial theme
	ui.isDarkTheme = true // Default to dark theme
	a.Settings().SetTheme(NewSmallTabsTheme(theme.DarkTheme()))

	// 1. Initialize backend services (DB, etc.)
	if err := ui.initServices(); err != nil {
		log.Fatalf("Failed to start services: %v", err)
	}

	// 2. Initialize controller components (slideshow, navigation, tagging)
	ui.initComponents(*slideshowIntervalFlag, *skipCountFlag)

	// 3. Build the main UI window and its components
	ui.UI.MainWin = a.NewWindow("FySlide")
	ui.UI.MainWin.SetContent(ui.buildMainUI())
	ui.UI.MainWin.SetCloseIntercept(func() {
		log.Println("Closing tag database...")
		if err := ui.tagDB.Close(); err != nil {
			log.Printf("Error closing tag database: %v", err)
		}
		ui.UI.MainWin.Close() // Proceed with closing the window
	})
	ui.UI.MainWin.SetIcon(resourceIconPng)
	ui.UI.MainWin.CenterOnScreen()
	ui.UI.MainWin.SetFullScreen(true)

	// 4. Run the initial file scan and wait for some results
	ui.runInitialScanAndWait(dir)

	// 5. Final setup after initial images are loaded
	if ui.imageState.GetCurrentImageCount() > 0 {
		// Initialize the permutation manager (for random mode)
		ui.imageState.permutationManager = scan.NewPermutationManager(&ui.imageState.images)
		// Start at the beginning of the current view (sequential or random).
		ui.imageState.SetIndex(0)
		ui.startBackgroundTasks()
		ui.loadAndDisplayCurrentImage()
	} else {
		// This case is also hit on timeout if no images loaded.
		ui.updateStatusBar() // Will show "No images available" or similar.
		ui.updateInfoText(nil)
	}

	// 6. Show the window and run the application
	ui.UI.MainWin.ShowAndRun()
}

// startBackgroundTasks starts the goroutines for the slideshow ticker and UI clock.
func (a *App) startBackgroundTasks() {
	ticker := time.NewTicker(a.slideshowManager.Interval())
	go a.pauser(ticker)
	go a.updateTimer()
}

func (a *App) updateTimer() {
	for range time.Tick(time.Second) {
		if a.UI.MainWin == nil || a.UI.clockLabel == nil { // Check if UI elements are still valid
			return // Exit goroutine if window is closed
		}
		formatted := time.Now().Format("Time: 03:04:05")
		fyne.Do(func() { a.UI.clockLabel.SetText(formatted) })
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
				a.Navigation.Navigate(1)
			})
		}
	}
}
