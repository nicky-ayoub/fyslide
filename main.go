// Main entry point for the application
package main

//go:generate fyne bundle --package ui -output internal/ui/bundle1.go assets/icon.png
//go:generate fyne bundle --package ui -output internal/ui/bundle2.go assets/back.png

import (
	"fyslide/internal/ui"
	"log"
)

func main() {
	// Set the logger prefix
	log.SetPrefix("Fyne Slide Show")

	ui.CreateApplication()
}
