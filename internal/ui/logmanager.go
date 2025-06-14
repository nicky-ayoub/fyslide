// internal/ui/logmanager.go
package ui

import (
	"fmt"

	"fyne.io/fyne/v2/widget"
)

const DefaultMaxLogMessages = 100 // Or get from App constants

type LogUIManager struct {
	logMessages     []string
	currentLogIndex int
	maxLogMessages  int

	// UI elements it controls
	statusLogLabel   *widget.Label
	statusLogUpBtn   *widget.Button
	statusLogDownBtn *widget.Button
}

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

	lm.statusLogLabel.SetText(fmt.Sprintf("[%d/%d] %s", lm.currentLogIndex+1, len(lm.logMessages), lm.logMessages[lm.currentLogIndex]))
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

func (lm *LogUIManager) ShowPreviousLogMessage() {
	if len(lm.logMessages) == 0 || lm.currentLogIndex <= 0 {
		return
	}
	lm.currentLogIndex--
	lm.UpdateLogDisplay()
}

func (lm *LogUIManager) ShowNextLogMessage() {
	if len(lm.logMessages) == 0 || lm.currentLogIndex >= len(lm.logMessages)-1 {
		return
	}
	lm.currentLogIndex++
	lm.UpdateLogDisplay()
}
