package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/slideshow"
	"fyslide/internal/tagging"

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
	GetSlideshowManager() *slideshow.Manager
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

// processBatchTagOperation handles batch tag operations (add/remove) for a list of image paths.
func (t *TaggingController) processBatchTagOperation(
	imagePaths []string,
	tags []string,
	operation tagOperationFunc,
	operationVerb string,
) *batchTagResult {
	t.host.AddLogMessage(fmt.Sprintf("Batch %s with [%s] for %d image(s)", operationVerb, strings.Join(tags, ", "), len(imagePaths)))

	type result struct {
		path string
		err  error
	}

	resultsChan := make(chan result, len(imagePaths))
	var wg sync.WaitGroup

	for _, path := range imagePaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			err := operation(p, tags)
			resultsChan <- result{path: p, err: err}
		}(path)
	}

	wg.Wait()
	close(resultsChan)

	batchResult := &batchTagResult{FilesAffected: make(map[string]bool)}
	for res := range resultsChan {
		batchResult.ImagesProcessed++
		if res.err != nil {
			batchResult.ErroredImages++
			if batchResult.FirstError == nil {
				batchResult.FirstError = fmt.Errorf("failed to %s on %s: %w", operationVerb, filepath.Base(res.path), res.err)
			}
		} else {
			batchResult.SuccessfulImages++
			batchResult.FilesAffected[res.path] = true
		}
	}

	t.host.AddLogMessage(fmt.Sprintf("Batch %s for [%s] complete. Images processed: %d, Successes: %d, Errors: %d.",
		operationVerb, strings.Join(tags, ", "), batchResult.ImagesProcessed, batchResult.SuccessfulImages, batchResult.ErroredImages))
	return batchResult
}

// executeTagOperation runs a tag operation (add/remove) in the background for one or more images.
func (t *TaggingController) executeTagOperation(
	tags []string,
	applyToAll bool,
	operation tagOperationFunc,
	operationVerb string,
) {
	if len(tags) == 0 {
		return // Or show an info dialog
	}

	go func() {
		t.activeOps.Add(1)
		defer t.activeOps.Add(-1)

		var imagePaths []string
		if applyToAll {
			currentDir := filepath.Dir(t.host.GetImageFullPath())
			for _, imageItem := range t.imageState.images {
				if filepath.Dir(imageItem.Path) == currentDir {
					imagePaths = append(imagePaths, imageItem.Path)
				}
			}
		} else {
			imagePaths = []string{t.host.GetImageFullPath()}
		}

		if len(imagePaths) == 0 {
			t.host.AddLogMessage(fmt.Sprintf("No images to %s.", operationVerb))
			return
		}

		t.host.AddLogMessage(fmt.Sprintf("Starting background task: %s %d image(s) with tags: [%s]", operationVerb, len(imagePaths), strings.Join(tags, ", ")))

		result := t.processBatchTagOperation(imagePaths, tags, operation, operationVerb)

		var statusMessage string
		if result.ErroredImages > 0 {
			statusMessage = fmt.Sprintf("Partial success %s. %d successes, %d errors.", operationVerb, result.SuccessfulImages, result.ErroredImages)
		} else if result.SuccessfulImages > 0 {
			statusMessage = fmt.Sprintf("Successfully finished %s %d image(s).", operationVerb, result.SuccessfulImages)
		}

		fyne.Do(func() {
			t.postOperationUpdate(result.FirstError, statusMessage, len(result.FilesAffected), result.FilesAffected[t.host.GetImageFullPath()])
		})
	}()
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
		rawInput := tagEntry.Text
		applyToAll := applyToAllCheck.Checked

		tagsToAdd := tagging.NormalizeTags(rawInput)

		if len(tagsToAdd) == 0 {
			dialog.ShowInformation("Add Tags", "No valid tags entered.", t.host.GetMainWindow())
			return
		}

		t.executeTagOperation(tagsToAdd, applyToAll, t.service.AddTagsToImage, "adding tags")
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
		applyToAll := removeFromAllCheck.Checked
		tagToRemove := selectedTag

		op := func(path string, tags []string) error {
			return t.service.RemoveTagsFromImage(path, tags)
		}
		t.executeTagOperation([]string{tagToRemove}, applyToAll, op, "removing tag")
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
