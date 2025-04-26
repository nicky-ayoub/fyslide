package ui

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// selectStackView activates the view at the given index (0 or 1) in the main content stack.
func (a *App) selectStackView(index int) {
	if a.UI.contentStack == nil {
		log.Println("ERROR: selectStackView called but contentStack is nil")
		return
	}

	var targetView fyne.CanvasObject
	if index == 0 {
		targetView = a.UI.imageContentView
	} else if index == 1 {
		targetView = a.UI.tagsContentView
	} else {
		log.Printf("ERROR: selectStackView called with invalid index: %d", index)
		return
	}

	if targetView == nil {
		log.Printf("ERROR: selectStackView - target view for index %d is nil", index)
		return
	}

	// Hide all objects in the stack first
	for _, obj := range a.UI.contentStack.Objects {
		obj.Hide()
	}

	// Show the target object
	targetView.Show()

	// Refresh the stack container to apply visibility changes
	a.UI.contentStack.Refresh()
	log.Printf("DEBUG: Switched stack view to index %d", index)

	// Special case: Refresh tags when switching TO the tags view
	if index == 1 && a.refreshTagsFunc != nil {
		log.Println("DEBUG: Refreshing tags data on view switch.")
		a.refreshTagsFunc()
	}
}

func (a *App) buildStatusBar() *fyne.Container {
	a.UI.pauseBtn = widget.NewButtonWithIcon("", theme.MediaPauseIcon(), a.togglePlay)
	// Use the renamed function addTag (if you renamed it)
	a.UI.tagBtn = widget.NewButtonWithIcon("", theme.DocumentIcon(), a.addTag) // Changed from a.tagFile
	// You could add a remove tag button here too if desired
	a.UI.removeTagBtn = widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), a.removeTag) // Need to add removeTagBtn to UI struct
	a.UI.randomBtn = widget.NewButtonWithIcon("", resourceDice24Png, a.toggleRandom)

	a.UI.tagBtn.Enable()
	a.UI.randomBtn.Enable()

	s := container.NewHBox(
		// --- End Added Buttons ---
		widget.NewButtonWithIcon("", theme.CancelIcon(), func() { a.app.Quit() }),
		widget.NewButtonWithIcon("", theme.MediaSkipPreviousIcon(), a.firstImage),
		widget.NewButtonWithIcon("", resourceBackPng, func() { a.direction = -1; a.nextImage() }),
		a.UI.pauseBtn,
		widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() { a.direction = 1; a.nextImage() }),
		widget.NewButtonWithIcon("", theme.MediaSkipNextIcon(), a.lastImage),
		a.UI.tagBtn,
		a.UI.removeTagBtn,
		widget.NewButtonWithIcon("", theme.DeleteIcon(), a.deleteFileCheck),
		a.UI.randomBtn,
		layout.NewSpacer(),

		// --- ADDED: View Switching Buttons ---
		widget.NewButtonWithIcon("", theme.FileImageIcon(), func() { // Button for Image View
			a.selectStackView(0) // Switch to image view
		}),
		widget.NewButtonWithIcon("", theme.ListIcon(), func() { // Button for Tags View
			a.selectStackView(1) // Switch to tags view
		}),
	)
	return s
}

// func (a *App) buildToolbar() *widget.Toolbar {
// 	a.UI.randomAction = widget.NewToolbarAction(resourceDice24Png, a.toggleRandom)
// 	a.UI.pauseAction = widget.NewToolbarAction(theme.MediaPauseIcon(), a.togglePlay)

// 	t := widget.NewToolbar(
// 		widget.NewToolbarAction(theme.CancelIcon(), func() { a.app.Quit() }),
// 		widget.NewToolbarAction(theme.MediaFastRewindIcon(), a.firstImage),
// 		widget.NewToolbarAction(resourceBackPng, func() { a.direction = -1; a.nextImage() }),
// 		a.UI.pauseAction,
// 		widget.NewToolbarAction(theme.MediaPlayIcon(), func() { a.direction = 1; a.nextImage() }),
// 		widget.NewToolbarAction(theme.MediaFastForwardIcon(), a.lastImage),
// 		// Use the renamed function addTag (if you renamed it)
// 		widget.NewToolbarAction(theme.DocumentIcon(), a.addTag), // Changed from a.tagFile
// 		// You could add a remove tag button here too if desired
// 		widget.NewToolbarAction(theme.ContentRemoveIcon(), a.removeTag),
// 		widget.NewToolbarAction(theme.DeleteIcon(), a.deleteFileCheck),
// 		a.UI.randomAction,
// 		widget.NewToolbarSpacer(),
// 		widget.NewToolbarAction(theme.HelpIcon(), func() {
// 			log.Println("Display help")
// 		}),
// 	)

// 	return t
// }

// buildTagsTab creates the content for the "Tags" tab with search and global removal
func (a *App) buildTagsTab() (fyne.CanvasObject, func()) {
	var tagList *widget.List
	var allTags []string            // Holds all tags fetched from DB
	var filteredData []string       // Holds the tags currently displayed in the list
	var selectedTagForAction string // Holds the string of the currently selected tag for actions

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search Tags...")

	// Function to filter and update the list display
	filterAndRefreshList := func(searchTerm string) {
		searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
		filteredData = []string{} // Clear previous filter results

		if searchTerm == "" {
			// If search is empty, show all tags
			if len(allTags) == 0 {
				filteredData = []string{"No tags found."} // Keep placeholder if DB is empty
			} else {
				filteredData = allTags
			}
		} else {
			// Filter allTags based on searchTerm
			for _, tag := range allTags {
				if strings.Contains(strings.ToLower(tag), searchTerm) {
					filteredData = append(filteredData, tag)
				}
			}
			if len(filteredData) == 0 {
				filteredData = []string{"No tags match search."} // Placeholder for no results
			}
		}

		if tagList != nil {
			tagList.Refresh()
			tagList.ScrollToTop() // Scroll to top after filtering
		}
	}

	// Function to load/reload tag data from DB and apply current filter
	loadAndFilterTagData := func() {
		var err error
		allTags, err = a.tagDB.GetAllTags() // GetAllTags already sorts them
		if err != nil {
			log.Printf("Error loading/refreshing tags: %v", err)
			allTags = []string{} // Ensure allTags is empty on error
			filteredData = []string{"Error loading tags"}
		} else if len(allTags) == 0 {
			filteredData = []string{"No tags found."}
		} else {
			// Apply the current search filter after loading
			filterAndRefreshList(searchEntry.Text)
			// Disable button and clear selection after refresh
			if tagList != nil {
				tagList.UnselectAll() // This will trigger OnUnselected
			}
			return // filterAndRefreshList already refreshes the list
		}

		// If there was an error or no tags, refresh the list directly
		if tagList != nil {
			tagList.Refresh()
			tagList.UnselectAll() // Ensure button is disabled
		}
	}

	searchEntry.OnChanged = func(searchTerm string) {
		filterAndRefreshList(searchTerm)
	}

	refreshButton := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		loadAndFilterTagData()
	})
	removeButton := widget.NewButtonWithIcon("Remove Tag Globally", theme.DeleteIcon(), func() {
		if selectedTagForAction == "" {
			return // Should not happen if button is enabled correctly, but safety check
		}

		confirmMessage := fmt.Sprintf("Are you sure you want to remove the tag '%s' from ALL images in the database?\nThis action cannot be undone.", selectedTagForAction)

		dialog.ShowConfirm("Confirm Global Tag Removal", confirmMessage, func(confirm bool) {
			if !confirm {
				return
			}

			log.Printf("User confirmed global removal of tag: %s", selectedTagForAction)
			err := a.removeTagGlobally(selectedTagForAction) // Call the new global removal function

			if err != nil {
				log.Printf("Error during global removal of tag '%s': %v", selectedTagForAction, err)
				dialog.ShowError(fmt.Errorf("failed to globally remove tag '%s': %w", selectedTagForAction, err), a.UI.MainWin)
			} else {
				log.Printf("Successfully initiated global removal of tag: %s", selectedTagForAction)
				dialog.ShowInformation("Success", fmt.Sprintf("Tag '%s' removed globally.", selectedTagForAction), a.UI.MainWin)
				// Refresh the list after successful removal
				loadAndFilterTagData()
				// Deselect and disable button after action
				tagList.UnselectAll()
			}
		}, a.UI.MainWin)
	})
	removeButton.Disable() // Start disabled
	// Combine search and refresh into a top bar
	topBar := container.NewBorder(nil, nil, nil, refreshButton, searchEntry)

	tagList = widget.NewList(
		func() int {
			return len(filteredData) // List length is based on filteredData
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("tag template") // Use label, simpler
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(filteredData[id]) // Display from filteredData
		},
	)

	tagList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filteredData) { // Bounds check on filteredData
			selectedTagForAction = ""
			removeButton.Disable()
			return
		}
		selectedTag := filteredData[id]
		isPlaceholder := selectedTag == "Error loading tags" || selectedTag == "No tags found." || selectedTag == "No tags match search."
		if isPlaceholder {
			selectedTagForAction = ""
			removeButton.Disable()
			return
		}
		selectedTagForAction = selectedTag
		removeButton.Enable()
		log.Printf("Tag selected from list: %s", selectedTag)
		a.applyFilter(selectedTag)
		if a.UI.contentStack != nil {
			a.selectStackView(0)
		}
	}

	// --- Handle Unselection ---
	tagList.OnUnselected = func(_ widget.ListItemID) {
		selectedTagForAction = ""
		removeButton.Disable()
		//a.clearFilter()
	}

	loadAndFilterTagData()

	content := container.NewBorder(topBar, removeButton, nil, nil, tagList)

	return content, loadAndFilterTagData
}

func (a *App) buildMainUI() fyne.CanvasObject {
	a.UI.MainWin.SetMaster()
	// set main mod key to super on darwin hosts, else set it to ctrl
	if runtime.GOOS == "darwin" {
		a.UI.mainModKey = fyne.KeyModifierSuper
	} else {
		a.UI.mainModKey = fyne.KeyModifierControl
	}
	a.UI.statusBar = a.buildStatusBar()

	// main menu
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("File"),
		fyne.NewMenu("Edit",
			fyne.NewMenuItem("Add Tag", a.addTag),
			fyne.NewMenuItem("Remove Tag", a.removeTag),
			fyne.NewMenuItemSeparator(), // Optional separator
			fyne.NewMenuItem("Delete Image", a.deleteFileCheck),
			fyne.NewMenuItem("Keyboard Shortucts", a.showShortcuts),
		),
		fyne.NewMenu("View",
			fyne.NewMenuItem("Next Image", func() { a.direction = 1; a.nextImage() }),
			fyne.NewMenuItem("Previous Image", func() { a.direction = -1; a.nextImage() }),
			fyne.NewMenuItemSeparator(),                              // NEW Separator
			fyne.NewMenuItem("Filter by Tag...", a.showFilterDialog), // NEW Filter option

		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About", func() {
				dialog.ShowCustom("About", "Ok", container.NewVBox(
					widget.NewLabel("A simple image slide show."),
					widget.NewHyperlink("Help and more information on Github", parseURL("https://github.com/nicky-ayoub/fyslide")),
					widget.NewLabel("v1.2 | License: MIT"),
				), a.UI.MainWin)
			}),
		),
	)
	a.UI.MainWin.SetMainMenu(mainMenu)
	a.buildKeyboardShortcuts()

	// image canvas
	a.image = &canvas.Image{}
	a.image.FillMode = canvas.ImageFillContain

	infoPanelContent := container.NewScroll(
		container.NewVBox(
			a.UI.clockLabel,
			a.UI.infoText,
		),
	)
	a.UI.split = container.NewHSplit(
		a.image,
		infoPanelContent, // Use the info panel content directly
	)
	a.UI.split.SetOffset(0.85)
	a.UI.imageContentView = a.UI.split // Store the image view content

	// --- Build Tags View Content ---
	tagsContent, refreshFunc := a.buildTagsTab()
	a.refreshTagsFunc = refreshFunc
	a.UI.tagsContentView = tagsContent // Store the tags view content

	// --- Create the Content Stack ---
	a.UI.contentStack = container.NewStack(
		a.UI.imageContentView, // Index 0
		a.UI.tagsContentView,  // Index 1
	)
	// Ensure the first view (image view) is visible initially
	a.UI.tagsContentView.Hide()
	a.UI.imageContentView.Show()

	return container.NewBorder(
		a.UI.statusBar, // a.UI.toolbar,   // Top
		nil,            // a.UI.statusBar, // Bottom
		nil,            // a.UI.explorer, // explorer left
		nil,            // right
		a.UI.contentStack,
	)
}
