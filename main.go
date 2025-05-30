// Main entry point for the application
package main

//go:generate fyne bundle --package ui -output internal/ui/bundle1.go assets/icon.png
//go:generate fyne bundle --package ui -output internal/ui/bundle3.go assets/dice-24.png
//go:generate fyne bundle --package ui -output internal/ui/bundle4.go assets/dice-disabled-24.png

import (
	"fyslide/internal/ui"
	"log"
)

func main() {
	// Set the logger prefix
	log.SetPrefix("Fyne Slide Show")

	ui.CreateApplication()
}
