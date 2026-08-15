// Package ui  Setup for the FySlide Application
package ui

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"

	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	//"fyne.io/fyne/v2/data/binding"
)

// Command-line flags
var (
	slideshowIntervalFlag = flag.Float64("slideshow-interval", 3.0, "Slideshow image display interval in seconds. Min: 0.1.")
	skipCountFlag         = flag.Int("skip-count", 20, "Number of images to skip with PageUp/PageDown. Min: 1.")
	versionFlag           = flag.Bool("version", false, "Print version and exit")
	loadDbFlag            = flag.Bool("load-db", false, "Pre-load known image paths from database on startup.")
	// This can be set during the build process using ldflags
	version = "dev"
)

// AppConfig holds the application configuration derived from command-line flags.
type AppConfig struct {
	Directory         string
	SlideshowInterval float64
	SkipCount         int
	LoadFromDB        bool
}

// createSplashScreen creates and returns a new splash screen window and its text label.
func createSplashScreen(a fyne.App) (fyne.Window, *widget.Label) {
	win := a.NewWindow("Loading FySlide")
	win.SetFixedSize(true)
	win.Resize(fyne.NewSize(640, 480))
	win.SetPadded(true)
	win.CenterOnScreen()

	splashIcon := canvas.NewImageFromResource(resourceFyslidesplash004Png)
	splashIcon.SetMinSize(fyne.NewSize(480, 480))
	splashText := widget.NewLabel("Initializing...")
	splashProgress := widget.NewProgressBarInfinite()
	splashText.Alignment = fyne.TextAlignCenter
	content := container.NewVBox(
		layout.NewSpacer(),
		container.NewHBox(layout.NewSpacer(), splashIcon, layout.NewSpacer()),
		splashProgress,
		splashText,
		layout.NewSpacer(),
	)
	win.SetContent(content)
	return win, splashText
}

// parseConfig handles command-line flags and arguments, validates them,
// and returns a configuration struct or an error.
func parseConfig() (*AppConfig, error) {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("fyslide version %s\n", version)
		return nil, nil // Signal to exit gracefully
	}

	// The first non-flag argument is the directory. Default to current directory.
	dir := "."
	if len(flag.Args()) > 0 {
		dir = flag.Args()[0]
	}

	// Check if the directory exists and is valid.
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("error accessing '%s': %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path '%s' is not a directory", dir)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("could not determine absolute path for '%s': %w", dir, err)
	}

	return &AppConfig{
		Directory:         absDir,
		SlideshowInterval: *slideshowIntervalFlag,
		SkipCount:         *skipCountFlag,
		LoadFromDB:        *loadDbFlag,
	}, nil
}

// setupAndLaunch performs the main application setup in a background goroutine.
func setupAndLaunch(ctx context.Context, ui *App, config *AppConfig, splashWin fyne.Window, splashText *widget.Label) {
	// This function will be called from the goroutine to update the splash screen text
	updateSplash := func(text string) {
		fyne.Do(func() {
			splashText.SetText(text)
		})
	}

	updateSplash("Initializing services...")
	// 1. Initialize backend services (DB, etc.)
	if err := ui.initServices(); err != nil {
		// In a real app, you might show an error dialog here before quitting.
		log.Fatalf("Failed to start services: %v", err)
	}

	updateSplash("Initializing components...")
	// 2. Initialize controller components (slideshow, navigation, tagging)
	ui.initComponents(config.SlideshowInterval, config.SkipCount)

	updateSplash("Building main user interface...")
	// 3. Build the main UI window and its components
	// Create and configure the main window on the Fyne UI thread.
	fyne.DoAndWait(func() {
		ui.UI.MainWin = ui.app.NewWindow("FySlide")
		ui.UI.MainWin.SetContent(ui.buildMainUI())
		ui.UI.MainWin.SetCloseIntercept(func() {
			if ui.Tagging.IsBusy() {
				dialog.ShowInformation("Operation in Progress", "A tagging operation is in progress.\nPlease wait for it to complete before closing the application.", ui.UI.MainWin)
				return
			}
			// Save the splitter offset before closing
			if ui.UI.split != nil {
				ui.app.Preferences().SetFloat(prefSplitterOffset, ui.UI.split.Offset)
			}

			log.Println("Closing application resources...")
			// Cancel background workers before closing DB
			if ui.appCancel != nil {
				ui.appCancel()
			}
			if ui.tagDB != nil {
				if err := ui.tagDB.Close(); err != nil {
					log.Printf("Error closing tag database: %v", err)
				}
			}
			ui.UI.MainWin.Close() // Proceed with closing the window
		})
		ui.UI.MainWin.SetIcon(resourceIconPng)
		ui.UI.MainWin.CenterOnScreen()
		//ui.UI.MainWin.SetFullScreen(true)
		ui.UI.MainWin.Resize(fyne.NewSize(1920, 1024))
	})

	// After the UI is built and logUIManager is initialized, flush any buffered logs.
	ui.flushLogBuffer()

	updateSplash(fmt.Sprintf("Scanning for images in %s...", filepath.Base(config.Directory)))
	// 4. Run the initial file scan and wait for some results
	ui.runInitialScanAndWait(ctx, config.Directory, splashText, config.LoadFromDB)

	updateSplash("Loading initial image...")
	// 5. Final setup after initial images are loaded
	if ui.imageState.GetCurrentImageCount() > 0 {
		// Now that the initial scan is done, sync the permutation manager for random mode.
		// This is done once here for performance, instead of on every batch add.
		ui.imageState.SyncPermutationManager()

		// Start at the beginning of the current view (sequential or random).
		ui.imageState.SetIndex(0)
		ui.startBackgroundTasks(ui.appCtx)
		// The first image will be loaded after the main window is shown to ensure
		// correct sizing.
	} else {
		// This case is also hit on timeout if no images loaded.
		ui.updateStatusBar() // Will show "No images available" or similar.
		ui.UpdateInfoText(nil)
	}

	// 6. Close splash and show main window
	fyne.Do(func() {
		splashWin.Close()
		ui.UI.MainWin.Show()

		// --- Load and apply persistent settings ---
		// Load scaling algorithm preference, defaulting to Bilinear.
		algoStr := ui.app.Preferences().StringWithFallback(prefScaleAlgorithm, "bilinear")
		initialAlgo := Bilinear
		if algoStr == "nearest" {
			initialAlgo = NearestNeighbor
		} else if algoStr == "bicubic" {
			initialAlgo = Bicubic
		}
		ui.SetScaleAlgorithm(initialAlgo)

		// Load thumbnail format preference, defaulting to JPEG.
		formatStr := ui.app.Preferences().StringWithFallback(prefThumbnailFormat, "jpeg")
		initialFormat := JPEG
		if formatStr == "png" {
			initialFormat = PNG
		}
		ui.SetThumbnailFormat(initialFormat)
		ui.updateThumbnailFormatMenu()

		// Load and apply splitter offset
		ui.UI.split.SetOffset(ui.app.Preferences().FloatWithFallback(prefSplitterOffset, initialSplitOffset))

		// Now that the window is visible and all widgets have their final sizes,
		// we can load the first image. The internal Reset() call within
		// LoadAndDisplayCurrentImage will now use the correct component size.
		ui.LoadAndDisplayCurrentImage()
	})
}

// run initializes and runs the Fyne application.
func run(ctx context.Context, config *AppConfig) {
	a := app.NewWithID("com.github.nicky-ayoub/fyslide")
	a.SetIcon(resourceIconPng)

	appCtx, appCancel := context.WithCancel(ctx)
	ui := &App{app: a, imageState: NewImageState(), scanCompleteChan: make(chan bool, 1), appCtx: appCtx, appCancel: appCancel}

	// Set initial theme
	ui.isDarkTheme = true // Default to dark theme
	a.Settings().SetTheme(NewSmallTabsTheme(theme.DarkTheme()))

	// --- Splash Screen ---
	splashWin, splashText := createSplashScreen(a)
	splashWin.Show()

	// --- Main Application Setup in Background ---
	// Delay starting the background setup slightly to ensure the Fyne
	// UI thread and driver are initialized by `a.Run()` before we make
	// any calls that marshal to the UI thread via `fyne.Do`/`DoAndWait`.
	go func() {
		time.Sleep(100 * time.Millisecond)
		setupAndLaunch(ctx, ui, config, splashWin, splashText)
	}()

	// Run the application event loop. This will initially just service the splash screen.
	a.Run()
}

// CreateApplication is the GUI entrypoint. It parses configuration and runs the application.
func CreateApplication(ctx context.Context) {
	config, err := parseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if config == nil { // This happens if --version is used
		return
	}
	run(ctx, config)
}
