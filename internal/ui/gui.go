package ui

import (
	"container/list"
	"fmt"
	"image/color"
	"runtime"
	"strings"

	"fyne.io/fyne/v2/layout"

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
		widget.NewToolbarAction(theme.MediaSkipNextIcon(), func() { a.navigate(1) }),
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

// tagListItem is a helper struct for the `buildTagsTab` function, holding a tag name
// and its usage count for display in the UI.
type tagListItem struct {
	Name  string
	Count int
}

// _createTagListWidgets creates the core UI components for the tags management view.
func (a *App) _createTagListWidgets() (
	searchEntry *widget.Entry,
	refreshButton *widget.Button,
	removeButton *widget.Button,
	tagList *widget.List,
	messageLabel *widget.Label,
) {
	searchEntry = widget.NewEntry()
	searchEntry.SetPlaceHolder("Search Tags...")

	refreshButton = widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), nil)
	removeButton = widget.NewButtonWithIcon("Remove Tag Globally", theme.DeleteIcon(), nil)
	removeButton.Disable() // Start disabled

	tagList = widget.NewList(
		func() int { return 0 }, // Length will be set by the controller logic
		func() fyne.CanvasObject {
			return widget.NewLabel("tag template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {}, // Update logic will be set by controller
	)

	messageLabel = widget.NewLabel(noTagsFoundMsg)
	messageLabel.Alignment = fyne.TextAlignCenter
	messageLabel.Wrapping = fyne.TextWrapWord

	return
}

// _setupTagListCallbacks wires up the event handlers and data logic for the tags view widgets.
func (a *App) _setupTagListCallbacks(
	searchEntry *widget.Entry,
	refreshButton *widget.Button,
	removeButton *widget.Button,
	tagList *widget.List,
	allTags *[]tagListItem,
	filteredDisplayData *[]tagListItem,
	selectedTagForAction *string,
	messageLabel *widget.Label,
) func() {

	var loadAndFilterTagData func() // Declare for mutual recursion with filterAndRefreshList

	// filterAndRefreshList updates the list display based on the current search term.
	filterAndRefreshList := func(searchTerm string) {
		searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
		*filteredDisplayData = []tagListItem{} // Clear previous filter results

		if searchTerm == "" {
			*filteredDisplayData = *allTags
		} else {
			for _, tag := range *allTags {
				if strings.Contains(strings.ToLower(tag.Name), searchTerm) {
					*filteredDisplayData = append(*filteredDisplayData, tag)
				}
			}
		}

		if len(*filteredDisplayData) == 0 {
			currentMsg := noTagsFoundMsg
			if searchTerm != "" {
				currentMsg = noTagsMatchSearchMsg
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

	// loadAndFilterTagData reloads all tag data from the service.
	loadAndFilterTagData = func() {
		fetchedTags, err := a.Service.ListAllTags()
		if err != nil {
			a.addLogMessage(fmt.Sprintf("Error loading/refreshing tags: %v", err))
			*allTags = []tagListItem{}
			messageLabel.SetText(errorLoadingTagsMsg)
		} else {
			*allTags = make([]tagListItem, len(fetchedTags))
			for i, tagInfo := range fetchedTags {
				(*allTags)[i] = tagListItem{Name: tagInfo.Name, Count: tagInfo.Count}
			}
		}
		filterAndRefreshList(searchEntry.Text)
		tagList.UnselectAll() // This will trigger OnUnselected and disable the button
	}

	searchEntry.OnChanged = filterAndRefreshList
	refreshButton.OnTapped = loadAndFilterTagData

	removeButton.OnTapped = func() {
		if *selectedTagForAction == "" {
			return
		}
		confirmMessage := fmt.Sprintf("Are you sure you want to remove the tag '%s' from ALL images in the database?\nThis action cannot be undone.", *selectedTagForAction)
		dialog.ShowConfirm("Confirm Global Tag Removal", confirmMessage, func(confirm bool) {
			if !confirm {
				return
			}
			a.addLogMessage(fmt.Sprintf("User confirmed global removal of tag: %s", *selectedTagForAction))
			err := a.removeTagGlobally(*selectedTagForAction)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to globally remove tag '%s': %w", *selectedTagForAction, err), a.UI.MainWin)
			} else {
				dialog.ShowInformation("Success", fmt.Sprintf("Tag '%s' removed globally.", *selectedTagForAction), a.UI.MainWin)
				loadAndFilterTagData() // Refresh list on success
			}
		}, a.UI.MainWin)
	}

	tagList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(*filteredDisplayData) {
			*selectedTagForAction = ""
			removeButton.Disable()
			return
		}
		selectedItem := (*filteredDisplayData)[id]
		*selectedTagForAction = selectedItem.Name
		removeButton.Enable()
		a.applyFilter(selectedItem.Name)
		if a.UI.contentStack != nil {
			a.selectStackView(imageViewIndex)
		}
	}

	tagList.OnUnselected = func(_ widget.ListItemID) {
		*selectedTagForAction = ""
		removeButton.Disable()
	}

	return loadAndFilterTagData
}

// buildTagsTab constructs the UI for the "Tags" management view.
func (a *App) buildTagsTab() (fyne.CanvasObject, func()) {
	// --- State Management ---
	var allTags, filteredDisplayData []tagListItem
	var selectedTagForAction string

	// --- UI Widget Creation ---
	searchEntry, refreshButton, removeButton, tagList, messageLabel := a._createTagListWidgets()

	// --- Data Binding and Callbacks ---
	tagList.Length = func() int {
		return len(filteredDisplayData)
	}
	tagList.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		item := filteredDisplayData[id]
		label := obj.(*widget.Label)
		label.SetText(fmt.Sprintf("%s (%d)", item.Name, item.Count))
	}

	// Setup all other callbacks and get the data loading function
	loadAndFilterTagData := a._setupTagListCallbacks(
		searchEntry, refreshButton, removeButton, tagList,
		&allTags, &filteredDisplayData, &selectedTagForAction, messageLabel,
	)

	// --- Initial Data Load ---
	loadAndFilterTagData()

	// --- Assemble Layout ---
	topBar := container.NewBorder(nil, nil, nil, refreshButton, searchEntry)
	listContentArea := container.NewStack(messageLabel, tagList)
	tagList.Hide() // Initially hide list, loadAndFilterTagData will show it if tags exist
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
const MaxVisibleThumbnails = 11

// ThumbnailsOnEachSide is the number of thumbnails to display on each side of the current one.
const ThumbnailsOnEachSide = MaxVisibleThumbnails / 2

// _getThumbnailIndicesToDisplay determines the list of image indices to show in the thumbnail strip.
// It handles both random and sequential modes, consolidating the logic for which thumbnails to show.
func _getThumbnailIndicesToDisplay(a *App) []int {
	currentList := a.getCurrentList()
	count := len(currentList)
	if count == 0 {
		return []int{}
	}

	// --- RANDOM MODE ---
	// In random mode, we build the list from history and the forward-queue.
	// This logic is complex, so we try it first and fall back to sequential if it fails.
	if a.random && a.thumbnailHistory.Len() > 0 && a.img.Path != "" {
		var currentElement *list.Element
		for e := a.thumbnailHistory.Back(); e != nil; e = e.Prev() {
			if e.Value.(string) == a.img.Path {
				currentElement = e
				break
			}
		}

		if currentElement != nil {
			// Found the current image in history, now build the list of paths around it.
			displayPaths := make([]string, 0, MaxVisibleThumbnails)

			// Add previous images by walking backwards from currentElement
			e := currentElement
			for i := 0; i < ThumbnailsOnEachSide; i++ {
				e = e.Prev()
				if e == nil {
					break
				}
				displayPaths = append([]string{e.Value.(string)}, displayPaths...) // Prepend
			}

			displayPaths = append(displayPaths, currentElement.Value.(string)) // Add current image

			// Add next images from history cache first
			e = currentElement
			for len(displayPaths) < MaxVisibleThumbnails {
				e = e.Next()
				if e == nil {
					break
				}
				displayPaths = append(displayPaths, e.Value.(string))
			}

			// Fill remaining slots from the navigationQueue
			if len(displayPaths) < MaxVisibleThumbnails {
				needed := MaxVisibleThumbnails - len(displayPaths)
				upcomingIndices := a.navigationQueue.GetUpcoming(needed + 1) // +1 for current
				if len(upcomingIndices) > 1 {
					for _, idx := range upcomingIndices[1:] {
						if idx >= 0 && idx < len(currentList) {
							displayPaths = append(displayPaths, currentList[idx].Path)
						}
					}
				}
			}

			// Convert the collected paths back to indices for the renderer.
			pathToIdx := make(map[string]int, len(currentList))
			for i, item := range currentList {
				pathToIdx[item.Path] = i
			}
			indices := make([]int, 0, len(displayPaths))
			for _, p := range displayPaths {
				if idx, ok := pathToIdx[p]; ok {
					indices = append(indices, idx)
				}
			}
			return indices
		}
	}

	// --- SEQUENTIAL MODE (or fallback for random mode) ---
	indicesToDisplay := make([]int, 0, MaxVisibleThumbnails)
	startIndex := a.index - ThumbnailsOnEachSide
	startIndex = min(startIndex, count-MaxVisibleThumbnails)
	startIndex = max(startIndex, 0)
	endIndex := min(startIndex+MaxVisibleThumbnails, count)

	for i := startIndex; i < endIndex; i++ {
		indicesToDisplay = append(indicesToDisplay, i)
	}
	return indicesToDisplay
}

// refreshThumbnailStrip updates the content of the horizontal thumbnail strip.
// It calculates a window of thumbnails around the current image and displays them.
// This function replaces updateThumbnailData and updateThumbnailSelection.
func (a *App) refreshThumbnailStrip() {
	if a.UI.thumbnailStrip == nil {
		return
	}

	a.UI.thumbnailStrip.RemoveAll()

	currentList := a.getCurrentList()
	if len(currentList) == 0 {
		a.UI.thumbnailStrip.Refresh()
		return
	}

	indicesToDisplay := _getThumbnailIndicesToDisplay(a)

	// Add a spacer before the thumbnails to push them to the center.
	a.UI.thumbnailStrip.Add(layout.NewSpacer())

	for _, idx := range indicesToDisplay {
		// 'idx' is the actual index of the image in the current image list (a.getCurrentList()).
		if idx < 0 || idx >= len(currentList) {
			continue // Skip if index out of range
		}
		item := currentList[idx] // The actual image data

		// Capture the loop variable for the closure.
		localIdx := idx

		// Create a tappable thumbnail widget
		tappableThumb := newTappableImage(theme.FileImageIcon(), func() {
			if localIdx == a.index {
				return // Do nothing if the current image's thumbnail is clicked
			}
			a.isNavigatingHistory = false // Clicking a thumb is not a history action

			// Directly jump to the selected index.
			a.index = localIdx
			a.navigationQueue.RotateTo(localIdx)
			a.loadAndDisplayCurrentImage()
		})
		tappableThumb.SetMinSize(fyne.NewSize(ThumbnailWidth, ThumbnailHeight)) // Consistent size

		// The thumbWidget is a stack that will hold the tappable image and a border if selected.
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
	// Add a spacer after the thumbnails to complete the centering.
	a.UI.thumbnailStrip.Add(layout.NewSpacer())

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
	a.UI.toolBar = a.buildToolbar()
	// --- Main Menu ---
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
			fyne.NewMenuItem("Next Image", func() { a.navigate(1) }),
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

	// --- Image View (Canvas and Info Panel) ---
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
