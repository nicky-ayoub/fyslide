package main

//go:generate fyne bundle --package ui -output bundle.go assets/icon.png

import (
	"fyslide/internal/ui"
	"log"
)

func main() {
	// Set the logger prefix
	log.SetPrefix("Fyne Slide Show")

	ui.CreateApplication()
}
