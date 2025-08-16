package ui

import (
	"fmt"
	"fyslide/internal/custom_widgets"
	"fyslide/internal/service"
	"image/color"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

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

// updateStatusBar updates the text of the status bar.
func (a *App) updateStatusBar() {
	if a.UI.statusPathLabel == nil {
		return
	}
	currentItem := a.getCurrentItem()
	statusText := "Ready"

	if currentItem != nil {
		statusText = fmt.Sprintf("%s  |  Image %d / %d", currentItem.Path, a.imageState.GetCurrentIndex()+1, a.imageState.GetCurrentImageCount())
		if a.imageState.IsFiltered() {
			statusText += fmt.Sprintf(" (Filtered: %s)", a.imageState.currentFilterTag)
		}
	}
	if a.slideshowManager.IsPaused() {
		statusText += " | Paused"
	} else {
		statusText += " | Playing"
	}
	a.UI.statusPathLabel.SetText(statusText) // Update only the path label
}

// AddLogMessage adds a message to the UI log display.
func (a *App) AddLogMessage(message string) {
	if a.logUIManager != nil {
		a.logUIManager.AddLogMessage(message)
	} else {
		// Buffer the log message if the UI manager is not ready
		a.logBuffer = append(a.logBuffer, message)
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
	if err == nil && len(currentTags) > 0 {
		tagsString = strings.Join(currentTags, ", ")
	}

	exifString := "(not available)"
	if len(info.EXIFData) > 0 {
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
	if a.imageState.IsFiltered() {
		filterStatus = fmt.Sprintf("\n**Filter Active:** %s\n", a.imageState.currentFilterTag)
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
## Tags for %s
%s

---
## EXIF Data
%s
`,
		filterStatus,
		formatNumberWithCommas(int64(a.imageState.GetCurrentIndex())),
		formatNumberWithCommas(int64(a.imageState.GetCurrentImageCount())),
		formatNumberWithCommas(info.Size),
		info.Width,
		info.Height,
		filepath.Base(a.img.Path),
		info.ModTime.Format("2006-01-02 15:04:05"),
		tagsString,
		exifString,
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
		a.AddLogMessage(msg)
	} else {
		msg := fmt.Sprintf("Error %s %s: %v", errorType, filepath.Base(imagePath), originalError)
		a.AddLogMessage(msg)
	}
}

// refreshThumbnailStrip updates the content of the horizontal thumbnail strip.
// It calculates a window of thumbnails around the current image and displays them.
func (a *App) refreshThumbnailStrip() {
	if a.UI.thumbnailStrip == nil {
		return
	}
	a.UI.thumbnailStrip.RemoveAll()

	const windowSize = 11 // Should be an odd number for a perfect center
	viewportItems, centerThumbIndex := a.getViewportItems(a.imageState.GetCurrentIndex(), windowSize)

	if len(viewportItems) == 0 {
		a.UI.thumbnailStrip.Refresh()
		return
	}

	// Add a spacer before the thumbnails to push them to the center.
	a.UI.thumbnailStrip.Add(layout.NewSpacer())

	for i, viewportItem := range viewportItems {
		item := viewportItem.Item
		viewIndex := viewportItem.ViewIndex

		// Create a tappable thumbnail widget.
		tappableThumb := custom_widgets.NewTappableImage(theme.FileImageIcon(), func() {
			if viewIndex == a.imageState.GetCurrentIndex() {
				return // Do nothing if the current image's thumbnail is clicked
			}
			// Pause slideshow on manual interaction.
			if !a.slideshowManager.IsPaused() {
				a.togglePlay()
			}
			// A thumbnail click is always a direct navigation action.
			a.Navigation.NavigateToImageIndex(viewIndex)
		})
		tappableThumb.SetMinSize(fyne.NewSize(ThumbnailWidth, ThumbnailHeight)) // Consistent size

		// The thumbWidget is a stack that will hold the tappable image and a border if selected.
		thumbWidget := container.NewStack(tappableThumb)

		// Use a closure to update the thumbnail when it's loaded asynchronously.
		updateThumb := func(resource fyne.Resource) {
			tappableThumb.SetResource(resource)
			thumbWidget.Refresh()
		}
		initialResource := a.thumbnailManager.GetThumbnail(item.Path, updateThumb)
		tappableThumb.SetResource(initialResource)
		thumbWidget.Refresh()

		// Add a border for the selected image
		if i == centerThumbIndex {
			border := canvas.NewRectangle(color.Transparent)
			border.StrokeColor = theme.Color(theme.ColorNamePrimary) // Use theme-aware color
			border.StrokeWidth = 3
			thumbWidget.Add(border) // Add border on top of the tappable image
		}
		a.UI.thumbnailStrip.Add(thumbWidget)
	}
	// Add a spacer after the thumbnails to complete the centering.
	a.UI.thumbnailStrip.Add(layout.NewSpacer())

	a.UI.thumbnailStrip.Refresh()
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

// updateClearFilterMenuVisibility enables or disables the "Clear Filter" menu item
// based on whether a filter is currently active.
func (a *App) updateClearFilterMenuVisibility() {
	if a.UI.clearFilterMenuItem == nil {
		return
	}
	a.UI.clearFilterMenuItem.Disabled = !a.imageState.IsFiltered()
	// Refresh the main menu to reflect the change in the item's disabled state.
	if a.UI.MainWin.MainMenu() != nil {
		a.UI.MainWin.MainMenu().Refresh()
	}
}

// togglePlay handles toggling the slideshow state and updating the UI icon.
func (a *App) togglePlay() {
	a.slideshowManager.TogglePlayPause()
	if a.slideshowManager.IsPaused() {
		if a.UI.pauseAction != nil {
			a.UI.pauseAction.SetIcon(theme.MediaPlayIcon())
		}
	} else {
		if a.UI.pauseAction != nil {
			a.UI.pauseAction.SetIcon(theme.MediaPauseIcon())
		}
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
	a.updateStatusBar()
}

// getDiceIcon returns the appropriate dice icon resource based on random mode and current theme.
func (a *App) getDiceIcon() fyne.Resource {
	if a.imageState.IsRandom() {
		if a.isDarkTheme {
			return resourceDiceDark24Png
		}
		return resourceDice24Png
	}
	if a.isDarkTheme {
		return resourceDiceDisabledDark24Png
	}
	return resourceDiceDisabled24Png
}

// toggleRandom handles toggling the random mode and updating the UI.
func (a *App) toggleRandom() {
	currentItem := a.imageState.GetCurrentItem()
	currentPath := ""
	if currentItem != nil {
		currentPath = currentItem.Path
	}
	a.imageState.ToggleRandomMode(currentPath)
	if a.UI.randomAction != nil {
		a.UI.randomAction.SetIcon(a.getDiceIcon())
	}
	if a.UI.toolBar != nil {
		a.UI.toolBar.Refresh()
	}
	a.loadAndDisplayCurrentImage()
	a.refreshThumbnailStrip()
}

// toggleTheme switches between the light and dark application themes.
func (a *App) toggleTheme() {
	a.isDarkTheme = !a.isDarkTheme
	if a.isDarkTheme {
		a.app.Settings().SetTheme(NewSmallTabsTheme(theme.DarkTheme()))
	} else {
		a.app.Settings().SetTheme(NewSmallTabsTheme(theme.LightTheme()))
	}

	if a.UI.randomAction != nil {
		a.UI.randomAction.SetIcon(a.getDiceIcon())
	}
}
