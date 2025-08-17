package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"fyslide/internal/scan"
	"fyslide/internal/service"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type TaggingController struct {
	service    *service.Service
	imageState *ImageState
	app        *App
}

// NewTaggingController creates a new instance of TaggingController.
func NewTaggingController(a *App, s *service.Service, i *ImageState) *TaggingController {
	return &TaggingController{app: a, service: s, imageState: i}
}

// applyFilter filters the image list based on the selected tags.
func (t *TaggingController) ApplyFilter(tags []string) {
	if len(tags) == 0 {
		t.clearFilter()
		return
	}
	t.app.AddLogMessage(fmt.Sprintf("Applying filter for tags: %s", strings.Join(tags, ", ")))

	initialPaths, err := t.service.ListImagesForTag(tags[0])
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tags[0], err), t.app.UI.MainWin)
		t.clearFilter()
		return
	}

	if len(initialPaths) == 0 {
		t.app.AddLogMessage(fmt.Sprintf("No images found with tag '%s'. Clearing filter.", tags[0]))
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
			dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tag, err), t.app.UI.MainWin)
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
			t.app.AddLogMessage(fmt.Sprintf("No images found with all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
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
		t.app.AddLogMessage(fmt.Sprintf("No currently loaded images match all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
		t.clearFilter()
		return
	}

	filterTag := strings.Join(tags, ", ")
	t.imageState.ApplyFilter(newFilteredImages, filterTag)
	t.app.AddLogMessage(fmt.Sprintf("Filter active: %d images with tags '%s'.", len(newFilteredImages), filterTag))

	t.app.updateClearFilterMenuVisibility()
	t.app.loadAndDisplayCurrentImage()
	t.app.UI.thumbnailBrowser.Refresh()
}

// clearFilter removes any active tag filter and navigates to the first image.
func (t *TaggingController) clearFilter() {
	if !t.imageState.IsFiltered() {
		return
	}
	t.app.AddLogMessage("Filter cleared. Showing all images.")
	t.imageState.ClearFilter()
	t.app.updateClearFilterMenuVisibility()
	t.app.Navigation.NavigateToIndex(0)
	t.app.UI.thumbnailBrowser.Refresh()
}

// removeTagGlobally initiates the process of removing a specific tag from all images in the database.
func (t *TaggingController) RemoveTagGlobally(tag string) error {
	if tag == "" {
		return nil
	}
	t.app.AddLogMessage(fmt.Sprintf("Global removal for tag '%s' started.", tag))
	successes, errors, err := t.service.RemoveTagGlobally(tag)
	t.app.AddLogMessage(fmt.Sprintf("Global removal for '%s': %d successes, %d errors.", tag, successes, errors))
	return err
}

// postOperationUpdate handles common UI feedback after a tag operation completes.
func (t *TaggingController) postOperationUpdate(errOp error, statusMessage string, filesAffectedCount int, wasCurrentFileAffected bool) {
	if errOp != nil {
		dialog.ShowError(errOp, t.app.UI.MainWin)
		t.app.AddLogMessage(fmt.Sprintf("Error during tag operation: %v", errOp))
	} else {
		if statusMessage != "" {
			t.app.AddLogMessage(fmt.Sprintf("Tag Operation Status: %s", statusMessage))
		}
	}

	if filesAffectedCount > 0 {
		if t.app.refreshTagsFunc != nil {
			t.app.refreshTagsFunc()
		}
		if wasCurrentFileAffected {
			imgInfo, _, err := t.app.ImageService.GetImageInfo(t.app.img.Path)
			if err == nil && imgInfo != nil {
				t.app.updateInfoText(imgInfo)
			} else {
				t.app.AddLogMessage(fmt.Sprintf("Error reloading info for current image after tag op: %v", err))
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
	if t.app.img.Path == "" {
		dialog.ShowInformation(title, "No image loaded to "+strings.ToLower(verb)+" tags.", t.app.UI.MainWin)
		return
	}

	if preDialogCheck != nil && !preDialogCheck() {
		return
	}

	t.app.slideshowManager.Pause(true)
	if t.app.slideshowManager.IsPaused() {
		t.app.AddLogMessage(fmt.Sprintf("Slideshow paused for tag %s.", strings.ToLower(verb)))
	}

	dialogCallback := func(confirm bool) {
		defer func() {
			t.app.slideshowManager.ResumeAfterOperation()
			if !t.app.slideshowManager.IsPaused() {
				t.app.AddLogMessage("Slideshow resumed.")
			}
		}()

		if !confirm {
			return
		}
		execute(confirm)
	}

	formDialog := dialog.NewForm(title, verb, "Cancel", formItems, dialogCallback, t.app.UI.MainWin)

	if entry, ok := focusableWidget.(*widget.Entry); ok {
		entry.OnSubmitted = func(text string) {
			if text != "" {
				t.app.AddLogMessage(fmt.Sprintf("Submitting %s for processing: %s", strings.ToLower(title), text))
				formDialog.Submit()
			}
		}
	}

	formDialog.Show()
	if focusableWidget != nil {
		t.app.UI.MainWin.Canvas().Focus(focusableWidget)
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

	t.app.AddLogMessage(fmt.Sprintf("Batch %s directory: %s with [%s]", operationVerb, filepath.Base(currentDir), strings.Join(tags, ", ")))

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

	t.app.AddLogMessage(fmt.Sprintf("Batch %s for [%s] in '%s' complete. Images processed: %d, Successes: %d, Errors: %d.",
		operationVerb, strings.Join(tags, ", "), filepath.Base(currentDir), batchResult.ImagesProcessed, batchResult.SuccessfulImages, batchResult.ErroredImages))
	return batchResult
}

// addTag shows a dialog to add a new tag to the current image.
func (t *TaggingController) addTag() {
	currentTags, err := t.service.ListTagsForImage(t.app.img.Path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), t.app.UI.MainWin)
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

	execute := func(confirm bool) {
		rawInput := tagEntry.Text
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
			dialog.ShowInformation("Add Tags", "No valid tags entered.", t.app.UI.MainWin)
			return
		}

		applyToAll := applyToAllCheck.Checked
		var errAddOp error
		var statusMessage string
		filesAffected := make(map[string]bool)
		var successfulAdditions, errorsEncountered int

		if applyToAll {
			currentDir := filepath.Dir(t.app.img.Path)
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
			errAddOp = t.service.AddTagsToImage(t.app.img.Path, tagsToAdd)
			if errAddOp == nil {
				successfulAdditions = len(tagsToAdd)
				filesAffected[t.app.img.Path] = true
			} else {
				errorsEncountered = len(tagsToAdd)
			}
			t.app.AddLogMessage(fmt.Sprintf("Add to %s: %d successes, %d errors.", filepath.Base(t.app.img.Path), successfulAdditions, errorsEncountered))
			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Partial success adding tags. %d errors occurred.", errorsEncountered)
			} else if successfulAdditions > 0 {
				statusMessage = fmt.Sprintf("Added %d tag(s) to current image.", len(tagsToAdd))
			}
		}
		t.postOperationUpdate(errAddOp, statusMessage, len(filesAffected), filesAffected[t.app.img.Path])
	}

	t.handleTagOperation("Add Tag", "Add", formItems, tagEntry, nil, execute)
}

// removeTag shows a dialog to remove an existing tag from the current image.
func (t *TaggingController) removeTag() {
	currentTags, err := t.service.ListTagsForImage(t.app.img.Path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), t.app.UI.MainWin)
		return
	}

	if len(currentTags) == 0 {
		dialog.ShowInformation("Remove Tag", "This image has no tags to remove.", t.app.UI.MainWin)
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

	execute := func(confirm bool) {
		if selectedTag == "" {
			return
		}
		applyToAll := removeFromAllCheck.Checked
		var errRemoveOp error
		var statusMessage string
		var imagesUntaggedCount, errorsEncountered int
		filesAffected := make(map[string]bool)

		if applyToAll {
			currentDir := filepath.Dir(t.app.img.Path)
			op := func(path string, tags []string) error {
				return t.service.RemoveTagsFromImage(path, tags)
			}
			result := t.processTagsForDirectory(currentDir, []string{selectedTag}, op, "untagging")
			imagesUntaggedCount = result.SuccessfulImages
			errorsEncountered = result.ErroredImages
			errRemoveOp = result.FirstError
			filesAffected = result.FilesAffected

			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Partial success removing tag. %d images untagged, %d errors.", imagesUntaggedCount, errorsEncountered)
			} else if imagesUntaggedCount > 0 {
				statusMessage = fmt.Sprintf("Tag '%s' removed from %d images in directory %s.", selectedTag, imagesUntaggedCount, filepath.Base(currentDir))
			}
		} else {
			errRemoveOp = t.service.RemoveTagsFromImage(t.app.img.Path, []string{selectedTag})
			if errRemoveOp == nil {
				imagesUntaggedCount = 1
				filesAffected[t.app.img.Path] = true
				statusMessage = fmt.Sprintf("Tag '%s' removed from current image.", selectedTag)
			}
			t.app.AddLogMessage(fmt.Sprintf("Remove from %s: %d successes, %d errors.", filepath.Base(t.app.img.Path), imagesUntaggedCount, errorsEncountered))
		}
		t.postOperationUpdate(errRemoveOp, statusMessage, len(filesAffected), filesAffected[t.app.img.Path])
	}

	t.handleTagOperation("Remove Tag", "Remove", formItems, nil, nil, execute)
}
