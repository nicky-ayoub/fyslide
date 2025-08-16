// In internal/ui/navigation_controller.go
package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type NavigationController struct {
	app *App
}

func NewNavigationController(app *App) *NavigationController {
	return &NavigationController{app: app}
}

// navigateToIndex sets the current image to a specific index, resets the navigation
// queue, and loads the image. It's a central helper for direct jumps.
func (nc *NavigationController) NavigateToIndex(newIndex int) {
	count := nc.app.getCurrentImageCount()
	if count == 0 || newIndex < 0 || newIndex >= count {
		return // Do nothing if the list is empty or the index is out of bounds.
	}

	nc.app.index = newIndex
	nc.app.loadAndDisplayCurrentImage()
}

// navigateToImageIndex handles a direct jump to a specific image index,
// for example, from a thumbnail click. It preserves the navigation queue
// in random mode where possible by rotating it.
func (nc *NavigationController) NavigateToImageIndex(targetIndex int) {
	count := nc.app.getCurrentImageCount()
	if count == 0 || targetIndex < 0 || targetIndex >= count {
		return // Invalid index
	}

	nc.app.index = targetIndex

	nc.app.loadAndDisplayCurrentImage()
}

func (nc *NavigationController) FirstImage() {
	if nc.app.getCurrentImageCount() == 0 {
		return
	}
	nc.app.index = 0
	nc.app.loadAndDisplayCurrentImage()
}

func (nc *NavigationController) LastImage() {
	nc.NavigateToIndex(nc.app.getCurrentImageCount() - 1)
}

// navigate moves the current image by a given offset.
// A positive offset moves forward, a negative offset moves backward sequentially.
// It dispatches to more specific handlers based on the offset.
func (nc *NavigationController) Navigate(offset int) {
	count := nc.app.getCurrentImageCount()
	if count == 0 {
		return
	}

	newIndex := nc.app.index + offset

	if newIndex >= count {
		newIndex = count - 1
	}

	if newIndex < 0 {
		newIndex = 0 // Wrap to the end
	}

	nc.app.index = newIndex
	nc.app.loadAndDisplayCurrentImage()
}

// ShowPreviousImage handles the "back" button logic.
func (nc *NavigationController) ShowPreviousImage() {
	// --- Pause slideshow if it's playing (user is navigating back) ---
	if !nc.app.slideshowManager.IsPaused() {
		nc.app.togglePlay()
	}

	nc.Navigate(-1)
}

// showJumpToImageDialog displays a dialog to jump to a specific image number.
func (nc *NavigationController) ShowJumpToImageDialog() {

	// Pause slideshow on manual interaction.
	if !nc.app.slideshowManager.IsPaused() {
		nc.app.togglePlay()
	}
	// Get the current image count to validate user input.
	count := nc.app.getCurrentImageCount()
	if count == 0 {
		dialog.ShowInformation("Jump to Image", "No images loaded.", nc.app.UI.MainWin)
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
			dialog.ShowInformation("Invalid Input", "Please enter a valid number.", nc.app.UI.MainWin)
			return
		}

		if num < 0 || num > count-1 {
			dialog.ShowInformation("Out of Range", fmt.Sprintf("Please enter a number between 0 and %d.", count-1), nc.app.UI.MainWin)
			return
		}

		nc.NavigateToIndex(num) // User input is 1-based, index is 0-based
	}, nc.app.UI.MainWin)

	// Set OnSubmitted for the entry to submit the form on Enter key.
	entry.OnSubmitted = func(s string) {
		formDialog.Submit()
	}

	formDialog.Show()
	nc.app.UI.MainWin.Canvas().Focus(entry)
}
