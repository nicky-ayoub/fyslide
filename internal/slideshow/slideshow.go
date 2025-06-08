// Package slideshow manages the automatic cycling of content.
package slideshow

import (
	"sync"
	"time"
)

// SlideshowManager handles the slideshow functionality.
type SlideshowManager struct {
	mu       sync.Mutex
	isPaused bool
	interval time.Duration
}

// NewSlideshowManager creates a new SlideshowManager.
// Interval is the time between automatic transitions.
func NewSlideshowManager(interval time.Duration) *SlideshowManager {
	if interval <= 0 {
		interval = 2 * time.Second // Default interval if invalid
	}
	return &SlideshowManager{
		isPaused: true, // Start paused by default
		interval: interval,
	}
}

// TogglePlayPause toggles the play/pause state.
func (sm *SlideshowManager) TogglePlayPause() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isPaused = !sm.isPaused
	// If it's now playing, and was previously paused due to an action,
	// this toggle effectively resumes it.
}

// Pause forces the slideshow to pause.
func (sm *SlideshowManager) Pause() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isPaused = true
}

// IsPaused returns true if the slideshow is currently paused.
func (sm *SlideshowManager) IsPaused() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.isPaused
}

// Interval returns the configured slideshow interval.
func (sm *SlideshowManager) Interval() time.Duration {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.interval
}
