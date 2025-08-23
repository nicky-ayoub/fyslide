// Package ui  Setup for the FySlide Application
package ui

import (
	"flag"
	"fmt"
	"fyslide/internal/scan"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	//"fyne.io/fyne/v2/data/binding"
)

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

	ui := &App{app: a, imageState: NewImageState(), scanCompleteChan: make(chan bool, 1)}

	// Set initial theme
	ui.isDarkTheme = true // Default to dark theme
	a.Settings().SetTheme(NewSmallTabsTheme(theme.DarkTheme()))

	// --- Splash Screen ---
	splashWin := a.NewWindow("Loading FySlide")
	splashWin.SetFixedSize(true)
	splashWin.Resize(fyne.NewSize(320, 240))
	splashWin.SetPadded(true)
	splashWin.CenterOnScreen()
	//splashWin.SetDecorated(false) // No title bar, etc.

	splashIcon := canvas.NewImageFromResource(resourceIconPng)
	splashIcon.SetMinSize(fyne.NewSize(128, 128))
	splashText := widget.NewLabel("Initializing...")
	splashProgress := widget.NewProgressBarInfinite()
	splashText.Alignment = fyne.TextAlignCenter
	splashContent := container.NewVBox(
		layout.NewSpacer(),
		container.NewHBox(layout.NewSpacer(), splashIcon, layout.NewSpacer()),
		splashProgress,
		splashText,
		layout.NewSpacer(),
	)
	splashWin.SetContent(splashContent)
	splashWin.Show()

	// --- Main Application Setup in Background ---
	go func() {
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
		ui.initComponents(*slideshowIntervalFlag, *skipCountFlag)

		updateSplash("Building main user interface...")
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

		// After the UI is built and logUIManager is initialized, flush any buffered logs.
		ui.flushLogBuffer()

		// Initialize the permutation manager (for random mode) before the scan starts.
		ui.imageState.permutationManager = scan.NewPermutationManager(&ui.imageState.images)
		updateSplash(fmt.Sprintf("Scanning for images in %s...", filepath.Base(dir)))
		// 4. Run the initial file scan and wait for some results
		ui.runInitialScanAndWait(dir, splashText)

		updateSplash("Loading initial image...")
		// 5. Final setup after initial images are loaded
		if ui.imageState.GetCurrentImageCount() > 0 {
			// Now that the initial scan is done, sync the permutation manager for random mode.
			// This is done once here for performance, instead of on every batch add.
			ui.imageState.SyncPermutationManager()

			// Start at the beginning of the current view (sequential or random).
			ui.imageState.SetIndex(0)
			ui.startBackgroundTasks()
			ui.LoadAndDisplayCurrentImage()
		} else {
			// This case is also hit on timeout if no images loaded.
			ui.updateStatusBar() // Will show "No images available" or similar.
			ui.UpdateInfoText(nil, nil, nil)
		}

		// 6. Close splash and show main window
		fyne.Do(func() {
			splashWin.Close()
			ui.UI.MainWin.Show()
		})
	}()

	// Run the application event loop. This will initially just service the splash screen.
	a.Run()
}
