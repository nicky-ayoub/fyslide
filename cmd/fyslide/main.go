// Package main fySlide is a simple image viewer with basic tagging and slideshow capabilities.
package main

//go:generate fyne bundle --package ui -output ../../internal/ui/bundle1.go ../../assets/icon.png
//go:generate fyne bundle --package ui -output ../../internal/ui/bundle3.go ../../assets/dice-24.png
//go:generate fyne bundle --package ui --append -output ../../internal/ui/bundle3.go ../../assets/dice-dark-24.png
//go:generate fyne bundle --package ui -output ../../internal/ui/bundle4.go ../../assets/dice-disabled-24.png
//go:generate fyne bundle --package ui --append -output ../../internal/ui/bundle4.go ../../assets/dice-disabled-dark-24.png
//go:generate fyne bundle --package ui -output ../../internal/ui/bundle5.go ../../assets/fyslidesplash004.png

import (
	"fyslide/internal/ui"
)

func main() {

	ui.CreateApplication()
}
