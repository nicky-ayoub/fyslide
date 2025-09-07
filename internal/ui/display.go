package ui

import (
	"fmt"
	"fyslide/internal/service"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
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
	currentItem := a.imageState.GetCurrentItem()
	statusText := "Ready"

	if currentItem != nil {
		statusText = fmt.Sprintf("%s  |  Image %d / %d", currentItem.Path, a.imageState.GetCurrentIndex()+1, a.imageState.GetCurrentImageCount())
		if a.imageState.IsFiltered() {
			statusText += fmt.Sprintf(" (Filtered: %s)", a.imageState.currentFilterTag)
		}
	}
	// if a.slideshowManager.IsPaused() {
	// 	statusText += " | Paused"
	// } else {
	// 	statusText += " | Playing"
	// }
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

// UpdateInfoText generates and displays the markdown-formatted metadata for the
// current image in the info panel, including stats, tags, and EXIF data.
func (a *App) UpdateInfoText(info *service.ImageInfo) {
	if a.img.Path == "" {
		a.UI.infoText.ParseMarkdown("# Info\n---\nNo image loaded.")
		return
	}

	if info == nil { // Called when image info isn't available (e.g. load error)
		a.UI.infoText.ParseMarkdown("# Info\n---\nImage metadata not available.")
		return
	}

	// --- Get Tags ---
	currentTags, tagsErr := a.Service.ListTagsForImage(a.img.Path)
	tagsString := "(none)" // Default if no tags or error occurred
	if tagsErr != nil {
		tagsString = "(error loading tags)"
	} else if len(currentTags) > 0 {
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
		info.ModTime.Format("2006-01-02 15:04:05"),
		filepath.Base(a.img.Path),
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
	a.UpdateInfoText(nil)
	if errorType == "Decoding" && formatName != "" {
		msg := fmt.Sprintf("Error %s %s (format: %s): %v", errorType, filepath.Base(imagePath), formatName, originalError)
		a.AddLogMessage(msg)
	} else {
		msg := fmt.Sprintf("Error %s %s: %v", errorType, filepath.Base(imagePath), originalError)
		a.AddLogMessage(msg)
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
		fyne.Do(func() { a.UI.toolBar.Refresh() }) // Ensure toolbar refresh is on main thread
	}
}

// UpdateClearFilterMenuVisibility enables or disables the "Clear Filter" menu item
// based on whether a filter is currently active.
func (a *App) UpdateClearFilterMenuVisibility() {
	if a.UI.clearFilterMenuItem == nil {
		return
	}
	a.UI.clearFilterMenuItem.Disabled = !a.imageState.IsFiltered()
	// Refresh the main menu to reflect the change in the item's disabled state.
	if a.UI.MainWin.MainMenu() != nil {
		a.UI.MainWin.MainMenu().Refresh()
	}
}

// TogglePlay handles toggling the slideshow state and updating the UI icon.
func (a *App) TogglePlay() {
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
	a.LoadAndDisplayCurrentImage()
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

// SetScaleAlgorithm sets the scaling algorithm on the zoomPanArea and updates the menu.
func (a *App) SetScaleAlgorithm(algo ScaleAlgorithmType) {
	if a.zoomPanArea != nil {
		a.zoomPanArea.SetScaleAlgorithm(algo)
		a.updateScaleAlgorithmMenu()
	}
}

// GetScaleAlgorithm returns the current scaling algorithm used by the zoomPanArea.
func (a *App) GetScaleAlgorithm() ScaleAlgorithmType {
	return a.zoomPanArea.GetScaleAlgorithm()
}

// updateScaleAlgorithmMenu updates the check marks on the scaling algorithm menu.
func (a *App) updateScaleAlgorithmMenu() {
	if a.UI.scaleNnMenuItem == nil || a.zoomPanArea == nil {
		return // UI not ready
	}
	currentAlgo := a.zoomPanArea.GetScaleAlgorithm()
	a.UI.scaleNnMenuItem.Checked = (currentAlgo == NearestNeighbor)
	a.UI.scaleBlMenuItem.Checked = (currentAlgo == Bilinear)
	a.UI.scaleBcMenuItem.Checked = (currentAlgo == Bicubic)

	// Refresh the main menu to show checkmark changes
	if a.UI.MainWin.MainMenu() != nil {
		a.UI.MainWin.MainMenu().Refresh()
	}
}
