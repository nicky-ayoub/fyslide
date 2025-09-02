package custom_widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// shortcutDetail holds the data for a single row in the shortcuts table.
type shortcutDetail struct {
	Description string
	Shortcut    string
}

// ShortcutTable is a custom widget that displays a table of keyboard shortcuts.
type ShortcutTable struct {
	widget.BaseWidget
	shortcutData []shortcutDetail
}

// NewShortcutTable creates a new ShortcutTable widget.
func NewShortcutTable() *ShortcutTable {
	st := &ShortcutTable{
		shortcutData: []shortcutDetail{
			{Description: "Quit Application", Shortcut: "Ctrl+Q"},
			{Description: "Next Image", Shortcut: "Arrow Right"},
			{Description: "Previous Image", Shortcut: "Arrow Left"},
			{Description: "Skip Images Back", Shortcut: "Page Up"},
			{Description: "Skip Images Forward", Shortcut: "Page Down"},
			{Description: "First Image", Shortcut: "Home"},
			{Description: "Last Image", Shortcut: "End"},
			{Description: "Toggle Play/Pause Slideshow", Shortcut: "P or Space"},
			{Description: "Delete Current Image", Shortcut: "Delete"},
			{Description: "Close Dialog/Overlay", Shortcut: "Esc"},
			{Description: "Zoom In Image", Shortcut: "+"},
			{Description: "Zoom Out Image", Shortcut: "-"},
			{Description: "Reset Image Zoom/Pan", Shortcut: "0"},
			{Description: "Pan Image Up", Shortcut: "Arrow Up"},
			{Description: "Pan Image Down", Shortcut: "Arrow Down"},
		},
	}
	st.ExtendBaseWidget(st)
	return st
}

// CreateRenderer implements fyne.Widget.
func (st *ShortcutTable) CreateRenderer() fyne.WidgetRenderer {
	table := widget.NewTable(
		func() (int, int) { return len(st.shortcutData) + 1, 2 }, // +1 for header row
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			isHeader := id.Row == 0 // First row is header
			dataIndex := id.Row - 1

			if id.Col == 0 { // Description column
				if isHeader {
					label.SetText("Description")
				} else {
					label.SetText(st.shortcutData[dataIndex].Description)
				}
			} else { // Shortcut column
				if isHeader {
					label.SetText("Shortcut")
				} else {
					label.SetText(st.shortcutData[dataIndex].Shortcut)
				}
			}
			label.TextStyle.Bold = isHeader
		},
	)
	table.SetColumnWidth(0, 250)
	table.SetColumnWidth(1, 250)

	return widget.NewSimpleRenderer(table)
}
