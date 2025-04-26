package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// smallTabsTheme wraps an existing theme and reduces padding.
type smallTabsTheme struct {
	fyne.Theme
}

// Ensure smallTabsTheme implements fyne.Theme
var _ fyne.Theme = (*smallTabsTheme)(nil)

// Size overrides the default theme size for padding.
func (t *smallTabsTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNamePadding {
		// Default is usually 4.0, let's try reducing it.
		// Adjust this value (e.g., 1.0, 2.0) to control the spacing.
		return 1.0
	}

	// For all other sizes, use the embedded theme's default.
	return t.Theme.Size(name)
}

// --- Delegate other theme methods to the embedded theme ---
// You might need to add more delegations if you encounter issues,
// but often just embedding and overriding Size is enough for this specific need.

func (t *smallTabsTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, variant)
}

func (t *smallTabsTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.Theme.Font(style)
}

func (t *smallTabsTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.Theme.Icon(name)
}

// NewSmallTabsTheme creates a new theme wrapper with reduced padding.
// It bases itself on the currently set theme.
func NewSmallTabsTheme(baseTheme fyne.Theme) fyne.Theme {
	return &smallTabsTheme{Theme: baseTheme}
}
