// Package slideshow manages the automatic cycling of content.
package slideshow

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultSlideshowInterval = 2 * time.Second
)

// LoggerFunc defines a function signature for logging messages.
type LoggerFunc func(message string)

// SlideshowManager handles the slideshow functionality.
type SlideshowManager struct {
	mu                 sync.Mutex
	isPaused           bool
	wasPlayingBeforeOp bool // Tracks if slideshow was playing before a temp pause
	interval           time.Duration
	logger             LoggerFunc
}

// NewSlideshowManager creates a new SlideshowManager.
// interval is the time between automatic transitions.
// logger is an optional logging function.
func NewSlideshowManager(interval time.Duration, logger LoggerFunc) *SlideshowManager {
	if interval <= 0 {
		interval = defaultSlideshowInterval // Default interval if invalid
	}
	sm := &SlideshowManager{
		isPaused:           true, // Start paused by default
		wasPlayingBeforeOp: false,
		interval:           interval,
		logger:             logger,
	}
	sm.logMsg("SlideshowManager initialized. Interval: %v, Initial state: Paused", sm.interval)
	return sm
}

// logMsg is a helper to use the configured logger.
func (sm *SlideshowManager) logMsg(format string, args ...interface{}) {
	if sm.logger != nil {
		sm.logger(fmt.Sprintf(format, args...))
	}
}

// TogglePlayPause toggles the play/pause state.
func (sm *SlideshowManager) TogglePlayPause() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isPaused = !sm.isPaused
	sm.wasPlayingBeforeOp = false // User toggle overrides any operation-specific state
	if sm.isPaused {
		sm.logMsg("Slideshow state toggled to: Paused")
	} else {
		sm.logMsg("Slideshow state toggled to: Playing")
	}
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
	sm.logMsg("Slideshow explicitly paused. Was playing before op: %t", sm.wasPlayingBeforeOp)
}

// ResumeAfterOperation resumes the slideshow only if it was playing before Pause(true) was called.
func (sm *SlideshowManager) ResumeAfterOperation() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.wasPlayingBeforeOp {
		sm.isPaused = false
		sm.logMsg("Slideshow resumed after operation. State: Playing")
	} else {
		sm.logMsg("Slideshow not resumed after operation (was not playing before or already resumed). Current paused state: %t", sm.isPaused)
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
