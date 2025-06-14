// Package slideshow manages the automatic cycling of content.
package slideshow

import (
	"sync"
	"time"
)

const (
	defaultSlideshowInterval = 2 * time.Second
)

// SlideshowManager handles the slideshow functionality.
type SlideshowManager struct {
	mu                 sync.Mutex
	isPaused           bool
	wasPlayingBeforeOp bool // Tracks if slideshow was playing before a temp pause
	interval           time.Duration
}

// NewSlideshowManager creates a new SlideshowManager.
// Interval is the time between automatic transitions.
func NewSlideshowManager(interval time.Duration) *SlideshowManager {
	if interval <= 0 {
		interval = defaultSlideshowInterval // Default interval if invalid
	}
	return &SlideshowManager{
		isPaused:           false, // Start unpaused (playing) by default
		wasPlayingBeforeOp: false,
		interval:           interval,
	}
}

// TogglePlayPause toggles the play/pause state.
func (sm *SlideshowManager) TogglePlayPause() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isPaused = !sm.isPaused
	sm.wasPlayingBeforeOp = false // User toggle overrides any operation-specific state
}

// Pause forces the slideshow to pause.
// If forOperation is true, it remembers if the slideshow was playing.
func (sm *SlideshowManager) Pause(forOperation bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if forOperation {
		// Record if it was playing *before* this operation's pause.
		// If already paused, wasPlayingBeforeOp becomes false. If playing, it becomes true.
		sm.wasPlayingBeforeOp = !sm.isPaused
	}
	sm.isPaused = true
}

// ResumeAfterOperation resumes the slideshow only if it was playing before Pause(true) was called.
func (sm *SlideshowManager) ResumeAfterOperation() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.wasPlayingBeforeOp {
		sm.isPaused = false
	}
	sm.wasPlayingBeforeOp = false // Reset the flag
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
