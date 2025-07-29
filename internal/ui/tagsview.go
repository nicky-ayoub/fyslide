package ui

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

// filterAndRefreshList updates the list display based on the current search term.
func (c *tagListController) filterAndRefreshList(searchTerm string) {
	searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
	c.filteredDisplayData = []tagListItem{} // Clear previous filter results

	if searchTerm == "" {
		c.filteredDisplayData = c.allTags
	} else {
		for _, tag := range c.allTags {
			if strings.Contains(strings.ToLower(tag.Name), searchTerm) {
				c.filteredDisplayData = append(c.filteredDisplayData, tag)
			}
		}
	}

	if len(c.filteredDisplayData) == 0 {
		currentMsg := noTagsFoundMsg
		if searchTerm != "" {
			currentMsg = noTagsMatchSearchMsg
		}
		c.messageLabel.SetText(currentMsg)
		c.messageLabel.Show()
		c.tagList.Hide()
	} else {
		c.messageLabel.Hide()
		c.tagList.Show()
		c.tagList.Refresh()
		c.tagList.ScrollToTop()
	}
}

// loadAndFilterTagData reloads all tag data from the service, sorts it, and refreshes the view.
func (c *tagListController) loadAndFilterTagData() {
	fetchedTags, err := c.app.Service.ListAllTags()
	if err != nil {
		c.app.addLogMessage(fmt.Sprintf("Error loading/refreshing tags: %v", err))
		c.allTags = []tagListItem{}
		c.messageLabel.SetText(errorLoadingTagsMsg)
	} else {
		c.allTags = make([]tagListItem, len(fetchedTags))
		for i, tagInfo := range fetchedTags {
			c.allTags[i] = tagListItem{Name: tagInfo.Name, Count: tagInfo.Count}
		}
		// Sort by count (descending), then by name (ascending) for ties.
		sort.Slice(c.allTags, func(i, j int) bool {
			if c.allTags[i].Count != c.allTags[j].Count {
				return c.allTags[i].Count > c.allTags[j].Count
			}
			return c.allTags[i].Name < c.allTags[j].Name
		})
	}
	c.filterAndRefreshList(c.searchEntry.Text)
	c.tagList.UnselectAll() // This will trigger OnUnselected and disable the button
}

// onRemoveTapped handles the logic for the "Remove Tag Globally" button.
func (c *tagListController) onRemoveTapped() {
	if c.selectedTagForAction == "" {
		return
	}
	confirmMessage := fmt.Sprintf("Are you sure you want to remove the tag '%s' from ALL images in the database?\nThis action cannot be undone.", c.selectedTagForAction)
	dialog.ShowConfirm("Confirm Global Tag Removal", confirmMessage, func(confirm bool) {
		if !confirm {
			return
		}
		c.app.addLogMessage(fmt.Sprintf("User confirmed global removal of tag: %s", c.selectedTagForAction))
		err := c.app.removeTagGlobally(c.selectedTagForAction)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to globally remove tag '%s': %w", c.selectedTagForAction, err), c.app.UI.MainWin)
		} else {
			dialog.ShowInformation("Success", fmt.Sprintf("Tag '%s' removed globally.", c.selectedTagForAction), c.app.UI.MainWin)
			c.loadAndFilterTagData() // Refresh list on success
		}
	}, c.app.UI.MainWin)
}

// onTagSelected handles when a user clicks a tag in the list.
func (c *tagListController) onTagSelected(id widget.ListItemID) {
	if id < 0 || id >= len(c.filteredDisplayData) {
		c.selectedTagForAction = ""
		c.removeButton.Disable()
		return
	}
	selectedItem := c.filteredDisplayData[id]
	c.selectedTagForAction = selectedItem.Name
	c.removeButton.Enable()
	c.app.applyFilter([]string{selectedItem.Name}) // Wrap single tag in a slice
	if c.app.UI.contentStack != nil {
		c.app.selectStackView(imageViewIndex)
	}
}

// onTagUnselected handles when a user deselects a tag.
func (c *tagListController) onTagUnselected(_ widget.ListItemID) {
	c.selectedTagForAction = ""
	c.removeButton.Disable()
}
