package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

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

func (a *App) firstImage() {
	if a.getCurrentImageCount() == 0 {
		return
	}
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

// showJumpToImageDialog displays a dialog to jump to a specific image number.
func (a *App) showJumpToImageDialog() {

	// Pause slideshow on manual interaction.
	if !a.slideshowManager.IsPaused() {
		a.togglePlay()
	}
	// Get the current image count to validate user input.
	count := a.getCurrentImageCount()
	if count == 0 {
		dialog.ShowInformation("Jump to Image", "No images loaded.", a.UI.MainWin)
		return
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder(fmt.Sprintf("Enter number (1-%d)", count))

	formDialog := dialog.NewForm("Jump to Image", "Go", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Image Number", entry),
	}, func(confirm bool) {
		if !confirm {
			return
		}

		numStr := entry.Text
		num, err := strconv.Atoi(numStr)
		if err != nil {
			dialog.ShowInformation("Invalid Input", "Please enter a valid number.", a.UI.MainWin)
			return
		}

		if num < 0 || num > count-1 {
			dialog.ShowInformation("Out of Range", fmt.Sprintf("Please enter a number between 0 and %d.", count-1), a.UI.MainWin)
			return
		}

		a.navigateToIndex(num) // User input is 1-based, index is 0-based
	}, a.UI.MainWin)

	// Set OnSubmitted for the entry to submit the form on Enter key.
	entry.OnSubmitted = func(s string) {
		formDialog.Submit()
	}

	formDialog.Show()
	a.UI.MainWin.Canvas().Focus(entry)
}
