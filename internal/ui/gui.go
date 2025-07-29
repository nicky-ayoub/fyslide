package ui

import (
	"fmt"
	"image/color"
	"runtime"

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

	toolBar             *widget.Toolbar
	randomAction        *widget.ToolbarAction // Action for toggling random mode
	pauseAction         *widget.ToolbarAction // Action for toggling play/pause
	showFullSizeAction  *widget.ToolbarAction // Action for showing image at full size
	clearFilterMenuItem *fyne.MenuItem        // For the View > Clear Filter menu item

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

// tagListController manages the state and logic for the tags view.
type tagListController struct {
	app *App // To access services and other app-level methods

	// UI Widgets
	searchEntry   *widget.Entry
	refreshButton *widget.Button
	removeButton  *widget.Button
	tagList       *widget.List
	messageLabel  *widget.Label

	// State
	allTags              []tagListItem
	filteredDisplayData  []tagListItem
	selectedTagForAction string
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
	a.UI.randomAction = widget.NewToolbarAction(a.getDiceIcon(), a.toggleRandom) // Use theme-aware icon

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
		widget.NewToolbarAction(theme.ContentRedoIcon(), a.showJumpToImageDialog),
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
		widget.NewToolbarAction(theme.ColorPaletteIcon(), a.toggleTheme),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			a.showHelpDialog()
		}),
	)

	return t
}

// newTagListController creates and initializes a new controller for the tags view.
func newTagListController(
	app *App,
	searchEntry *widget.Entry,
	refreshButton *widget.Button,
	removeButton *widget.Button,
	tagList *widget.List,
	messageLabel *widget.Label,
) *tagListController {
	c := &tagListController{
		app:           app,
		searchEntry:   searchEntry,
		refreshButton: refreshButton,
		removeButton:  removeButton,
		tagList:       tagList,
		messageLabel:  messageLabel,
	}

	// Wire up the callbacks
	c.searchEntry.OnChanged = c.filterAndRefreshList
	c.refreshButton.OnTapped = c.loadAndFilterTagData
	c.removeButton.OnTapped = c.onRemoveTapped
	c.tagList.OnSelected = c.onTagSelected
	c.tagList.OnUnselected = c.onTagUnselected

	return c
}

// buildTagsTab constructs the UI for the "Tags" management view.
func (a *App) buildTagsTab() (fyne.CanvasObject, func()) {
	// --- UI Widget Creation ---
	searchEntry, refreshButton, removeButton, tagList, messageLabel := a._createTagListWidgets()

	// --- Controller Creation and Wiring ---
	controller := newTagListController(a, searchEntry, refreshButton, removeButton, tagList, messageLabel)

	// --- Data Binding ---
	tagList.Length = func() int {
		return len(controller.filteredDisplayData)
	}
	tagList.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		item := controller.filteredDisplayData[id]
		label := obj.(*widget.Label)
		label.SetText(fmt.Sprintf("%s (%d)", item.Name, item.Count))
	}

	// --- Initial Data Load ---
	controller.loadAndFilterTagData()

	// --- Assemble Layout ---
	topBar := container.NewBorder(nil, nil, nil, refreshButton, searchEntry)
	listContentArea := container.NewStack(messageLabel, tagList)
	tagList.Hide() // Initially hide list, controller will show it if tags exist
	content := container.NewBorder(topBar, removeButton, nil, nil, listContentArea)

	// The refresh function is now the controller's data loading method.
	return content, controller.loadAndFilterTagData
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

// MaxVisibleThumbnails defines the maximum number of thumbnails to display in the strip.
const MaxVisibleThumbnails = 11

func (a *App) buildMainUI() fyne.CanvasObject {
	a.UI.MainWin.SetMaster()
	// set main mod key to super on darwin hosts, else set it to ctrl
	if runtime.GOOS == "darwin" {
		a.UI.mainModKey = fyne.KeyModifierSuper
	} else {
		a.UI.mainModKey = fyne.KeyModifierControl
	}
	a.UI.toolBar = a.buildToolbar()

	// --- Menu Item for Clearing Filter ---
	a.UI.clearFilterMenuItem = fyne.NewMenuItem("Clear Filter", a.clearFilter)
	a.UI.clearFilterMenuItem.Disabled = true // Start disabled, enabled when a filter is active

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
			a.UI.clearFilterMenuItem,
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
