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

func (a *App) buildToolbar() *widget.Toolbar {
	a.UI.randomAction = widget.NewToolbarAction(resourceDice24Png, a.toggleRandom)
	a.UI.pauseAction = widget.NewToolbarAction(theme.MediaPauseIcon(), a.togglePlay)

	t := widget.NewToolbar(
		widget.NewToolbarAction(theme.CancelIcon(), func() { a.app.Quit() }),
		widget.NewToolbarAction(theme.MediaFastRewindIcon(), a.firstImage),
		widget.NewToolbarAction(theme.MediaSkipPreviousIcon(), a.ShowPreviousImage),
		a.UI.pauseAction,
		widget.NewToolbarAction(theme.MediaSkipNextIcon(), func() { a.direction = 1; a.nextImage() }), // Changed icon
		widget.NewToolbarAction(theme.MediaFastForwardIcon(), a.lastImage),
		widget.NewToolbarAction(theme.DocumentIcon(), a.addTag), // Changed from a.tagFile
		widget.NewToolbarAction(theme.ContentRemoveIcon(), a.removeTag),
		widget.NewToolbarAction(theme.DeleteIcon(), a.deleteFileCheck),
		a.UI.randomAction,
		widget.NewToolbarSpacer(),

		widget.NewToolbarAction(theme.FileImageIcon(), func() { // Button for Image View
			a.selectStackView(0) // Switch to image view
		}),
		widget.NewToolbarAction(theme.ListIcon(), func() { // Button for Tags View
			a.selectStackView(1) // Switch to tags view
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			log.Println("Display help")
		}),
	)

	return t
}

// Helper struct for buildTagsTab to hold tag name and count.
// Count = -1 indicates a placeholder message not to be treated as a real tag.
type tagListItem struct {
	Name  string
	Count int
}

// buildTagsTab creates the content for the "Tags" tab with search and global removal
func (a *App) buildTagsTab() (fyne.CanvasObject, func()) {
	var tagList *widget.List
	var allTags []tagListItem       // Holds all tags (name and count) fetched from DB
	var filteredData []tagListItem  // Holds the tags currently displayed in the list
	var selectedTagForAction string // Holds the string of the currently selected tag for actions

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search Tags...")

	// Function to filter and update the list display
	filterAndRefreshList := func(searchTerm string) {
		searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
		filteredData = []tagListItem{} // Clear previous filter results

		if searchTerm == "" {
			// If search is empty, show all tags
			if len(allTags) == 0 {
				filteredData = []tagListItem{{Name: "No tags found.", Count: -1}}
			} else {
				filteredData = allTags
			}
		} else {
			// Filter allTags based on searchTerm
			for _, tag := range allTags {
				if strings.Contains(strings.ToLower(tag.Name), searchTerm) {
					filteredData = append(filteredData, tag)
				}
			}
			if len(filteredData) == 0 {
				filteredData = []tagListItem{{Name: "No tags match search.", Count: -1}}
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
		// a.tagDB.GetAllTags() now returns []tagging.TagWithCount, error
		fetchedTagsWithCounts, err := a.tagDB.GetAllTags()
		if err != nil {
			log.Printf("Error loading/refreshing tags: %v", err)
			allTags = []tagListItem{} // Ensure allTags is empty on error
			filteredData = []tagListItem{{Name: "Error loading tags", Count: -1}}
		} else if len(fetchedTagsWithCounts) == 0 { // Check length of fetched data
			allTags = []tagListItem{}
			filteredData = []tagListItem{{Name: "No tags found.", Count: -1}}
		} else {
			// Convert []tagging.TagWithCount to []tagListItem for the UI
			tempAllTags := make([]tagListItem, len(fetchedTagsWithCounts))
			for i, tagInfo := range fetchedTagsWithCounts {
				tempAllTags[i] = tagListItem{Name: tagInfo.Name, Count: tagInfo.Count}
			}
			allTags = tempAllTags

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
			item := filteredData[id]
			label := obj.(*widget.Label)
			if item.Count == -1 { // It's a placeholder message
				label.SetText(item.Name)
			} else {
				label.SetText(fmt.Sprintf("%s (%d)", item.Name, item.Count))
			}
		},
	)

	tagList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filteredData) { // Bounds check on filteredData
			log.Println("DEBUG: Tag selection out of bounds or filteredData empty.")
			selectedTagForAction = ""
			removeButton.Disable()
			return
		}
		selectedItem := filteredData[id]

		if selectedItem.Count == -1 { // Check if it's a placeholder
			log.Printf("DEBUG: Placeholder item selected: %s", selectedItem.Name)
			selectedTagForAction = ""
			removeButton.Disable()
			return
		}
		selectedTagForAction = selectedItem.Name // Store only the name for actions
		removeButton.Enable()
		log.Printf("Tag selected from list: %s (Count: %d)", selectedItem.Name, selectedItem.Count)
		a.applyFilter(selectedItem.Name) // Apply filter using only the tag name
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
	//a.UI.ribbonBar = a.buildRibbon()
	a.UI.toolBar = a.buildToolbar()
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
			fyne.NewMenuItem("Previous Image", a.ShowPreviousImage),
			fyne.NewMenuItemSeparator(),                              // NEW Separator
			fyne.NewMenuItem("Filter by Tag...", a.showFilterDialog), // NEW Filter option

		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About", func() {
				aboutDialog := NewAbout(&a.UI.MainWin, "About FySlide", resourceIconPng)
				aboutDialog.Show()
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

	// --- Initialize Status Bar ---
	a.UI.statusBar = widget.NewLabel("Loading images...")
	a.UI.statusBar.Alignment = fyne.TextAlignCenter // Align text to the center

	return container.NewBorder(
		a.UI.toolBar,   // top
		a.UI.statusBar, // bottom
		nil,            // a.UI.explorer, // explorer left
		nil,            // right
		a.UI.contentStack,
	)
}
