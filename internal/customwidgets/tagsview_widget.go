package customwidgets

import (
	"fmt"
	"fyslide/internal/tagging"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	// ImageViewIndex must match the index used in the main app for the image view.
	ImageViewIndex = 0
	// TagsViewIndex must match the index used in the main app for the tags view.
	TagsViewIndex = 1

	noTagsFoundMsg       = "No tags found."
	noTagsMatchSearchMsg = "No tags match search."
	errorLoadingTagsMsg  = "Error loading tags."
)

// TagsViewHost defines the interface that the TagsView widget needs to communicate
// back to its hosting application, without creating a direct dependency.
type TagsViewHost interface {
	ListAllTags() ([]tagging.TagWithCount, error)
	AddLogMessage(message string)
	RemoveTagGlobally(tag string) error
	ApplyFilter(tags []string)
	SelectStackView(index int)
	GetWindow() fyne.Window
}

// tagListItem is a helper struct holding a tag name and its usage count.
type tagListItem struct {
	Name  string
	Count int
}

// TagsView is a custom widget that displays a searchable, sortable list of tags.
type TagsView struct {
	widget.BaseWidget
	host TagsViewHost // The hosting application that provides services and actions.

	// Internal state
	allTags              []tagListItem
	filteredDisplayData  []tagListItem
	selectedTagForAction string
	sortMode             string // "By Count" or "By Name"

	// UI Components
	searchEntry   *widget.Entry
	refreshButton *widget.Button
	removeButton  *widget.Button
	tagList       *widget.List
	messageLabel  *widget.Label
	sortToggle    *Toggle
	sortLabel     *widget.Label

	// The top-level container for the widget's content
	content fyne.CanvasObject
}

// NewTagsView creates a new instance of the TagsView widget.
func NewTagsView(host TagsViewHost) *TagsView {
	tv := &TagsView{
		host:     host,
		sortMode: "By Count", // Default sort mode
	}
	tv.ExtendBaseWidget(tv) // Important for custom widgets
	return tv
}

// CreateRenderer implements fyne.Widget.
func (tv *TagsView) CreateRenderer() fyne.WidgetRenderer {
	// --- UI Widget Creation ---
	tv.searchEntry = widget.NewEntry()
	tv.searchEntry.SetPlaceHolder("Search Tags...")

	tv.refreshButton = widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), tv.loadAndFilterTagData)
	tv.removeButton = widget.NewButtonWithIcon("Remove Tag Globally", theme.DeleteIcon(), tv.onRemoveTapped)
	tv.removeButton.Disable() // Start disabled

	tv.tagList = widget.NewList(
		func() int { return len(tv.filteredDisplayData) },
		func() fyne.CanvasObject { return widget.NewLabel("tag template") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := tv.filteredDisplayData[id]
			label := obj.(*widget.Label)
			label.SetText(fmt.Sprintf("%s (%d)", item.Name, item.Count))
		},
	)

	tv.messageLabel = widget.NewLabel(noTagsFoundMsg)
	tv.messageLabel.Alignment = fyne.TextAlignCenter
	tv.messageLabel.Wrapping = fyne.TextWrapWord

	tv.sortLabel = widget.NewLabel("Sort: By Count")
	tv.sortToggle = NewToggle(func(toggled bool) {
		if toggled {
			tv.sortMode = "By Name"
			tv.sortLabel.SetText("Sort: By Name")
		} else {
			tv.sortMode = "By Count"
			tv.sortLabel.SetText("Sort: By Count")
		}
		tv.sortTagList()
		tv.filterAndRefreshList(tv.searchEntry.Text)
	})

	// --- Wire up callbacks ---
	tv.searchEntry.OnChanged = tv.filterAndRefreshList
	tv.tagList.OnSelected = tv.onTagSelected
	tv.tagList.OnUnselected = tv.onTagUnselected

	// --- Assemble Layout ---
	sortControl := container.NewHBox(tv.sortLabel, tv.sortToggle, layout.NewSpacer())
	controls := container.NewVBox(sortControl, container.NewBorder(nil, nil, nil, tv.refreshButton, tv.searchEntry))
	listContentArea := container.NewStack(tv.messageLabel, tv.tagList)
	tv.tagList.Hide() // Initially hide list, will be shown if tags exist

	tv.content = container.NewBorder(controls, tv.removeButton, nil, nil, listContentArea)

	// --- Initial Data Load ---
	tv.loadAndFilterTagData()

	return widget.NewSimpleRenderer(tv.content)
}

// RefreshData is a public method to allow external triggers to refresh the view.
func (tv *TagsView) RefreshData() {
	tv.loadAndFilterTagData()
}

func (tv *TagsView) sortTagList() {
	sort.Slice(tv.allTags, func(i, j int) bool {
		tagI := tv.allTags[i]
		tagJ := tv.allTags[j]

		if tv.sortMode == "By Name" {
			return strings.ToLower(tagI.Name) < strings.ToLower(tagJ.Name)
		}
		// Default to "By Count"
		if tagI.Count != tagJ.Count {
			return tagI.Count > tagJ.Count // Descending
		}
		return strings.ToLower(tagI.Name) < strings.ToLower(tagJ.Name) // Secondary sort by name ascending
	})
}

func (tv *TagsView) filterAndRefreshList(searchTerm string) {
	searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
	tv.filteredDisplayData = []tagListItem{} // Clear previous filter results

	if searchTerm == "" {
		tv.filteredDisplayData = tv.allTags
	} else {
		for _, tag := range tv.allTags {
			if strings.Contains(strings.ToLower(tag.Name), searchTerm) {
				tv.filteredDisplayData = append(tv.filteredDisplayData, tag)
			}
		}
	}

	if len(tv.filteredDisplayData) == 0 {
		currentMsg := noTagsFoundMsg
		if searchTerm != "" {
			currentMsg = noTagsMatchSearchMsg
		}
		tv.messageLabel.SetText(currentMsg)
		tv.messageLabel.Show()
		tv.tagList.Hide()
	} else {
		tv.messageLabel.Hide()
		tv.tagList.Show()
	}
	tv.tagList.Refresh()
	tv.tagList.ScrollToTop()
}

func (tv *TagsView) loadAndFilterTagData() {
	fetchedTags, err := tv.host.ListAllTags()
	if err != nil {
		tv.host.AddLogMessage(fmt.Sprintf("Error loading/refreshing tags: %v", err))
		tv.allTags = []tagListItem{}
		tv.messageLabel.SetText(errorLoadingTagsMsg)
	} else {
		tv.allTags = make([]tagListItem, len(fetchedTags))
		for i, tagInfo := range fetchedTags {
			tv.allTags[i] = tagListItem{Name: tagInfo.Name, Count: tagInfo.Count}
		}
		tv.sortTagList() // Apply initial sort
	}
	tv.filterAndRefreshList(tv.searchEntry.Text)
	tv.tagList.UnselectAll() // This will trigger OnUnselected and disable the button
}

func (tv *TagsView) onRemoveTapped() {
	if tv.selectedTagForAction == "" {
		return
	}
	confirmMessage := fmt.Sprintf("Are you sure you want to remove the tag '%s' from ALL images in the database?\nThis action cannot be undone.", tv.selectedTagForAction)
	dialog.ShowConfirm("Confirm Global Tag Removal", confirmMessage, func(confirm bool) {
		if !confirm {
			return
		}
		tv.host.AddLogMessage(fmt.Sprintf("User confirmed global removal of tag: %s", tv.selectedTagForAction))
		err := tv.host.RemoveTagGlobally(tv.selectedTagForAction)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to globally remove tag '%s': %w", tv.selectedTagForAction, err), tv.host.GetWindow())
		} else {
			dialog.ShowInformation("Success", fmt.Sprintf("Tag '%s' removed globally.", tv.selectedTagForAction), tv.host.GetWindow())
			tv.loadAndFilterTagData() // Refresh list on success
		}
	}, tv.host.GetWindow())
}

func (tv *TagsView) onTagSelected(id widget.ListItemID) {
	if id < 0 || id >= len(tv.filteredDisplayData) {
		tv.selectedTagForAction = ""
		tv.removeButton.Disable()
		return
	}
	selectedItem := tv.filteredDisplayData[id]
	tv.selectedTagForAction = selectedItem.Name
	tv.removeButton.Enable()
	tv.host.ApplyFilter([]string{selectedItem.Name}) // Wrap single tag in a slice
	tv.host.SelectStackView(ImageViewIndex)
}

func (tv *TagsView) onTagUnselected(_ widget.ListItemID) {
	tv.selectedTagForAction = ""
	tv.removeButton.Disable()
}
