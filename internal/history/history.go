// Package history manages the navigation history for an application.
package history

// HistoryManager manages the navigation history.
type HistoryManager struct {
	stack        []string
	currentIndex int
	capacity     int
}

// NewHistoryManager creates a new HistoryManager.
// If capacity is 0, history is disabled. Negative capacity is treated as 0.
func NewHistoryManager(capacity int) *HistoryManager {
	if capacity < 0 {
		capacity = 0 // Ensure capacity is not negative
	}
	return &HistoryManager{
		stack:        make([]string, 0, capacity),
		currentIndex: -1,
		capacity:     capacity,
	}
}

// RecordNavigation records a new navigation path.
// If navigating to a new path after going back, future history is cleared.
func (hm *HistoryManager) RecordNavigation(path string) {
	if hm.capacity == 0 {
		return
	}

	// If currentIndex is not at the end of the stack (e.g., user went back, then chose a new path),
	// truncate the "future" part of history.
	if hm.currentIndex != -1 && hm.currentIndex < len(hm.stack)-1 {
		hm.stack = hm.stack[:hm.currentIndex+1] // Truncate, currentIndex is now at the new end
	}

	// Avoid adding if it's the exact same path as the current top of history.
	// This check is valid after potential truncation above.
	// hm.currentIndex >= 0 ensures we don't try to access hm.stack[-1] if stack is empty.
	if hm.currentIndex >= 0 && hm.stack[hm.currentIndex] == path {
		return // Path is the same as current; no change needed.
	}

	// Add the new path
	hm.stack = append(hm.stack, path)

	// Trim history if it exceeds capacity (remove from the beginning)
	if len(hm.stack) > hm.capacity {
		hm.stack = hm.stack[len(hm.stack)-hm.capacity:]
	}
	// After adding a new path (and potentially trimming), currentIndex points to the new last item.
	hm.currentIndex = len(hm.stack) - 1
}

// NavigateBack attempts to get the previous path from history.
// Returns the path and true if successful, or an empty string and false.
func (hm *HistoryManager) NavigateBack() (path string, ok bool) {
	if hm.capacity == 0 {
		return "", false
	}
	if hm.currentIndex <= 0 { // Need to be at least at the second item (index 1) to go back
		return "", false
	}
	hm.currentIndex--
	return hm.stack[hm.currentIndex], true
}

// NavigateForward attempts to get the next path from history.
// Returns the path and true if successful, or an empty string and false.
func (hm *HistoryManager) NavigateForward() (path string, ok bool) {
	if hm.capacity == 0 {
		return "", false
	}
	if hm.currentIndex == -1 || hm.currentIndex >= len(hm.stack)-1 {
		return "", false
	}
	hm.currentIndex++
	return hm.stack[hm.currentIndex], true
}

// RemovePath removes all occurrences of a given path from the history stack
// and adjusts the currentIndex accordingly.
func (hm *HistoryManager) RemovePath(pathToRemove string) {
	if hm.capacity == 0 || len(hm.stack) == 0 {
		return
	}

	newStack := make([]string, 0, len(hm.stack))
	newCurrentIndex := hm.currentIndex

	itemsRemovedBeforeCurrent := 0
	currentWasRemoved := false

	for i, p := range hm.stack {
		if p == pathToRemove {
			if i < hm.currentIndex {
				itemsRemovedBeforeCurrent++
			} else if i == hm.currentIndex {
				currentWasRemoved = true
			}
		} else {
			newStack = append(newStack, p)
		}
	}

	if len(newStack) == len(hm.stack) { // No items were removed
		return
	}

	hm.stack = newStack
	// Adjust currentIndex
	if currentWasRemoved {
		// If the current item was removed, try to point to the item before it,
		// or the new end if it was the last, or the beginning if it was the first.
		newCurrentIndex = hm.currentIndex - itemsRemovedBeforeCurrent - 1
	} else {
		newCurrentIndex = hm.currentIndex - itemsRemovedBeforeCurrent
	}

	if len(hm.stack) == 0 {
		hm.currentIndex = -1
	} else if newCurrentIndex >= len(hm.stack) {
		hm.currentIndex = len(hm.stack) - 1
	} else if newCurrentIndex < 0 && len(hm.stack) > 0 { // If it became negative, but stack is not empty
		hm.currentIndex = 0 // Point to the first valid item
	} else {
		hm.currentIndex = newCurrentIndex
	}
}
