// Package ui provides the user interface components for the FySlide application.
package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// DefaultMaxLogMessages is the maximum number of log messages to keep in memory.
const DefaultMaxLogMessages = 20 // Or get from App constants

// LogUIManager is responsible for managing the log messages and their display in the UI
type LogUIManager struct {
	mu              sync.RWMutex
	logMessages     []string
	currentLogIndex int
	maxLogMessages  int

	// UI elements it controls
	statusLogLabel   *widget.Label
	statusLogUpBtn   *widget.Button
	statusLogDownBtn *widget.Button
}

// NewLogUIManager creates a new LogUIManager with the specified UI components and maximum log messages.
func NewLogUIManager(logLabel *widget.Label, upBtn, downBtn *widget.Button, maxMessages int) *LogUIManager {
	if maxMessages <= 0 {
		maxMessages = DefaultMaxLogMessages
	}
	return &LogUIManager{
		logMessages:      make([]string, 0, maxMessages),
		currentLogIndex:  -1,
		maxLogMessages:   maxMessages,
		statusLogLabel:   logLabel,
		statusLogUpBtn:   upBtn,
		statusLogDownBtn: downBtn,
	}
}

// AddLogMessage adds a new log message to the LogUIManager and updates the display.
func (lm *LogUIManager) AddLogMessage(message string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.statusLogLabel == nil {
		return
	}
	// ... (existing logic from App.addLogMessage)
	lm.logMessages = append(lm.logMessages, message)
	if len(lm.logMessages) > lm.maxLogMessages {
		lm.logMessages = lm.logMessages[len(lm.logMessages)-lm.maxLogMessages:]
	}
	lm.currentLogIndex = len(lm.logMessages) - 1 // Point to the newest message
	lm.updateLogDisplay()                        // Call the internal, non-locking version
}

// UpdateLogDisplay is the public, thread-safe method to refresh the log display.
func (lm *LogUIManager) UpdateLogDisplay() {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	lm.updateLogDisplay()
}

// updateLogDisplay is the internal, non-thread-safe version of UpdateLogDisplay.
// It must be called from a method that already holds a lock.
func (lm *LogUIManager) updateLogDisplay() {
	// ... (existing logic from App.updateLogDisplay, using lm.fields)
	if lm.statusLogLabel == nil || lm.statusLogUpBtn == nil || lm.statusLogDownBtn == nil {
		return
	}
	// ... rest of the logic
	if len(lm.logMessages) == 0 {
		lm.statusLogLabel.SetText("")
		lm.statusLogUpBtn.Disable()
		lm.statusLogDownBtn.Disable()
		return
	}

	if lm.currentLogIndex < 0 {
		lm.currentLogIndex = 0
	} else if lm.currentLogIndex >= len(lm.logMessages) {
		lm.currentLogIndex = len(lm.logMessages) - 1
	}

	// Capture the state that will be used in the UI update.
	msg := lm.logMessages[lm.currentLogIndex]
	idx := lm.currentLogIndex
	count := len(lm.logMessages)
	fyne.Do(func() {
		lm.statusLogLabel.SetText(fmt.Sprintf("[%d/%d] %s", idx+1, count, msg))

		if lm.currentLogIndex <= 0 {
			lm.statusLogUpBtn.Disable()
		} else {
			lm.statusLogUpBtn.Enable()
		}
		if lm.currentLogIndex >= len(lm.logMessages)-1 {
			lm.statusLogDownBtn.Disable()
		} else {
			lm.statusLogDownBtn.Enable()
		}
	})
}

// ShowPreviousLogMessage allows navigation through the log messages.
func (lm *LogUIManager) ShowPreviousLogMessage() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if len(lm.logMessages) == 0 || lm.currentLogIndex <= 0 {
		return
	}
	lm.currentLogIndex--
	lm.UpdateLogDisplay()
}

// ShowNextLogMessage allows navigation through the log messages.
func (lm *LogUIManager) ShowNextLogMessage() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if len(lm.logMessages) == 0 || lm.currentLogIndex >= len(lm.logMessages)-1 {
		return
	}
	lm.currentLogIndex++
	lm.UpdateLogDisplay()
}
