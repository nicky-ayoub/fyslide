// Package ui In internal/ui/navigation_controller.go
package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NavigationHost defines the interface that the NavigationController needs to
// communicate with its hosting application.
type NavigationHost interface {
	LoadAndDisplayCurrentImage()
	IsSlideshowPaused() bool
	TogglePlay()
	GetWindow() fyne.Window
}

// NavigationController manages image navigation logic.
type NavigationController struct {
	host       NavigationHost
	imageState *ImageState
}

// NewNavigationController creates a new navigation controller.
func NewNavigationController(host NavigationHost, imageState *ImageState) *NavigationController {
	return &NavigationController{host: host, imageState: imageState}
}

// NavigateToIndex sets the current image to a specific index and loads the image.
// It's a central helper for direct jumps like from the "Jump to" dialog.
func (nc *NavigationController) NavigateToIndex(newIndex int) {
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 || newIndex < 0 || newIndex >= count {
		return // Do nothing if the list is empty or the index is out of bounds.
	}

	nc.imageState.SetIndex(newIndex)
	nc.host.LoadAndDisplayCurrentImage()
}

// NavigateToImageIndex handles a direct jump to a specific image index,
// for example, from a thumbnail click.
func (nc *NavigationController) NavigateToImageIndex(targetIndex int) {
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 || targetIndex < 0 || targetIndex >= count {
		return // Invalid index
	}

	nc.imageState.SetIndex(targetIndex)

	nc.host.LoadAndDisplayCurrentImage()
}

// FirstImage navigates to the first image in the current list.
func (nc *NavigationController) FirstImage() {
	if nc.imageState.GetCurrentImageCount() == 0 {
		return
	}
	nc.imageState.SetIndex(0)
	nc.host.LoadAndDisplayCurrentImage()
}

// LastImage navigates to the last image in the current list.
func (nc *NavigationController) LastImage() {
	if nc.imageState.GetCurrentImageCount() == 0 {
		return
	}
	nc.NavigateToIndex(nc.imageState.GetCurrentImageCount() - 1)
}

// Navigate moves the current image by a given offset.
// A positive offset moves forward, a negative offset moves backward sequentially.
func (nc *NavigationController) Navigate(offset int) {
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 {
		return
	}

	newIndex := nc.imageState.GetCurrentIndex() + offset

	if newIndex >= count {
		newIndex = count - 1
	}

	if newIndex < 0 {
		newIndex = 0 // Wrap to the end
	}

	nc.imageState.SetIndex(newIndex)
	nc.host.LoadAndDisplayCurrentImage()
}

// ShowPreviousImage navigates to the previous image and pauses the slideshow if active.
func (nc *NavigationController) ShowPreviousImage() {
	// --- Pause slideshow if it's playing (user is navigating back) ---
	if !nc.host.IsSlideshowPaused() {
		nc.host.TogglePlay()
	}

	nc.Navigate(-1)
}

// ShowJumpToImageDialog displays a dialog to jump to a specific image number.
func (nc *NavigationController) ShowJumpToImageDialog() {

	// Pause slideshow on manual interaction.
	if !nc.host.IsSlideshowPaused() {
		nc.host.TogglePlay()
	}
	// Get the current image count to validate user input.
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 {
		dialog.ShowInformation("Jump to Image", "No images loaded.", nc.host.GetWindow())
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
			dialog.ShowInformation("Invalid Input", "Please enter a valid number.", nc.host.GetWindow())
			return
		}

		if num < 1 || num > count {
			dialog.ShowInformation("Out of Range", fmt.Sprintf("Please enter a number between 1 and %d.", count), nc.host.GetWindow())
			return
		}

		nc.NavigateToIndex(num - 1) // User input is 1-based, index is 0-based
	}, nc.host.GetWindow())

	// Set OnSubmitted for the entry to submit the form on Enter key.
	entry.OnSubmitted = func(_ string) {
		formDialog.Submit()
	}

	formDialog.Show()
	nc.host.GetWindow().Canvas().Focus(entry)
}
