// Package customwidgets contains custom Fyne widgets for the application.
package customwidgets // import "fyslide/internal/customwidgets"

import (
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Toggle is a widget that can be in two states, on or off.
type Toggle struct {
	widget.DisableableWidget
	// Toggled is the current state of the toggle.
	Toggled bool

	togglePLock sync.RWMutex

	// OnChanged is the callback function that is called when the state of the toggle changes.
	OnChanged func(bool) `json:"="`

	focused bool
	hovered bool

	binder binder

	minSize fyne.Size // cached for hover/top pos calcs
}

// NewToggle creates a new toggle widget.
func NewToggle(changed func(bool)) *Toggle {
	t := &Toggle{
		OnChanged: changed,
	}
	t.ExtendBaseWidget(t)
	return t
}

// NewToggleWithData creates a new toggle widget with a data binding.
func NewToggleWithData(data binding.Bool) *Toggle {
	toggle := NewToggle(nil)
	toggle.Bind(data)

	return toggle
}

// Bind connects the toggle to a data binding.
func (t *Toggle) Bind(data binding.Bool) {
	t.binder.SetCallback(t.updateFromData)
	t.binder.Bind(data)

	t.OnChanged = func(_ bool) {
		t.binder.CallWithData(t.writeData)
	}
}

// SetToggled sets the state of the toggle.
func (t *Toggle) SetToggled(toggled bool) {
	t.togglePLock.Lock()
	if toggled == t.Toggled {
		t.togglePLock.Unlock()
		return
	}

	t.Toggled = toggled
	onChanged := t.OnChanged
	t.togglePLock.Unlock()

	if onChanged != nil {
		onChanged(toggled)
	}

	t.Refresh()
}

// Hide hides the toggle widget.
func (t *Toggle) Hide() {
	if t.focused {
		t.FocusLost()
		if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
			c.Focus(nil)
		}
	}
	t.BaseWidget.Hide()
}

// MouseIn is called when the mouse enters the widget.
func (t *Toggle) MouseIn(me *desktop.MouseEvent) {
	t.MouseMoved(me)
}

// MouseOut is called when the mouse leaves the widget.
func (t *Toggle) MouseOut() {
	if t.hovered {
		t.hovered = false
		t.Refresh()
	}
}

// MouseMoved is called when the mouse moves over the widget.
func (t *Toggle) MouseMoved(me *desktop.MouseEvent) {
	if t.Disabled() {
		return
	}
	oldhovered := t.hovered

	t.hovered = t.minSize.IsZero() ||
		(me.Position.X <= t.minSize.Width && me.Position.Y <= t.minSize.Height)

	if oldhovered != t.hovered {
		t.Refresh()
	}
}

// Tapped is called when the user taps the widget.
func (t *Toggle) Tapped(pe *fyne.PointEvent) {
	if t.Disabled() {
		return
	}
	if !t.minSize.IsZero() &&
		(pe.Position.X > t.minSize.Width || pe.Position.Y > t.minSize.Height) {
		// tapped outside
		return
	}

	if !t.focused {
		if !fyne.CurrentDevice().IsMobile() {
			if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
				c.Focus(t)
			}
		}
	}
	t.SetToggled(!t.Toggled)
}

// MinSize returns the minimum size of the widget.
func (t *Toggle) MinSize() fyne.Size {
	t.ExtendBaseWidget(t)
	t.minSize = t.BaseWidget.MinSize()
	return t.minSize
}

// CreateRenderer creates a new renderer for the toggle widget.
func (t *Toggle) CreateRenderer() fyne.WidgetRenderer {
	th := t.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()

	t.ExtendBaseWidget(t)

	var bgColor fyne.ThemeColorName
	if t.Toggled {
		bgColor = theme.ColorNamePrimary
	} else {
		bgColor = theme.ColorNameInputBackground
	}
	bg := canvas.NewRectangle(th.Color(bgColor, v))
	bg.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	bg.CornerRadius = th.Size(theme.SizeNameInlineIcon) / 2
	bg.StrokeWidth = th.Size(theme.SizeNameInputBorder)

	indicator := canvas.NewCircle(th.Color(theme.ColorNameForegroundOnPrimary, v))
	indicator.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	indicator.StrokeWidth = th.Size(theme.SizeNameInputBorder)

	t.togglePLock.RLock()
	defer t.togglePLock.RUnlock()

	focusIndicator := canvas.NewCircle(th.Color(theme.ColorNameBackground, v))

	r := &toggleRenderer{
		bg:             bg,
		indicator:      indicator,
		focusIndicator: focusIndicator,
		toggle:         t,
	}

	r.applyTheme(th, v)
	r.updateToggle(th, v)
	r.updateFocusIndicator(th, v)
	return r
}

// FocusGained is called when the widget gains focus.
func (t *Toggle) FocusGained() {
	if t.Disabled() {
		return
	}

	t.focused = true

	t.Refresh()
}

// FocusLost is called when the widget loses focus.
func (t *Toggle) FocusLost() {
	t.focused = false
	t.Refresh()
}

// TypedRune is called when a rune is typed.
func (t *Toggle) TypedRune(r rune) {
	if t.Disabled() {
		return
	}
	if r == ' ' {
		t.SetToggled(!t.Toggled)
	}
}

// TypedKey is called when a key is typed.
func (t *Toggle) TypedKey(key *fyne.KeyEvent) {}

// Unbind disconnects the toggle from a data binding.
func (t *Toggle) Unbind() {
	t.OnChanged = nil
	t.binder.Unbind()
}

func (t *Toggle) updateFromData(data binding.DataItem) {
	if data == nil {
		return
	}
	boolSource, ok := data.(binding.Bool)
	if !ok {
		return
	}
	val, err := boolSource.Get()
	if err != nil {
		fyne.LogError("Error getting current data value", err)
		return
	}
	t.SetToggled(val)
}

func (t *Toggle) writeData(data binding.DataItem) {
	if data == nil {
		return
	}
	boolTarget, ok := data.(binding.Bool)
	if !ok {
		return
	}
	currentValue, err := boolTarget.Get()
	if err != nil {
		return
	}
	if currentValue != t.Toggled {
		err := boolTarget.Set(t.Toggled)
		if err != nil {
			fyne.LogError(fmt.Sprintf("Failed to set binding value to %t", t.Toggled), err)
		}
	}
}

type toggleRenderer struct {
	bg             *canvas.Rectangle
	indicator      *canvas.Circle
	focusIndicator *canvas.Circle
	toggle         *Toggle

	indicatorOffPos      fyne.Position
	indicatorOnPos       fyne.Position
	focusIndicatorOffPos fyne.Position
	focusIndicatorOnPos  fyne.Position
}

func (r *toggleRenderer) Destroy() {}
func (r *toggleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		r.bg,
		r.focusIndicator,
		r.indicator,
	}
}

func (r *toggleRenderer) MinSize() fyne.Size {
	th := r.toggle.Theme()

	pad4 := th.Size(theme.SizeNameInnerPadding)
	iconInline := th.Size(theme.SizeNameInlineIcon)
	borderSize := th.Size(theme.SizeNameInputBorder)
	minFunc := fyne.NewSize(
		(iconInline*2)+pad4*2+borderSize,
		iconInline+pad4+borderSize,
	)

	return minFunc
}

func (r *toggleRenderer) Layout(size fyne.Size) {
	th := r.toggle.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	borderSize := th.Size(theme.SizeNameInputBorder)
	iconInlineSize := th.Size(theme.SizeNameInlineIcon)

	r.indicatorOffPos = fyne.NewPos(
		innerPadding/2+borderSize,
		(size.Height-iconInlineSize-borderSize-innerPadding/2)/2,
	)
	indicatorSize := fyne.NewSquareSize(iconInlineSize + innerPadding/2)

	focusIndicatorSize := fyne.NewSquareSize(iconInlineSize + innerPadding)
	r.focusIndicatorOffPos = fyne.NewPos(
		innerPadding/4+borderSize,
		(size.Height-focusIndicatorSize.Height)/2,
	)
	r.indicatorOnPos = r.indicatorOffPos.AddXY(iconInlineSize+innerPadding, 0)
	r.focusIndicatorOnPos = r.focusIndicatorOffPos.AddXY(iconInlineSize+innerPadding, 0)

	r.toggle.togglePLock.RLock()
	toggled := r.toggle.Toggled
	r.toggle.togglePLock.RUnlock()
	if toggled {
		r.indicator.Move(r.indicatorOnPos)
		r.focusIndicator.Move(r.focusIndicatorOnPos)
	} else {
		r.indicator.Move(r.indicatorOffPos)
		r.focusIndicator.Move(r.focusIndicatorOffPos)
	}

	bgPos := fyne.NewPos(
		innerPadding/2,
		(size.Height-iconInlineSize)/2,
	)
	bgSize := fyne.NewSize(iconInlineSize*2+innerPadding, iconInlineSize)
	r.bg.Resize(bgSize)
	r.bg.Move(bgPos)
	r.indicator.Resize(indicatorSize)
}

func (r *toggleRenderer) applyTheme(th fyne.Theme, v fyne.ThemeVariant) {
	if r.toggle.Disabled() {
		r.indicator.FillColor = th.Color(theme.ColorNameDisabled, v)
	} else {
		r.indicator.FillColor = th.Color(theme.ColorNameForegroundOnPrimary, v)
	}

	r.indicator.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	r.indicator.StrokeWidth = th.Size(theme.SizeNameInputBorder)

	r.bg.CornerRadius = th.Size(theme.SizeNameInlineIcon) / 2
	r.bg.StrokeWidth = th.Size(theme.SizeNameInputBorder)
}

func (r *toggleRenderer) Refresh() {
	th := r.toggle.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()

	r.toggle.togglePLock.RLock()
	r.applyTheme(th, v)
	r.updateFocusIndicator(th, v)
	r.updateToggle(th, v)
	r.toggle.togglePLock.RUnlock()
}

func (r *toggleRenderer) updateFocusIndicator(th fyne.Theme, v fyne.ThemeVariant) {
	if r.toggle.Disabled() {
		r.focusIndicator.FillColor = color.Transparent
	} else if r.toggle.focused {
		r.focusIndicator.FillColor = th.Color(theme.ColorNameFocus, v)
	} else if r.toggle.hovered {
		r.focusIndicator.FillColor = th.Color(theme.ColorNameHover, v)
	} else {
		r.focusIndicator.FillColor = color.Transparent
	}

	if r.toggle.Toggled {
		r.focusIndicator.Move(r.focusIndicatorOnPos)
	} else {
		r.focusIndicator.Move(r.focusIndicatorOffPos)
	}
}

func (r *toggleRenderer) updateToggle(th fyne.Theme, v fyne.ThemeVariant) {
	if r.toggle.Toggled {
		r.indicator.Move(r.indicatorOnPos)
		r.bg.FillColor = th.Color(theme.ColorNamePrimary, v)
	} else {
		r.indicator.Move(r.indicatorOffPos)
		r.bg.FillColor = th.Color(theme.ColorNameInputBackground, v)
	}
}
