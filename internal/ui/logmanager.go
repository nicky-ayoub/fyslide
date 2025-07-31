// Package ui provides the user interface components for the FySlide application.
package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// DefaultMaxLogMessages is the maximum number of log messages to keep in memory.
const DefaultMaxLogMessages = 100 // Or get from App constants

// LogUIManager is responsible for managing the log messages and their display in the UI
type LogUIManager struct {
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
	if lm.statusLogLabel == nil {
		return
	}
	// ... (existing logic from App.addLogMessage)
	lm.logMessages = append(lm.logMessages, message)
	if len(lm.logMessages) > lm.maxLogMessages {
		lm.logMessages = lm.logMessages[len(lm.logMessages)-lm.maxLogMessages:]
	}
	lm.currentLogIndex = len(lm.logMessages) - 1
	lm.UpdateLogDisplay()
}

// UpdateLogDisplay updates the log display based on the current log messages and index.
func (lm *LogUIManager) UpdateLogDisplay() {
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

	fyne.Do(func() {
		lm.statusLogLabel.SetText(fmt.Sprintf("[%d/%d] %s", lm.currentLogIndex+1, len(lm.logMessages), lm.logMessages[lm.currentLogIndex]))
	})

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
}

// ShowPreviousLogMessage allows navigation through the log messages.
func (lm *LogUIManager) ShowPreviousLogMessage() {
	if len(lm.logMessages) == 0 || lm.currentLogIndex <= 0 {
		return
	}
	lm.currentLogIndex--
	lm.UpdateLogDisplay()
}

// ShowNextLogMessage allows navigation through the log messages.
func (lm *LogUIManager) ShowNextLogMessage() {
	if len(lm.logMessages) == 0 || lm.currentLogIndex >= len(lm.logMessages)-1 {
		return
	}
	lm.currentLogIndex++
	lm.UpdateLogDisplay()
}
