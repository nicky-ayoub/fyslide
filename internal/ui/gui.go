package ui

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"

	"fyne.io/fyne/v2/canvas"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	imageViewIndex = 0
	tagsViewIndex  = 1

	noTagsFoundMsg       = "No tags found."
	noTagsMatchSearchMsg = "No tags match search."
	errorLoadingTagsMsg  = "Error loading tags."
	initialSplitOffset   = 0.85
)

// UI struct
type UI struct {
	MainWin    fyne.Window
	mainModKey fyne.KeyModifier

	split      *container.Split
	clockLabel *widget.Label
	infoText   *widget.RichText

	toolBar            *widget.Toolbar
	randomAction       *widget.ToolbarAction // Action for toggling random mode
	pauseAction        *widget.ToolbarAction // Action for toggling play/pause
	showFullSizeAction *widget.ToolbarAction // Action for showing image at full size

	contentStack     *fyne.Container   // To hold the main views
	imageContentView fyne.CanvasObject // ADDED: Holds the image view (split)
	tagsContentView  fyne.CanvasObject // ADDED: Holds the tags view content
	// --- Status Bar Elements ---
	statusBar        *fyne.Container // Changed from *widget.Label to *fyne.Container
	statusPathLabel  *widget.Label   // For file path and image count
	statusLogLabel   *widget.Label   // For log messages
	statusLogUpBtn   *widget.Button
	statusLogDownBtn *widget.Button

	// --- Thumbnail Browser Elements ---
	thumbnailBrowser *fyne.Container // Container holding the strip and collapse button
	thumbnailStrip   *fyne.Container // The HBox holding the actual thumbnail images
	collapseButton   *widget.Button
}

// selectStackView activates the view at the given index (0 or 1) in the main content stack.
func (a *App) selectStackView(index int) {
	if a.UI.contentStack == nil {
		a.addLogMessage("Internal UI Error: Cannot switch view, content stack not initialized.")
		return
	}

	if index < 0 || index >= len(a.UI.contentStack.Objects) {
		a.addLogMessage(fmt.Sprintf("Internal UI Error: Invalid view index %d.", index))
		return
	}
	// Check if the object at the target index is nil
	if a.UI.contentStack.Objects[index] == nil {
		a.addLogMessage(fmt.Sprintf("Internal UI Error: Target view for index %d is not available.", index))
		return
	}

	// Show the target object and hide others
	for i, obj := range a.UI.contentStack.Objects {
		if i != index {
			obj.Hide()
		} else {
			obj.Show() // This is the targetView
		}
	}

	// Refresh the stack container to apply visibility changes
	a.UI.contentStack.Refresh()
	// log.Printf("DEBUG: Switched stack view to index %d", index)

	// Special case: Refresh tags when switching TO the tags view
	if index == tagsViewIndex && a.refreshTagsFunc != nil {
		// log.Println("DEBUG: Refreshing tags data on view switch.")
		a.refreshTagsFunc()
	}
}

func (a *App) buildToolbar() *widget.Toolbar {
	a.UI.randomAction = widget.NewToolbarAction(resourceDice24Png, a.toggleRandom)

	initialPauseIcon := theme.MediaPlayIcon() // Default for paused state
	if a.slideshowManager != nil && !a.slideshowManager.IsPaused() {
		initialPauseIcon = theme.MediaPauseIcon()
	}
	a.UI.pauseAction = widget.NewToolbarAction(initialPauseIcon, a.togglePlay)
	a.UI.showFullSizeAction = widget.NewToolbarAction(theme.ZoomInIcon(), a.handleShowFullSizeBtn)
	a.UI.showFullSizeAction.Disable() // Initially disabled

	t := widget.NewToolbar(
		widget.NewToolbarAction(theme.CancelIcon(), func() { a.app.Quit() }),
		widget.NewToolbarAction(theme.MediaFastRewindIcon(), a.firstImage),
		widget.NewToolbarAction(theme.MediaSkipPreviousIcon(), a.ShowPreviousImage),
		a.UI.pauseAction,
		widget.NewToolbarAction(theme.MediaSkipNextIcon(), a.nextImage),
		widget.NewToolbarAction(theme.MediaFastForwardIcon(), a.lastImage),
		widget.NewToolbarAction(theme.DocumentIcon(), a.addTag), // Changed from a.tagFile
		widget.NewToolbarAction(theme.ContentRemoveIcon(), a.removeTag),
		widget.NewToolbarAction(theme.DeleteIcon(), a.deleteFileCheck),
		a.UI.randomAction,
		widget.NewToolbarSeparator(),
		a.UI.showFullSizeAction,
		widget.NewToolbarSpacer(),

		widget.NewToolbarAction(theme.FileImageIcon(), func() { // Button for Image View
			a.selectStackView(imageViewIndex) // Switch to image view
		}),
		widget.NewToolbarAction(theme.ListIcon(), func() { // Button for Tags View
			a.selectStackView(tagsViewIndex) // Switch to tags view
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			a.showHelpDialog()
		}),
	)

	return t
}

// tagListItem is a helper struct for buildTagsTab to hold tag name and count.
// Count = -1 indicates a placeholder message not to be treated as a real tag.
type tagListItem struct {
	Name  string
	Count int
}

// buildTagsTab creates the content for the "Tags" tab with search and global removal
func (a *App) buildTagsTab() (fyne.CanvasObject, func()) {
	var tagList *widget.List
	var allTags []tagListItem             // Holds all tags (name and count) fetched from DB
	var filteredDisplayData []tagListItem // Holds the tags currently displayed in the list
	var messageLabel *widget.Label        // For placeholder/status messages
	var listContentArea *fyne.Container   // A stack to hold either the list or the message
	var selectedTagForAction string       // Holds the string of the currently selected tag for actions

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search Tags...")

	// Function to filter and update the list display
	filterAndRefreshList := func(searchTerm string) {
		searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
		filteredDisplayData = []tagListItem{} // Clear previous filter results

		if searchTerm == "" {
			// If search is empty, show all tags
			filteredDisplayData = allTags
		} else {
			// Filter allTags based on searchTerm
			for _, tag := range allTags {
				if strings.Contains(strings.ToLower(tag.Name), searchTerm) {
					filteredDisplayData = append(filteredDisplayData, tag)
				}
			}
		}

		if len(filteredDisplayData) == 0 {
			currentMsg := noTagsFoundMsg
			if searchTerm != "" { // If search was active and found nothing
				currentMsg = noTagsMatchSearchMsg
			} else if len(allTags) > 0 && searchTerm == "" { // Search empty, but allTags had items (should not happen if filteredDisplayData is empty)
				// This case implies allTags itself was empty, so noTagsFoundMsg is correct.
			}
			messageLabel.SetText(currentMsg)
			messageLabel.Show()
			tagList.Hide()
		} else {
			messageLabel.Hide()
			tagList.Show()
			tagList.Refresh()
			tagList.ScrollToTop()
		}
	}

	// Function to load/reload tag data from DB and apply current filter
	loadAndFilterTagData := func() {
		var err error
		fetchedTagsWithCounts, err := a.Service.ListAllTags()
		if err != nil {
			a.addLogMessage(fmt.Sprintf("Error loading/refreshing tags: %v", err))
			allTags = []tagListItem{}
			messageLabel.SetText(errorLoadingTagsMsg)
			messageLabel.Show()
			tagList.Hide()
		} else if len(fetchedTagsWithCounts) == 0 { // Check length of fetched data
			allTags = []tagListItem{}
			// messageLabel will be set by filterAndRefreshList if allTags is empty
			filterAndRefreshList(searchEntry.Text) // Show "No tags found"
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
			if tagList != nil && tagList.Visible() {
				tagList.UnselectAll() // This will trigger OnUnselected
			}
			return // filterAndRefreshList already refreshes the list
		}

		if tagList != nil { // Ensure button is disabled if list is not shown or empty
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

			a.addLogMessage(fmt.Sprintf("User confirmed global removal of tag: %s", selectedTagForAction))
			err := a.removeTagGlobally(selectedTagForAction) // Call the new global removal function

			if err != nil {
				// Error is already logged by removeTagGlobally (via addLogMessage) and shown in dialog
				dialog.ShowError(fmt.Errorf("failed to globally remove tag '%s': %w", selectedTagForAction, err), a.UI.MainWin)
			} else {
				// Success message is logged by removeTagGlobally (via addLogMessage)
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
			return len(filteredDisplayData) // List length is based on filteredDisplayData
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("tag template") // Use label, simpler
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			// Placeholders are handled by showing messageLabel, so item here is always a real tag.
			item := filteredDisplayData[id]
			label := obj.(*widget.Label)
			label.SetText(fmt.Sprintf("%s (%d)", item.Name, item.Count))
		},
	)

	tagList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filteredDisplayData) { // Bounds check on filteredDisplayData
			// log.Println("DEBUG: Tag selection out of bounds or filteredData empty.")
			selectedTagForAction = ""
			removeButton.Disable()
			return
		}
		// No need to check for placeholder (Count == -1) as list only contains real tags now.
		selectedItem := filteredDisplayData[id]
		selectedTagForAction = selectedItem.Name // Store only the name for actions
		removeButton.Enable()
		// log.Printf("Tag selected from list: %s (Count: %d)", selectedItem.Name, selectedItem.Count)
		a.applyFilter(selectedItem.Name) // Apply filter using only the tag name
		if a.UI.contentStack != nil {
			a.selectStackView(imageViewIndex)
		}
	}

	// --- Handle Unselection ---
	tagList.OnUnselected = func(_ widget.ListItemID) {
		selectedTagForAction = ""
		removeButton.Disable()
		//a.clearFilter()
	}

	messageLabel = widget.NewLabel(noTagsFoundMsg) // Default message
	messageLabel.Alignment = fyne.TextAlignCenter
	messageLabel.Wrapping = fyne.TextWrapWord

	listContentArea = container.NewStack(messageLabel, tagList)
	tagList.Hide() // Initially hide list, loadAndFilterTagData will show it if tags exist

	loadAndFilterTagData()
	content := container.NewBorder(topBar, removeButton, nil, nil, listContentArea)

	return content, loadAndFilterTagData
}

// showHelpDialog displays a simple help dialog with application features.
func (a *App) showHelpDialog() {
	helpText := `
## FySlide Help

FySlide is an image viewer with tagging capabilities.

**Core Features:**
*   **Image Viewing:** Navigate through images using toolbar buttons or keyboard shortcuts.
    *   **Slideshow:** Automatically cycles through images. Play/Pause with the toolbar button or 'P'/Space.
    *   **Navigation:** Next/Previous, First/Last, Skip (PageUp/PageDown).
    *   **Random Mode:** Toggle random image display with the dice icon.
*   **Tagging:**
    *   **Add Tags:** Assign tags to the current image or all images in the current directory.
    *   **Remove Tags:** Remove tags from the current image or all images in the current directory.
    *   **Global Tag Removal:** Remove a specific tag from all images in the database (via Tags View).
*   **Filtering:**
    *   Filter the displayed images by selecting a tag (via Menu > View > Filter by Tag... or by clicking a tag in the Tags View).
    *   Clear the filter to see all images again.
*   **Image Deletion:** Delete the currently viewed image (with confirmation).
*   **History:** Navigate back and forward through your viewing history.

**User Interface:**
*   **Toolbar:** Provides quick access to common actions.
*   **Image View:** Displays the current image and an information panel (stats, tags).
*   **Tags View:** Lists all tags in the database, allows searching, global tag removal, and filtering by clicking a tag.
*   **Status Bar:**
    *   Shows the current image path, count, and filter status.
    *   Displays log messages (use up/down arrows next to the log to scroll through messages).
*   **Info Panel:** Shows details about the current image, including its tags.

**Keyboard Shortcuts:**
*   A comprehensive list of keyboard shortcuts can be found via Menu > Edit > Keyboard Shortcuts.
*   Common shortcuts:
    *   Arrow Keys: Next/Previous image.
    *   Q: Quit.
    *   P or Space: Toggle Play/Pause.
    *   Delete: Delete current image.
`
	dialog.ShowCustom("FySlide Help", "Close", widget.NewRichTextFromMarkdown(helpText), a.UI.MainWin)
}

// --- Tappable Image Custom Widget ---

// tappableImage is a custom widget that displays an image and handles tap events.
type tappableImage struct {
	widget.BaseWidget
	image    *canvas.Image
	onTapped func()
}

// newTappableImage creates a new tappableImage widget.
func newTappableImage(res fyne.Resource, onTapped func()) *tappableImage {
	ti := &tappableImage{
		image:    canvas.NewImageFromResource(res),
		onTapped: onTapped,
	}
	ti.image.FillMode = canvas.ImageFillContain
	ti.ExtendBaseWidget(ti) // Important: call this to register the widget
	return ti
}

// CreateRenderer is a mandatory method for a Fyne widget.
func (t *tappableImage) CreateRenderer() fyne.WidgetRenderer {
	// We just need to render the image, so it has no background of its own.
	return widget.NewSimpleRenderer(t.image)
}

// Tapped is called when the widget is tapped.
func (t *tappableImage) Tapped(_ *fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

// SetResource updates the image resource and refreshes.
func (t *tappableImage) SetResource(res fyne.Resource) {
	t.image.Resource = res
	canvas.Refresh(t.image)
}

// SetMinSize sets the minimum size of the tappable image.
func (t *tappableImage) SetMinSize(size fyne.Size) {
	t.image.SetMinSize(size)
}

// MaxVisibleThumbnails defines the maximum number of thumbnails to display in the strip.
const MaxVisibleThumbnails = 10

// refreshThumbnailStrip updates the content of the horizontal thumbnail strip.
// It calculates a window of thumbnails around the current image and displays them.
// This function replaces updateThumbnailData and updateThumbnailSelection.
func (a *App) refreshThumbnailStrip() {
	if a.UI.thumbnailStrip == nil || a.navigationQueue == nil {
		return
	}

	a.UI.thumbnailStrip.RemoveAll()

	currentList := a.getCurrentList()
	count := len(currentList)

	if count == 0 {
		a.UI.thumbnailStrip.Refresh()
		return
	}

	// --- Calculate the window of thumbnails to display ---
	// The goal is to center the current image (a.index) in the strip.
	var indicesToDisplay []int
	centerPosInStrip := MaxVisibleThumbnails / 2
	startIndex := a.index - centerPosInStrip

	// Adjust window if it goes before the start of the list
	if startIndex < 0 {
		startIndex = 0
	}

	// Adjust window if it goes past the end of the list
	endIndex := startIndex + MaxVisibleThumbnails - 1
	if endIndex >= count {
		endIndex = count - 1
		// Shift the start back to maintain the window size, if possible
		startIndex = endIndex - (MaxVisibleThumbnails - 1)
		if startIndex < 0 {
			startIndex = 0
		}
	}

	for i := startIndex; i <= endIndex; i++ {
		indicesToDisplay = append(indicesToDisplay, i)
	}

	for _, idx := range indicesToDisplay {
		// 'i' is the position in the strip (0-9), idx is the *actual image index* in the list.
		if idx < 0 || idx >= len(currentList) {
			continue // Skip if index out of range
		}
		item := currentList[idx] // The actual image data

		// Create a tappable thumbnail widget
		tappableThumb := newTappableImage(theme.FileImageIcon(), func() { // Closure to handle click
			if idx == a.index { // Clicked the *current* image in the strip (now handled by strip logic)
				return
			}
			a.isNavigatingHistory = false // Clicking a thumb is not history
			// Find the clicked image's actual index in the image list
			newIndex := a.navigationQueue.RotateTo(idx)
			if newIndex != -1 {
				a.index = newIndex // Update the main index to match what's now at the front
				a.loadAndDisplayCurrentImage()
			}
		})
		tappableThumb.SetMinSize(fyne.NewSize(ThumbnailWidth, ThumbnailHeight)) // Consistent size

		// Get thumbnail - cached or placeholder, with async load if needed
		// Pass the widget to update in the callback
		// Returns cached or placeholder image immediately, *and* calls onComplete only if async load is needed

		// Create the stack for the thumbnail and its border *before* the GetThumbnail call
		// so it can be captured by the callback.
		thumbWidget := container.NewStack(tappableThumb)

		initialResource := a.thumbnailManager.GetThumbnail(item.Path, func(resource fyne.Resource) { // Pass thumbWidget to callback
			// 'idx' and 'item' are also correctly captured here.
			// Check if the image path for this thumbnail is still the same
			// (list might have changed while thumbnail was loading)
			if currentListCheck := a.getCurrentList(); idx < len(currentListCheck) && currentListCheck[idx].Path == item.Path { // Check against original item path
				tappableThumb.SetResource(resource)
				// The callback is already on the main UI thread, so just refresh.
				thumbWidget.Refresh()
			}
		})
		// Immediately set the resource. This will be the cached image or the placeholder.
		// If it was a placeholder, the callback above will update it later.
		tappableThumb.SetResource(initialResource)
		// Refresh the widget immediately to ensure cached images are displayed correctly.
		// This is safe as we are on the main UI thread here.
		thumbWidget.Refresh()
		// Add a border for the selected image
		if idx == a.index {
			border := canvas.NewRectangle(color.Transparent)
			border.StrokeColor = theme.PrimaryColor()
			border.StrokeWidth = 3
			thumbWidget.Add(border) // Add border on top of the tappable image
		}
		a.UI.thumbnailStrip.Add(thumbWidget)
	}
	a.UI.thumbnailStrip.Refresh()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
			fyne.NewMenuItem("Next Image", a.nextImage),
			fyne.NewMenuItem("Previous Image", a.ShowPreviousImage),
			fyne.NewMenuItemSeparator(),                              // NEW Separator
			fyne.NewMenuItem("Filter by Tag...", a.showFilterDialog), // NEW Filter option

		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("Help", a.showHelpDialog),
			fyne.NewMenuItem("About", func() {
				aboutDialog := NewAbout(&a.UI.MainWin, "About FySlide", resourceIconPng)
				aboutDialog.Show()
			}),
		),
	)
	a.UI.MainWin.SetMainMenu(mainMenu)
	a.buildKeyboardShortcuts()

	// image canvas
	a.zoomPanArea = NewZoomPanArea(nil, func() { // Pass the interaction callback
		a.slideshowManager.Pause(true)
	})
	// Set the callback for zoom/pan changes to update the toolbar action visibility
	a.zoomPanArea.SetOnZoomPanChange(a.updateShowFullSizeButtonVisibility)

	infoPanelContent := container.NewScroll(
		container.NewVBox(
			a.UI.clockLabel,
			a.UI.infoText,
		),
	)
	a.UI.split = container.NewHSplit(
		a.zoomPanArea,
		infoPanelContent,
	)
	a.UI.split.SetOffset(initialSplitOffset)
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
	a.UI.statusPathLabel = widget.NewLabel("Loading images...")
	a.UI.statusPathLabel.Alignment = fyne.TextAlignLeading

	a.UI.statusLogLabel = widget.NewLabel("") // Initially empty
	a.UI.statusLogLabel.Alignment = fyne.TextAlignCenter
	a.UI.statusLogLabel.Truncation = fyne.TextTruncateEllipsis

	// Initialize LogUIManager and connect its methods to the buttons
	// Note: a.logUIManager will be nil until this point.
	a.UI.statusLogUpBtn = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		if a.logUIManager != nil {
			a.logUIManager.ShowPreviousLogMessage()
		}
	})
	a.UI.statusLogDownBtn = widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		if a.logUIManager != nil {
			a.logUIManager.ShowNextLogMessage()
		}
	})
	a.UI.statusLogUpBtn.Disable()   // Initially disabled
	a.UI.statusLogDownBtn.Disable() // Initially disabled

	logScrollButtons := container.NewHBox(a.UI.statusLogUpBtn, a.UI.statusLogDownBtn)

	// --- Build Thumbnail Browser ---
	a.UI.thumbnailStrip = container.NewHBox()

	// Create a container for the strip that has a minimum height.
	// We use a stack with a transparent rectangle that has the desired MinSize.
	stripSizer := canvas.NewRectangle(color.Transparent)
	stripSizer.SetMinSize(fyne.NewSize(0, ThumbnailHeight+10))
	sizedStrip := container.NewStack(stripSizer, a.UI.thumbnailStrip)

	a.UI.collapseButton = widget.NewButtonWithIcon("", theme.MoveDownIcon(), nil)
	a.UI.collapseButton.OnTapped = func() {
		// Toggle visibility of the sized container for the thumbnail strip
		if sizedStrip.Visible() {
			sizedStrip.Hide()
			a.UI.collapseButton.SetIcon(theme.MoveUpIcon())
		} else {
			sizedStrip.Show()
			a.UI.collapseButton.SetIcon(theme.MoveDownIcon())
		}
	}

	a.UI.statusBar = container.NewBorder(
		nil, nil, // top, bottom
		a.UI.statusPathLabel, // left
		logScrollButtons,     // right
		a.UI.statusLogLabel,  // center (main space for log message)
	)

	a.UI.thumbnailBrowser = container.NewBorder(
		nil, nil, // top, bottom
		nil, a.UI.collapseButton, // left, right
		sizedStrip, // center - use the sized container
	)

	// Instantiate LogUIManager now that its UI elements are created.
	// a.maxLogMessages is set in App.init() using DefaultMaxLogMessages from app.go
	a.logUIManager = NewLogUIManager(a.UI.statusLogLabel, a.UI.statusLogUpBtn, a.UI.statusLogDownBtn, a.maxLogMessages)
	a.logUIManager.UpdateLogDisplay() // Call once to set initial button states based on (empty) log

	mainContentAndThumbs := container.NewBorder(nil, a.UI.thumbnailBrowser, nil, nil, a.UI.contentStack)

	return container.NewBorder(
		a.UI.toolBar,   // top
		a.UI.statusBar, // bottom
		nil, nil,       // left, right
		mainContentAndThumbs,
	)
}
