// package ui contains application lifecycle and initialization logic.
package ui

import (
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/slideshow"
	"fyslide/internal/tagging"
	"time"

	"fyne.io/fyne/v2"
)

const (
	// DefaultSkipCount is the default number of images to skip with PageUp/PageDown.
	DefaultSkipCount = 20
)

// init initializes the application's core components, including the slideshow
// manager and other configuration settings based on provided flags.
func (a *App) init(slideshowIntervalSec float64, skipNum int) {
	a.img = Img{EXIFData: make(map[string]string)} // Initialize EXIFData

	// a.AddLogMessage is thread-safe, so it can be called directly from any goroutine.
	slideshowLogger := func(message string) {
		a.AddLogMessage(fmt.Sprintf("Slideshow: %s", message))
	}

	a.skipCount = skipNum
	a.slideshowManager = slideshow.NewSlideshowManager(time.Duration(slideshowIntervalSec*1000)*time.Millisecond, slideshowLogger) //nolint:durationcheck
	a.maxLogMessages = DefaultMaxLogMessages

	if a.skipCount <= 0 {
		fyne.LogError(fmt.Sprintf("Skip count must be positive. Defaulting to %d. Got: %d", DefaultSkipCount, skipNum), nil)
		a.skipCount = DefaultSkipCount
	}
}

// initServices initializes the database and all backend services.
func (a *App) initServices() error {
	// a.AddLogMessage is thread-safe and handles buffering, so it can be used directly.
	appLoggerFunc := a.AddLogMessage

	var err error
	a.tagDB, err = tagging.NewTagDB("", appLoggerFunc)
	if err != nil {
		return fmt.Errorf("failed to initialize tag database: %w", err)
	}

	fileScanner := scan.FileScannerImpl{}
	a.Service = service.NewService(a.tagDB, &fileScanner, appLoggerFunc)
	a.ImageService = service.NewImageService()
	a.thumbnailManager = NewThumbnailManager(a.ImageService, appLoggerFunc)

	return nil
}

// initComponents initializes all the controller components of the application.
// It should be called after services are initialized.
func (a *App) initComponents(slideshowIntervalSec float64, skipNum int) {
	// Call the original init for basic setup (slideshow manager, skip count, etc.)
	a.init(slideshowIntervalSec, skipNum)

	// Now initialize controllers that depend on services and the app instance.
	a.Navigation = NewNavigationController(a, a.imageState) // 'a' satisfies NavigationHost
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

// flushLogBuffer sends any buffered log messages to the LogUIManager after it has been initialized.
func (a *App) flushLogBuffer() {
	if a.logUIManager == nil || len(a.logBuffer) == 0 {
		return
	}
	for _, msg := range a.logBuffer {
		a.logUIManager.AddLogMessage(msg)
	}
	a.logBuffer = nil // Clear the buffer
}

// startBackgroundTasks starts the goroutines for the slideshow ticker and UI clock.
func (a *App) startBackgroundTasks() {
	ticker := time.NewTicker(a.slideshowManager.Interval())
	go a.slideshowAdvancer(ticker)
	go a.updateTimer()
}
