// In internal/ui/navigation_controller.go
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

type NavigationController struct {
	host       NavigationHost
	imageState *ImageState
}

func NewNavigationController(host NavigationHost, imageState *ImageState) *NavigationController {
	return &NavigationController{host: host, imageState: imageState}
}

// navigateToIndex sets the current image to a specific index, resets the navigation
// queue, and loads the image. It's a central helper for direct jumps.
func (nc *NavigationController) NavigateToIndex(newIndex int) {
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 || newIndex < 0 || newIndex >= count {
		return // Do nothing if the list is empty or the index is out of bounds.
	}

	nc.imageState.index = newIndex
	nc.host.LoadAndDisplayCurrentImage()
}

// navigateToImageIndex handles a direct jump to a specific image index,
// for example, from a thumbnail click. It preserves the navigation queue
// in random mode where possible by rotating it.
func (nc *NavigationController) NavigateToImageIndex(targetIndex int) {
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 || targetIndex < 0 || targetIndex >= count {
		return // Invalid index
	}

	nc.imageState.index = targetIndex

	nc.host.LoadAndDisplayCurrentImage()
}

func (nc *NavigationController) FirstImage() {
	if nc.imageState.GetCurrentImageCount() == 0 {
		return
	}
	nc.imageState.index = 0
	nc.host.LoadAndDisplayCurrentImage()
}

func (nc *NavigationController) LastImage() {
	nc.NavigateToIndex(nc.imageState.GetCurrentImageCount() - 1)
}

// navigate moves the current image by a given offset.
// A positive offset moves forward, a negative offset moves backward sequentially.
// It dispatches to more specific handlers based on the offset.
func (nc *NavigationController) Navigate(offset int) {
	count := nc.imageState.GetCurrentImageCount()
	if count == 0 {
		return
	}

	newIndex := nc.imageState.index + offset

	if newIndex >= count {
		newIndex = count - 1
	}

	if newIndex < 0 {
		newIndex = 0 // Wrap to the end
	}

	nc.imageState.index = newIndex
	nc.host.LoadAndDisplayCurrentImage()
}

// ShowPreviousImage handles the "back" button logic.
func (nc *NavigationController) ShowPreviousImage() {
	// --- Pause slideshow if it's playing (user is navigating back) ---
	if !nc.host.IsSlideshowPaused() {
		nc.host.TogglePlay()
	}

	nc.Navigate(-1)
}

// showJumpToImageDialog displays a dialog to jump to a specific image number.
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
	entry.OnSubmitted = func(s string) {
		formDialog.Submit()
	}

	formDialog.Show()
	nc.host.GetWindow().Canvas().Focus(entry)
}
