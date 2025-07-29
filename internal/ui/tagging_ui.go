package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"fyslide/internal/scan"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// showFilterDialog displays a dialog to select a tag for filtering.
func (a *App) showFilterDialog() {
	allTagsWithCounts, err := a.Service.ListAllTags()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get tags for filtering: %w", err), a.UI.MainWin)
		return
	}

	if len(allTagsWithCounts) == 0 {
		dialog.ShowInformation("Filter by Tag", "No tags found in the database to filter by.", a.UI.MainWin)
		return
	}

	sortMode := "By Count"
	var selectedTagName string

	filterSelector := widget.NewSelect([]string{}, nil)

	updateTagList := func() {
		sort.Slice(allTagsWithCounts, func(i, j int) bool {
			tagI := allTagsWithCounts[i]
			tagJ := allTagsWithCounts[j]

			if sortMode == "By Name" {
				return strings.ToLower(tagI.Name) < strings.ToLower(tagJ.Name)
			}
			// Default to "By Count"
			if tagI.Count != tagJ.Count {
				return tagI.Count > tagJ.Count // Descending
			}
			return strings.ToLower(tagI.Name) < strings.ToLower(tagJ.Name) // Secondary sort by name ascending
		})

		tagDisplayNames := make([]string, len(allTagsWithCounts))
		for i, tagInfo := range allTagsWithCounts {
			tagDisplayNames[i] = fmt.Sprintf("%s (%d)", tagInfo.Name, tagInfo.Count)
		}

		options := append([]string{"(Show All / Clear Filter)"}, tagDisplayNames...)
		filterSelector.Options = options

		currentSelection := filterSelector.Selected
		found := false
		for _, opt := range options {
			if opt == currentSelection {
				found = true
				break
			}
		}
		if !found && len(options) > 0 {
			if a.isFiltered {
				for _, displayName := range options {
					if strings.HasPrefix(displayName, a.currentFilterTag+" (") {
						filterSelector.SetSelected(displayName)
						found = true
						break
					}
				}
			}
			if !found {
				filterSelector.SetSelected(options[0])
			}
		}
		filterSelector.Refresh()
	}

	sortRadio := widget.NewRadioGroup([]string{"By Count", "By Name"}, func(s string) {
		sortMode = s
		updateTagList()
	})
	sortRadio.SetSelected(sortMode)

	updateTagList() // Initial population

	dialog.ShowForm("Filter by Tag", "Apply", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Sort", sortRadio),
		widget.NewFormItem("Select Tag", filterSelector),
	}, func(confirm bool) {
		if !confirm {
			return
		}

		selectedOption := filterSelector.Selected
		if selectedOption == "(Show All / Clear Filter)" {
			a.clearFilter()
		} else {
			// Extract tag name from "tag (count)"
			parts := strings.SplitN(selectedOption, " (", 2)
			if len(parts) > 0 {
				selectedTagName = parts[0]
				a.applyFilter([]string{selectedTagName})
			}
		}
	}, a.UI.MainWin)
}

// applyFilter filters the image list based on the selected tags.
func (a *App) applyFilter(tags []string) {
	if len(tags) == 0 {
		a.clearFilter()
		return
	}
	a.addLogMessage(fmt.Sprintf("Applying filter for tags: %s", strings.Join(tags, ", ")))

	initialPaths, err := a.Service.ListImagesForTag(tags[0])
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tags[0], err), a.UI.MainWin)
		a.clearFilter()
		return
	}

	if len(initialPaths) == 0 {
		a.addLogMessage(fmt.Sprintf("No images found with tag '%s'. Clearing filter.", tags[0]))
		a.clearFilter()
		return
	}

	filteredPathSet := make(map[string]struct{}, len(initialPaths))
	for _, path := range initialPaths {
		filteredPathSet[path] = struct{}{}
	}

	for i := 1; i < len(tags); i++ {
		tag := tags[i]
		nextTagPaths, err := a.Service.ListImagesForTag(tag)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to get images for tag '%s': %w", tag, err), a.UI.MainWin)
			a.clearFilter()
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
			a.addLogMessage(fmt.Sprintf("No images found with all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
			a.clearFilter()
			return
		}
	}

	newFilteredImages := make(scan.FileItems, 0, len(filteredPathSet))
	for _, item := range a.images {
		if _, ok := filteredPathSet[item.Path]; ok {
			newFilteredImages = append(newFilteredImages, item)
		}
	}

	if len(newFilteredImages) == 0 {
		a.addLogMessage(fmt.Sprintf("No currently loaded images match all selected tags. Clearing filter: %s", strings.Join(tags, ", ")))
		a.clearFilter()
		return
	}

	a.filteredImages = newFilteredImages
	a.filteredPermutationManager = scan.NewPermutationManager(&a.filteredImages)
	a.isFiltered = true
	a.currentFilterTag = strings.Join(tags, ", ")
	a.index = 0
	a.addLogMessage(fmt.Sprintf("Filter active: %d images with tags '%s'.", len(a.filteredImages), a.currentFilterTag))

	a.updateClearFilterMenuVisibility()
	a.loadAndDisplayCurrentImage()
	a.refreshThumbnailStrip()
}

// _clearFilterState resets the application's filter state variables without triggering a navigation.
func (a *App) _clearFilterState() {
	if !a.isFiltered {
		return
	}
	a.isFiltered = false
	a.currentFilterTag = ""
	a.filteredImages = nil
	a.filteredPermutationManager = nil
}

// clearFilter removes any active tag filter and navigates to the first image.
func (a *App) clearFilter() {
	if !a.isFiltered {
		return
	}
	a.addLogMessage("Filter cleared. Showing all images.")
	a._clearFilterState()
	a.updateClearFilterMenuVisibility()
	a.navigateToIndex(0)
	a.refreshThumbnailStrip()
}

// removeTagGlobally initiates the process of removing a specific tag from all images in the database.
func (a *App) removeTagGlobally(tag string) error {
	if tag == "" {
		return nil
	}
	a.addLogMessage(fmt.Sprintf("Global removal for tag '%s' started.", tag))
	successes, errors, err := a.Service.RemoveTagGlobally(tag)
	a.addLogMessage(fmt.Sprintf("Global removal for '%s': %d successes, %d errors.", tag, successes, errors))
	return err
}

// postOperationUpdate handles common UI feedback after a tag operation completes.
func (a *App) postOperationUpdate(errOp error, statusMessage string, filesAffectedCount int, wasCurrentFileAffected bool) {
	if errOp != nil {
		dialog.ShowError(errOp, a.UI.MainWin)
		a.addLogMessage(fmt.Sprintf("Error during tag operation: %v", errOp))
	} else {
		if statusMessage != "" {
			a.addLogMessage(fmt.Sprintf("Tag Operation Status: %s", statusMessage))
		}
	}

	if filesAffectedCount > 0 {
		if a.refreshTagsFunc != nil {
			a.refreshTagsFunc()
		}
		if wasCurrentFileAffected {
			imgInfo, _, err := a.ImageService.GetImageInfo(a.img.Path)
			if err == nil && imgInfo != nil {
				a.updateInfoText(imgInfo)
			} else {
				a.addLogMessage(fmt.Sprintf("Error reloading info for current image after tag op: %v", err))
			}
		}
	}
}

// handleTagOperation provides a generic framework for creating a tag operation dialog.
func (a *App) handleTagOperation(
	title string,
	verb string,
	formItems []*widget.FormItem,
	focusableWidget fyne.Focusable,
	preDialogCheck func() bool,
	execute func(confirm bool),
) {
	if a.img.Path == "" {
		dialog.ShowInformation(title, "No image loaded to "+strings.ToLower(verb)+" tags.", a.UI.MainWin)
		return
	}

	if preDialogCheck != nil && !preDialogCheck() {
		return
	}

	a.slideshowManager.Pause(true)
	if a.slideshowManager.IsPaused() {
		a.addLogMessage(fmt.Sprintf("Slideshow paused for tag %s.", strings.ToLower(verb)))
	}

	dialogCallback := func(confirm bool) {
		defer func() {
			a.slideshowManager.ResumeAfterOperation()
			if !a.slideshowManager.IsPaused() {
				a.addLogMessage("Slideshow resumed.")
			}
		}()

		if !confirm {
			return
		}
		execute(confirm)
	}

	formDialog := dialog.NewForm(title, verb, "Cancel", formItems, dialogCallback, a.UI.MainWin)

	if entry, ok := focusableWidget.(*widget.Entry); ok {
		entry.OnSubmitted = func(text string) {
			if text != "" {
				a.addLogMessage(fmt.Sprintf("Submitting %s for processing: %s", strings.ToLower(title), text))
				formDialog.Submit()
			}
		}
	}

	formDialog.Show()
	if focusableWidget != nil {
		a.UI.MainWin.Canvas().Focus(focusableWidget)
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
func (a *App) processTagsForDirectory(
	currentDir string,
	tags []string,
	operation tagOperationFunc,
	operationVerb string,
) *batchTagResult {

	a.addLogMessage(fmt.Sprintf("Batch %s directory: %s with [%s]", operationVerb, filepath.Base(currentDir), strings.Join(tags, ", ")))

	type result struct {
		// path is the file path of the image processed.
		path string
		err  error
	}

	var imagesToProcess []string
	for _, imageItem := range a.images {
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

	a.addLogMessage(fmt.Sprintf("Batch %s for [%s] in '%s' complete. Images processed: %d, Successes: %d, Errors: %d.",
		operationVerb, strings.Join(tags, ", "), filepath.Base(currentDir), batchResult.ImagesProcessed, batchResult.SuccessfulImages, batchResult.ErroredImages))
	return batchResult
}

// addTag shows a dialog to add a new tag to the current image.
func (a *App) addTag() {
	currentTags, err := a.Service.ListTagsForImage(a.img.Path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
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
		potentialTags := strings.Split(rawInput, ",")
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
			dialog.ShowInformation("Add Tags", "No valid tags entered.", a.UI.MainWin)
			return
		}

		applyToAll := applyToAllCheck.Checked
		var errAddOp error
		var statusMessage string
		filesAffected := make(map[string]bool)
		var successfulAdditions, errorsEncountered int

		if applyToAll {
			currentDir := filepath.Dir(a.img.Path)
			result := a.processTagsForDirectory(currentDir, tagsToAdd, a.Service.AddTagsToImage, "tagging")
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
			errAddOp = a.Service.AddTagsToImage(a.img.Path, tagsToAdd)
			if errAddOp == nil {
				successfulAdditions = len(tagsToAdd)
				filesAffected[a.img.Path] = true
			} else {
				errorsEncountered = len(tagsToAdd)
			}
			a.addLogMessage(fmt.Sprintf("Add to %s: %d successes, %d errors.", filepath.Base(a.img.Path), successfulAdditions, errorsEncountered))
			if errorsEncountered > 0 {
				statusMessage = fmt.Sprintf("Partial success adding tags. %d errors occurred.", errorsEncountered)
			} else if successfulAdditions > 0 {
				statusMessage = fmt.Sprintf("Added %d tag(s) to current image.", len(tagsToAdd))
			}
		}
		a.postOperationUpdate(errAddOp, statusMessage, len(filesAffected), filesAffected[a.img.Path])
	}

	a.handleTagOperation("Add Tag", "Add", formItems, tagEntry, nil, execute)
}

// removeTag shows a dialog to remove an existing tag from the current image.
func (a *App) removeTag() {
	currentTags, err := a.Service.ListTagsForImage(a.img.Path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get current tags: %w", err), a.UI.MainWin)
		return
	}

	if len(currentTags) == 0 {
		dialog.ShowInformation("Remove Tag", "This image has no tags to remove.", a.UI.MainWin)
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
			currentDir := filepath.Dir(a.img.Path)
			op := func(path string, tags []string) error {
				return a.Service.RemoveTagsFromImage(path, tags)
			}
			result := a.processTagsForDirectory(currentDir, []string{selectedTag}, op, "untagging")
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
			errRemoveOp = a.Service.RemoveTagsFromImage(a.img.Path, []string{selectedTag})
			if errRemoveOp == nil {
				imagesUntaggedCount = 1
				filesAffected[a.img.Path] = true
				statusMessage = fmt.Sprintf("Tag '%s' removed from current image.", selectedTag)
			}
			a.addLogMessage(fmt.Sprintf("Remove from %s: %d successes, %d errors.", filepath.Base(a.img.Path), imagesUntaggedCount, errorsEncountered))
		}
		a.postOperationUpdate(errRemoveOp, statusMessage, len(filesAffected), filesAffected[a.img.Path])
	}

	a.handleTagOperation("Remove Tag", "Remove", formItems, nil, nil, execute)
}
