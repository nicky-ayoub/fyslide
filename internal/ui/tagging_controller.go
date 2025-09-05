package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/slideshow"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// TaggingHost interface defines the methods required by the TaggingController
type TaggingHost interface {
	AddLogMessage(msg string)
	LoadAndDisplayCurrentImage()
	UpdateClearFilterMenuVisibility()
	GetMainWindow() fyne.Window
	GetImageFullPath() string
	GetImageService() *service.ImageService
	RefreshTags()                           // Refreshes the list of all tags in the tag view
	UpdateInfoText(info *service.ImageInfo) // Refreshes the info panel for the current image
	GetSlideshowManager() *slideshow.SlideshowManager
	NavigateToIndex(index int)
}

// TaggingController manages tagging operations and interactions with the tagging service.
type TaggingController struct {
	service    *service.Service
	imageState *ImageState
	host       TaggingHost
	activeOps  atomic.Int32
}

// NewTaggingController creates a new instance of TaggingController.
func NewTaggingController(h TaggingHost, s *service.Service, i *ImageState) *TaggingController {
	return &TaggingController{host: h, service: s, imageState: i}
}

// IsBusy returns true if there are background tagging operations in progress.
func (t *TaggingController) IsBusy() bool {
	return t.activeOps.Load() > 0
}

// ApplyFilter filters the image list based on the selected tags.
func (t *TaggingController) ApplyFilter(tags []string) {
	if len(tags) == 0 {
		t.clearFilter()
		return
	}
	t.host.AddLogMessage(fmt.Sprintf("Applying filter for tags: %s", strings.Join(tags, ", ")))

	initialPaths, err := t.service.ListImagesForTag(tags[0])
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tags[0], err), t.host.GetMainWindow())
		t.clearFilter()
		return
	}

	if len(initialPaths) == 0 {
		t.host.AddLogMessage(fmt.Sprintf("No images found with tag '%s'. Clearing filter.", tags[0]))
		t.clearFilter()
		return
	}

	filteredPathSet := make(map[string]struct{}, len(initialPaths))
	for _, path := range initialPaths {
		filteredPathSet[path] = struct{}{}
	}

	for i := 1; i < len(tags); i++ {
		tag := tags[i]
		nextTagPaths, err := t.service.ListImagesForTag(tag)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tag, err), t.host.GetMainWindow())
			t.clearFilter()
			return
		}

		intersection := make(map[string]struct{})
		for _, path := range nextTagPaths {
			if _, ok := filteredPathSet[path]; ok {
				intersection[path] = struct{}{}
			}
		}
		filteredPathSet = intersection

		if len(filteredPathSet) == 0 {
			t.host.AddLogMessage(fmt.Sprintf("No images found with all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
			t.clearFilter()
			return
		}
	}

	newFilteredImages := make(scan.FileItems, 0, len(filteredPathSet))
	for _, item := range t.imageState.images {
		if _, ok := filteredPathSet[item.Path]; ok {
			newFilteredImages = append(newFilteredImages, item)
		}
	}

	if len(newFilteredImages) == 0 {
		t.host.AddLogMessage(fmt.Sprintf("No currently loaded images match all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
		t.clearFilter()
		return
	}

	filterTag := strings.Join(tags, ", ")
	t.imageState.ApplyFilter(newFilteredImages, filterTag)
	t.host.AddLogMessage(fmt.Sprintf("Filter active: %d images with tags '%s'.", len(newFilteredImages), filterTag))

	t.host.UpdateClearFilterMenuVisibility()
	t.host.LoadAndDisplayCurrentImage()
}

// clearFilter removes any active tag filter and navigates to the first image.
func (t *TaggingController) clearFilter() {
	if !t.imageState.IsFiltered() {
		return
	}
	currentPath := t.host.GetImageFullPath() // Get path *before* clearing
	t.host.AddLogMessage("Filter cleared. Showing all images.")
	t.imageState.ClearFilter(currentPath) // Pass the path to preserve position
	t.host.UpdateClearFilterMenuVisibility()
	// No longer need to navigate to 0, just load the new current image
	t.host.LoadAndDisplayCurrentImage()
}

// RemoveTagGlobally initiates the process of removing a specific tag from all images in the database.
func (t *TaggingController) RemoveTagGlobally(tag string) error {
	if tag == "" {
		return nil
	}
	t.host.AddLogMessage(fmt.Sprintf("Global removal for tag '%s' started.", tag))
	successes, errors, err := t.service.RemoveTagGlobally(tag)
	t.host.AddLogMessage(fmt.Sprintf("Global removal for '%s': %d successes, %d errors.", tag, successes, errors))
	return err
}

// postOperationUpdate handles common UI feedback after a tag operation completes.
func (t *TaggingController) postOperationUpdate(errOp error, statusMessage string, filesAffectedCount int, wasCurrentFileAffected bool) {
	if errOp != nil {
		dialog.ShowError(errOp, t.host.GetMainWindow())
		t.host.AddLogMessage(fmt.Sprintf("Error during tag operation: %v", errOp))
	} else {
		if statusMessage != "" {
			t.host.AddLogMessage(fmt.Sprintf("Tag Operation Status: %s", statusMessage))
		}
	}

	if filesAffectedCount > 0 {
		t.host.RefreshTags()

		// If the currently viewed file was changed, we need to refresh its info panel.
		if wasCurrentFileAffected {
			imgInfo, _, err := t.host.GetImageService().GetImageInfo(t.host.GetImageFullPath()) // Re-fetch image info
			if err == nil && imgInfo != nil {
				t.host.UpdateInfoText(imgInfo) // UpdateInfoText will now fetch its own tags
			} else { // Handle case where image info fails to load
				t.host.AddLogMessage(fmt.Sprintf("Error reloading info for current image after tag op: %v", err))
			}
		}
	}
}

// handleTagOperation provides a generic framework for creating a tag operation dialog.
func (t *TaggingController) handleTagOperation(
	title string,
	verb string,
	formItems []*widget.FormItem,
	focusableWidget fyne.Focusable,
	preDialogCheck func() bool,
	execute func(confirm bool),
) {
	if t.host.GetImageFullPath() == "" {
		dialog.ShowInformation(title, "No image loaded to "+strings.ToLower(verb)+" tags.", t.host.GetMainWindow())
		return
	}

	if preDialogCheck != nil && !preDialogCheck() {
		return
	}

	t.host.GetSlideshowManager().Pause(true)
	if t.host.GetSlideshowManager().IsPaused() {
		t.host.AddLogMessage(fmt.Sprintf("Slideshow paused for tag %s.", strings.ToLower(verb)))
	}

	dialogCallback := func(confirm bool) {
		defer func() {
			t.host.GetSlideshowManager().ResumeAfterOperation()
			if !t.host.GetSlideshowManager().IsPaused() {
				t.host.AddLogMessage("Slideshow resumed.")
			}
		}()

		if !confirm {
			return
		}
		execute(confirm)
	}

	formDialog := dialog.NewForm(title, verb, "Cancel", formItems, dialogCallback, t.host.GetMainWindow())

	if entry, ok := focusableWidget.(*widget.Entry); ok {
		entry.OnSubmitted = func(text string) {
			if text != "" {
				t.host.AddLogMessage(fmt.Sprintf("Submitting %s for processing: %s", strings.ToLower(title), text))
				formDialog.Submit()
			}
		}
	}

	formDialog.Show()
	if focusableWidget != nil {
		t.host.GetMainWindow().Canvas().Focus(focusableWidget)
	}
}

// tagOperationFunc defines a function that performs a tag operation on a single image path with a set of tags.
type tagOperationFunc func(imagePath string, tags []string) error

// batchTagResult holds the aggregated results of a batch tag operation.
type batchTagResult struct {
	SuccessfulImages int
	ErroredImages    int
	ImagesProcessed  int
	FirstError       error
	FilesAffected    map[string]bool
}

// processTagsForDirectory handles batch tag operations (add/remove) for all images in a directory.
func (t *TaggingController) processTagsForDirectory(
	currentDir string,
	tags []string,
	operation tagOperationFunc,
	operationVerb string,
) *batchTagResult {

	t.host.AddLogMessage(fmt.Sprintf("Batch %s directory: %s with [%s]", operationVerb, filepath.Base(currentDir), strings.Join(tags, ", ")))

	type result struct {
		// path is the file path of the image processed.
		path string
		err  error
	}

	var imagesToProcess []string
	for _, imageItem := range t.imageState.images {
		if filepath.Dir(imageItem.Path) == currentDir {
			imagesToProcess = append(imagesToProcess, imageItem.Path)
		}
	}

	if len(imagesToProcess) == 0 {
		return &batchTagResult{FilesAffected: make(map[string]bool)}
	}

	resultsChan := make(chan result, len(imagesToProcess))
	var wg sync.WaitGroup

	for _, path := range imagesToProcess {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			err := operation(p, tags)
			resultsChan <- result{path: p, err: err}
		}(path)
	}

	wg.Wait()
	close(resultsChan)

	batchResult := &batchTagResult{
		FilesAffected: make(map[string]bool),
	}
	for res := range resultsChan {
		batchResult.ImagesProcessed++
		if res.err != nil {
			batchResult.ErroredImages++
			if batchResult.FirstError == nil {
				batchResult.FirstError = fmt.Errorf("failed to %s %s: %w", operationVerb, filepath.Base(res.path), res.err)
			}
		} else {
			batchResult.SuccessfulImages++
			batchResult.FilesAffected[res.path] = true
		}
	}

	t.host.AddLogMessage(fmt.Sprintf("Batch %s for [%s] in '%s' complete. Images processed: %d, Successes: %d, Errors: %d.",
		operationVerb, strings.Join(tags, ", "), filepath.Base(currentDir), batchResult.ImagesProcessed, batchResult.SuccessfulImages, batchResult.ErroredImages))
	return batchResult
}

// showAddTagDialog displays a dialog for adding tags to the current image or all images in the directory.
func (t *TaggingController) showAddTagDialog() {
	currentTags, err := t.service.ListTagsForImage(t.host.GetImageFullPath())
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), t.host.GetMainWindow())
		return
	}

	tagEntry := widget.NewEntry()
	tagEntry.SetPlaceHolder("Enter tag(s) separated by commas...")

	currentTagsText := "Current tags: (none)"
	if len(currentTags) > 0 {
		currentTagsText = fmt.Sprintf("Current tags: %s", strings.Join(currentTags, ", "))
	}
	currentTagsLabel := widget.NewLabel(currentTagsText)

	applyToAllCheck := widget.NewCheck("Apply tag(s) to all images in this directory", nil)
	applyToAllCheck.SetChecked(true)

	formItems := []*widget.FormItem{
		widget.NewFormItem("", currentTagsLabel),
		widget.NewFormItem("New Tag(s)", tagEntry),
		widget.NewFormItem("", applyToAllCheck),
	}

	execute := func(_ bool) {
		// Capture values from UI elements before starting the goroutine.
		rawInput := tagEntry.Text
		applyToAll := applyToAllCheck.Checked

		// Run the potentially long-running tag operation in a background goroutine
		// so the UI doesn't freeze.
		go func() {
			t.activeOps.Add(1)
			defer t.activeOps.Add(-1)

			potentialTags := regexp.MustCompile(`[,.]`).Split(rawInput, -1)
			var tagsToAdd []string
			uniqueTags := make(map[string]bool)
			for _, pt := range potentialTags {
				tag := strings.ToLower(strings.TrimSpace(pt))
				if tag != "" && !uniqueTags[tag] {
					tagsToAdd = append(tagsToAdd, tag)
					uniqueTags[tag] = true
				}
			}

			if len(tagsToAdd) == 0 {
				fyne.Do(func() {
					dialog.ShowInformation("Add Tags", "No valid tags entered.", t.host.GetMainWindow())
				})
				return
			}

			t.host.AddLogMessage(fmt.Sprintf("Starting background task to add tags: [%s]", strings.Join(tagsToAdd, ", ")))

			var errAddOp error
			var statusMessage string
			filesAffected := make(map[string]bool)
			var successfulAdditions, errorsEncountered int

			if applyToAll {
				currentDir := filepath.Dir(t.host.GetImageFullPath())
				result := t.processTagsForDirectory(currentDir, tagsToAdd, t.service.AddTagsToImage, "tagging")
				successfulAdditions = result.SuccessfulImages * len(tagsToAdd)
				errorsEncountered = result.ErroredImages * len(tagsToAdd)
				errAddOp = result.FirstError
				filesAffected = result.FilesAffected

				if errorsEncountered > 0 {
					statusMessage = fmt.Sprintf("Partial success adding tags to %d images. %d errors occurred.", len(filesAffected), errorsEncountered)
				} else if successfulAdditions > 0 {
					statusMessage = fmt.Sprintf("Added tag(s) to %d images in %s.", len(filesAffected), filepath.Base(currentDir))
				}
			} else {
				errAddOp = t.service.AddTagsToImage(t.host.GetImageFullPath(), tagsToAdd)
				if errAddOp == nil {
					successfulAdditions = len(tagsToAdd)
					filesAffected[t.host.GetImageFullPath()] = true
				} else {
					errorsEncountered = len(tagsToAdd)
				}
				t.host.AddLogMessage(fmt.Sprintf("Add to %s: %d successes, %d errors.", filepath.Base(t.host.GetImageFullPath()), successfulAdditions, errorsEncountered))
				if errorsEncountered > 0 {
					statusMessage = fmt.Sprintf("Partial success adding tags. %d errors occurred.", errorsEncountered)
				} else if successfulAdditions > 0 {
					statusMessage = fmt.Sprintf("Added %d tag(s) to current image.", len(tagsToAdd))
				}
			}
			fyne.Do(func() {
				t.postOperationUpdate(errAddOp, statusMessage, len(filesAffected), filesAffected[t.host.GetImageFullPath()])
			})
		}()
	}

	t.handleTagOperation(
		"Add Tag",
		"Add",
		formItems,
		tagEntry,
		nil, // No pre-dialog check needed
		execute,
	)
}

// showFilterByTagsDialog displays a dialog to filter images by multiple tags.
func (t *TaggingController) showFilterByTagsDialog() {
	allTagsWithCount, err := t.service.ListAllTags()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to list all tags: %w", err), t.host.GetMainWindow())
		return
	}

	if len(allTagsWithCount) == 0 {
		dialog.ShowInformation("Filter by Tags", "No tags found in the database to filter by.", t.host.GetMainWindow())
		return
	}

	// Sort tags alphabetically for display
	sort.Slice(allTagsWithCount, func(i, j int) bool {
		return allTagsWithCount[i].Name < allTagsWithCount[j].Name
	})

	// Map to store the checked state of each tag
	checkedTags := make(map[string]bool)

	// Create a list of check widgets
	content := container.NewVBox()
	for _, tagInfo := range allTagsWithCount {
		// Capture tag name for the closure
		tagName := tagInfo.Name
		check := widget.NewCheck(fmt.Sprintf("%s (%d)", tagName, tagInfo.Count), func(checked bool) {
			checkedTags[tagName] = checked
		})
		content.Add(check)
	}
	scrollableContent := container.NewScroll(content)

	// Create and show the dialog
	d := dialog.NewCustomConfirm(
		"Filter by Tags",
		"Apply",
		"Cancel",
		scrollableContent,
		func(confirm bool) {
			if !confirm {
				return
			}

			var selectedTags []string
			// Iterate over the original sorted list to maintain order
			for _, tagInfo := range allTagsWithCount {
				if checkedTags[tagInfo.Name] {
					selectedTags = append(selectedTags, tagInfo.Name)
				}
			}
			t.ApplyFilter(selectedTags) // ApplyFilter handles empty slice by clearing the filter
		},
		t.host.GetMainWindow(),
	)

	d.Resize(fyne.NewSize(400, 500))
	d.Show()
}

// showRemoveTagDialog displays a dialog for removing a tag from the current image or all images in the directory.
func (t *TaggingController) showRemoveTagDialog() {
	currentTags, err := t.service.ListTagsForImage(t.host.GetImageFullPath())
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), t.host.GetMainWindow())
		return
	}

	if len(currentTags) == 0 {
		dialog.ShowInformation("Remove Tag", "This image has no tags to remove.", t.host.GetMainWindow())
		return
	}

	var selectedTag string
	tagSelector := widget.NewSelect(currentTags, func(s string) { selectedTag = s })
	tagSelector.SetSelected(currentTags[0])
	selectedTag = currentTags[0]
	removeFromAllCheck := widget.NewCheck("Remove tag(s) from all images in this directory", nil)
	formItems := []*widget.FormItem{
		widget.NewFormItem("Select Tag to Remove", tagSelector),
		widget.NewFormItem("", removeFromAllCheck),
	}

	execute := func(_ bool) {
		// Capture values from UI elements before starting the goroutine.
		applyToAll := removeFromAllCheck.Checked
		tagToRemove := selectedTag

		// Run the potentially long-running tag operation in a background goroutine.
		go func() {
			t.activeOps.Add(1)
			defer t.activeOps.Add(-1)

			var errRemoveOp error
			var statusMessage string
			var imagesUntaggedCount, errorsEncountered int
			filesAffected := make(map[string]bool)

			if applyToAll {
				currentDir := filepath.Dir(t.host.GetImageFullPath())
				op := func(path string, tags []string) error {
					return t.service.RemoveTagsFromImage(path, tags)
				}
				result := t.processTagsForDirectory(currentDir, []string{tagToRemove}, op, "untagging")
				imagesUntaggedCount = result.SuccessfulImages
				errorsEncountered = result.ErroredImages
				errRemoveOp = result.FirstError
				filesAffected = result.FilesAffected

				if errorsEncountered > 0 {
					statusMessage = fmt.Sprintf("Partial success removing tag. %d images untagged, %d errors.", imagesUntaggedCount, errorsEncountered)
				} else if imagesUntaggedCount > 0 {
					statusMessage = fmt.Sprintf("Tag '%s' removed from %d images in directory %s.", tagToRemove, imagesUntaggedCount, filepath.Base(currentDir))
				}
			} else {
				errRemoveOp = t.service.RemoveTagsFromImage(t.host.GetImageFullPath(), []string{tagToRemove})
				if errRemoveOp == nil {
					imagesUntaggedCount = 1
					filesAffected[t.host.GetImageFullPath()] = true
					statusMessage = fmt.Sprintf("Tag '%s' removed from current image.", tagToRemove)
				} else {
					errorsEncountered = 1
				}
				t.host.AddLogMessage(fmt.Sprintf("Remove from %s: %d successes, %d errors.", filepath.Base(t.host.GetImageFullPath()), imagesUntaggedCount, errorsEncountered))
			}
			fyne.Do(func() {
				t.postOperationUpdate(errRemoveOp, statusMessage, len(filesAffected), filesAffected[t.host.GetImageFullPath()])
			})
		}()
	}

	t.handleTagOperation(
		"Remove Tag",
		"Remove",
		formItems,
		tagSelector,
		nil, // Pre-dialog check is now handled at the top of this function.
		execute,
	)
}
