// Package ui contains user interface components and logic.
package ui

import (
	"math/rand"
	"sync"
)

const (
	// NavigationQueueSize is the number of upcoming images to keep in the queue.
	// It should be larger than MaxVisibleThumbnails.
	NavigationQueueSize = 20
)

// NavigationQueue manages the upcoming sequence of images, supporting both random and sequential modes.
type NavigationQueue struct {
	app          *App
	queue        []int // Stores the indices of upcoming images. queue[0] is the current image.
	modeIsRandom bool
	mu           sync.Mutex
}

// NewNavigationQueue creates a new navigation queue manager.
func NewNavigationQueue(app *App) *NavigationQueue {
	return &NavigationQueue{
		app:   app,
		queue: []int{},
	}
}

// SetMode changes the navigation mode and refills the queue.
func (nq *NavigationQueue) SetMode(isRandom bool) {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	if nq.modeIsRandom == isRandom {
		return // No change
	}
	nq.modeIsRandom = isRandom
	if len(nq.queue) > 0 {
		nq.resetAndFill(nq.queue[0])
	}
}

// PopAndAdvance moves to the next image in the queue and returns its index.
// It triggers a non-destructive append if the queue becomes too short.
func (nq *NavigationQueue) PopAndAdvance() int {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	if len(nq.queue) <= 1 {
		currentIdx := 0
		if len(nq.queue) == 1 {
			currentIdx = nq.queue[0]
		}
		nq.resetAndFill(currentIdx)
		if len(nq.queue) <= 1 {
			return -1
		}
	}

	nq.queue = nq.queue[1:]

	if len(nq.queue) < NavigationQueueSize/2 {
		go nq.appendToQueue()
	}

	return nq.queue[0]
}

// GetUpcoming returns a slice of the next `count` image indices from the queue for display.
func (nq *NavigationQueue) GetUpcoming(count int) []int {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	if len(nq.queue) == 0 {
		return []int{}
	}

	if count > len(nq.queue) {
		count = len(nq.queue)
	}
	result := make([]int, count)
	copy(result, nq.queue[:count])
	return result
}

// PositionOf returns the position (0-based) of a targetIndex within the queue.
// It returns -1 if the targetIndex is not found in the queue.
func (nq *NavigationQueue) PositionOf(targetIndex int) int {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	for i, qi := range nq.queue {
		if qi == targetIndex {
			return i
		}
	}

	return -1
}

// RotateTo makes the item at the given index in the queue the new "current" item (index 0).
// It rotates the queue so that the target image is at the front. If the target image is not
// found in the current queue (e.g., a "history" thumbnail in random mode), it resets the queue.
func (nq *NavigationQueue) RotateTo(targetIndex int) int {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	if len(nq.queue) == 0 {
		return -1 // Empty queue, nothing to do.
	}

	// Find the index in the queue that matches the targetIndex from the broader image list.
	queueIndex := -1
	for i, qi := range nq.queue {
		if qi == targetIndex {
			queueIndex = i
			break
		}
	}

	if queueIndex == -1 {
		// The targetIndex is not in the current queue; reset and fill from there.
		nq.resetAndFill(targetIndex)
		return nq.queue[0]
	}

	if queueIndex == 0 {
		return nq.queue[0] // Already at the front, nothing to do.
	}

	// Rotate the queue.
	newQueue := make([]int, len(nq.queue))
	copy(newQueue, nq.queue[queueIndex:])                            // Elements from targetIndex to the end
	copy(newQueue[len(nq.queue)-queueIndex:], nq.queue[:queueIndex]) // Elements from start to targetIndex
	nq.queue = newQueue

	// No need to refill here; the queue retains its size, just rotated.
	// Refilling is still done only by PopAndAdvance.

	// The queue has been rotated, but queue[0] now correctly represents the new "current" index.
	if len(nq.queue) > 0 {
		return nq.queue[0] // Return the new current image index (now at the front).
	}

	return -1
}

// ResetAndFill clears the queue and refills it starting from a specific index.
func (nq *NavigationQueue) ResetAndFill(startingIndex int) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	nq.resetAndFill(startingIndex)
}

// appendToQueue adds new images to the end of the queue without resetting it.
func (nq *NavigationQueue) appendToQueue() {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	count := nq.app.getCurrentImageCount()
	if count <= 1 || len(nq.queue) >= count {
		return
	}

	usedIndices := make(map[int]bool)
	for _, idx := range nq.queue {
		usedIndices[idx] = true
	}

	if nq.modeIsRandom {
		for len(nq.queue) < NavigationQueueSize && len(nq.queue) < count {
			nextIdx := rand.Intn(count)
			if !usedIndices[nextIdx] {
				nq.queue = append(nq.queue, nextIdx)
				usedIndices[nextIdx] = true
			}
		}
	} else {
		lastIdx := nq.queue[len(nq.queue)-1]
		for len(nq.queue) < NavigationQueueSize && len(nq.queue) < count {
			lastIdx = (lastIdx + 1) % count
			if !usedIndices[lastIdx] {
				nq.queue = append(nq.queue, lastIdx)
				usedIndices[lastIdx] = true
			} else {
				break
			}
		}
	}
}

// resetAndFill is the internal logic to populate the queue from scratch.
func (nq *NavigationQueue) resetAndFill(startingIndex int) {
	nq.queue = []int{}
	count := nq.app.getCurrentImageCount()
	if count == 0 {
		return
	}

	if startingIndex < 0 || startingIndex >= count {
		startingIndex = 0
	}

	nq.queue = append(nq.queue, startingIndex)

	if nq.modeIsRandom {
		usedIndices := make(map[int]bool)
		usedIndices[startingIndex] = true
		for len(nq.queue) < NavigationQueueSize && len(nq.queue) < count {
			nextIdx := rand.Intn(count)
			if !usedIndices[nextIdx] {
				nq.queue = append(nq.queue, nextIdx)
				usedIndices[nextIdx] = true
			}
		}
	} else {
		current := startingIndex
		for len(nq.queue) < NavigationQueueSize && len(nq.queue) < count {
			current = (current + 1) % count
			nq.queue = append(nq.queue, current)
		}
	}
}
