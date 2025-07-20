package service

import (
	"fmt"
	"fyslide/internal/scan"
)

// ViewManager encapsulates data management logic for the application's image views.
type ViewManager struct {
	images                     scan.FileItems
	permutationManager         *scan.PermutationManager
	filteredImages             scan.FileItems
	filteredPermutationManager *scan.PermutationManager
	isFiltered                 bool
	currentFilterTag           string
}

// NewViewManager creates a new ViewManager instance, initializing its fields.
func NewViewManager(images scan.FileItems, isFiltered bool, currentFilterTag string) *ViewManager {
	vm := &ViewManager{
		images:           images,
		isFiltered:       isFiltered,
		currentFilterTag: currentFilterTag,
	}
	// Initialize the PermutationManager for the full image list
	if len(images) > 0 {
		vm.permutationManager = scan.NewPermutationManager(&vm.images)
	}
	return vm
}

// SetImages updates the main images list and initializes the permutation manager.
func (vm *ViewManager) SetImages(images scan.FileItems) {
	vm.images = images
	vm.permutationManager = scan.NewPermutationManager(&vm.images)
}

// ApplyFilter applies a filter to the image list.
func (vm *ViewManager) ApplyFilter(filteredImages scan.FileItems, tag string) {
	vm.filteredImages = filteredImages
	vm.filteredPermutationManager = scan.NewPermutationManager(&vm.filteredImages)
	vm.isFiltered = true
	vm.currentFilterTag = tag
}

// ClearFilter removes any active filter.
func (vm *ViewManager) ClearFilter() {
	vm.filteredImages = nil
	vm.filteredPermutationManager = nil
	vm.isFiltered = false
	vm.currentFilterTag = ""
}

// GetCurrentList returns the active image list (filtered or full).
func (vm *ViewManager) GetCurrentList() scan.FileItems {
	if vm.isFiltered {
		return vm.filteredImages
	}
	return vm.images
}

// GetCurrentImageCount returns the number of images in the current view.
func (vm *ViewManager) GetCurrentImageCount() int {
	return len(vm.GetCurrentList())
}

// GetItemByViewIndex retrieves a FileItem from the active view (sequential or random).
func (vm *ViewManager) GetItemByViewIndex(viewIndex int, random bool) (*scan.FileItem, error) {
	// 1. Determine the active data sources based on the filter state.
	activeList := &vm.images
	activeManager := vm.permutationManager

	if vm.isFiltered {
		activeManager = vm.filteredPermutationManager
		activeList = &vm.filteredImages
	}

	// 2. Check for an empty or uninitialized data source.
	if activeList == nil || len(*activeList) == 0 {
		return nil, fmt.Errorf("active list is empty or not initialized")
	}

	// 3. Retrieve the item based on the current mode (random or sequential).
	if random {
		if activeManager == nil {
			return nil, fmt.Errorf("random mode is on but PermutationManager is not initialized")
		}
		item, err := activeManager.GetDataByShuffledIndex(viewIndex)
		if err != nil {
			return nil, fmt.Errorf("error getting data for shuffled index %d: %w", viewIndex, err)
		}
		return &item, nil
	}

	// Default to sequential mode retrieval.
	if viewIndex < 0 || viewIndex >= len(*activeList) {
		return nil, fmt.Errorf("sequential index %d out of bounds", viewIndex)
	}
	return &(*activeList)[viewIndex], nil
}

// SyncNewData synchronizes the PermutationManager with any changes in the underlying data.
func (vm *ViewManager) SyncNewData() {
	if vm.permutationManager != nil {
		vm.permutationManager.SyncNewData()
	}
	if vm.filteredPermutationManager != nil {
		vm.filteredPermutationManager.SyncNewData()
	}
}
