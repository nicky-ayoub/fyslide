// Package ui  Setup for the FySlide Application
package ui

import (
	"fmt"
	"fyslide/internal/scan"
	"image"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/go-gl/glfw/v3.3/glfw"
	//"fyne.io/fyne/v2/data/binding"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Img struct
type Img struct {
	OriginalImage image.Image
	EditedImage   *image.RGBA
	Path          string
	Directory     string
}

// UI struct
type UI struct {
	MainWin    fyne.Window
	mainModKey fyne.KeyModifier

	split       *container.Split
	clockLabel  *widget.Label
	countLabel  *widget.Label
	widthLabel  *widget.Label
	heightLabel *widget.Label
	imgSize     *widget.Label
	imgLastMod  *widget.Label
	statusBar   *fyne.Container
	quit        *widget.Button
	first       *widget.Button
	leftArrow   *widget.Button
	pauseBtn    *widget.Button
	rightArrow  *widget.Button
	last        *widget.Button
	deleteBtn   *widget.Button
	tagBtn      *widget.Button
	statusLabel *widget.Label
	toolbar     *widget.Toolbar

	//explorer *widget.Accordion
}

// App represents the whole application with all its windows, widgets and functions
type App struct {
	app fyne.App
	UI  UI

	//fileTree binding.URITree

	images scan.FileItems
	index  int
	img    Img
	image  *canvas.Image

	paused    bool
	direction int
}

func parseURL(urlStr string) *url.URL {
	link, err := url.Parse(urlStr)
	if err != nil {
		fyne.LogError("Could not parse URL", err)
	}
	return link

}

// DisplayImage displays the image on the canvas at the current index
func (a *App) DisplayImage() error {
	// decode and update the image + get image path
	//fmt.Printf("Displaying %s\n", a.images[a.index].Path)
	var err error
	file, err := os.Open(a.images[a.index].Path)
	if err != nil {
		return fmt.Errorf("unable to open image '%s' %v", a.images[a.index].Path, err)
	}
	image, name, err := image.Decode(file)
	if err != nil {
		fmt.Printf("Unable image.Decode(%q) of format %q\n", file.Name(), name)
		return fmt.Errorf("unable to decode image %v", err)
	}
	a.img.OriginalImage = image
	a.img.Path = file.Name()
	a.image.Image = a.img.OriginalImage
	a.image.Refresh()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("unable to get file info %v", err)
	}

	a.UI.imgSize.SetText(fmt.Sprintf("Size: %d bytes", fileInfo.Size()))
	a.UI.imgLastMod.SetText(fmt.Sprintf("Last modified: \n%s", fileInfo.ModTime().Format("02-01-2006")))
	w := fmt.Sprintf("Width:   %dpx", a.img.OriginalImage.Bounds().Max.X)
	h := fmt.Sprintf("Height: %dpx", a.img.OriginalImage.Bounds().Max.Y)
	c := fmt.Sprintf("Count: %d", a.imageCount())
	a.UI.widthLabel.SetText(w)
	a.UI.heightLabel.SetText(h)
	a.UI.countLabel.SetText(c)

	a.UI.MainWin.SetTitle(fmt.Sprintf("FySlide - %v", (strings.Split(a.img.Path, "/")[len(strings.Split(a.img.Path, "/"))-1])))
	a.UI.statusLabel.SetText(fmt.Sprintf("Image %s, %d of %d", a.img.Path, a.index+1, len(a.images)))

	a.UI.leftArrow.Enable()
	a.UI.rightArrow.Enable()
	a.UI.first.Enable()
	a.UI.last.Enable()
	return nil
}

func (a *App) firstImage() {
	a.index = 0
	a.DisplayImage()
	a.direction = 1

}

func (a *App) lastImage() {
	a.index = len(a.images) - 1
	a.DisplayImage()
	a.direction = -1

}

func (a *App) nextImage() {
	a.index += a.direction
	if a.index < 0 {
		a.direction = 1
		a.index = 0
	} else if a.index >= len(a.images) {
		a.direction = -1
		a.index = len(a.images) - 1
	}
	a.DisplayImage()
}

func (a *App) togglePlay() {
	if a.paused {
		a.paused = false
		a.UI.pauseBtn.SetIcon(theme.MediaPauseIcon())
		a.UI.toolbar.Items[2].(*widget.ToolbarAction).SetIcon(theme.MediaPauseIcon())
	} else {
		a.paused = true
		a.UI.pauseBtn.SetIcon(theme.MediaPlayIcon())
		a.UI.toolbar.Items[2].(*widget.ToolbarAction).SetIcon(theme.MediaPlayIcon())
	}
}

func (a *App) updateTime() {
	formatted := time.Now().Format("Time: 03:04:05")
	a.UI.clockLabel.SetText(formatted)
}
func (a *App) tagFile() {
	dialog.ShowCustom("TAGGER", "Ok", container.NewVBox(
		widget.NewLabel("Add image tag."),
		widget.NewHyperlink("Help and more information on Github", parseURL("https://github.com/nicky-ayoub/fyslide")),
		widget.NewLabel("v1.2 | License: MIT"),
	), a.UI.MainWin)
}

// Delete file

func (a *App) deleteFileCheck() {
	dialog.ShowConfirm("Delete file!", "Are you sure?\n This action can't be undone.", func(b bool) {
		if b {
			a.deleteFile()
		}
	}, a.UI.MainWin)
}

func (a *App) deleteFile() {
	if err := os.Remove(a.img.Path); err != nil {
		dialog.NewError(err, a.UI.MainWin)
		return
	}
	if a.index == len(a.images)-1 {
		a.direction = -1
		a.nextImage()
	} else if len(a.images) == 1 {
		a.direction = 1
		a.image.Image = nil
		a.img.EditedImage = nil
		a.img.OriginalImage = nil
		a.UI.rightArrow.Disable()
		a.UI.leftArrow.Disable()
		a.UI.deleteBtn.Disable()
		a.image.Refresh()
	} else {
		a.nextImage()
	}
}

// func pathToURI(path string) (fyne.URI, error) {
// 	absPath, _ := filepath.Abs(path)
// 	fileURI := storage.NewFileURI(absPath)
// 	return fileURI, nil
// }

func (a *App) loadImages(root string) {
	scan.Run(root, &a.images)
}

func (a *App) imageCount() int {
	return len(a.images)
}

func (a *App) init() {
	a.img = Img{}
}

// CreateApplication is the GUI entrypoint
func CreateApplication() {

	ScreenWidth, ScreenHeight := getDisplayResolution()
	log.Printf("Screen resolution: %f x %f\n", ScreenWidth, ScreenHeight)

	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("error while opening the directory : %v\n", err)
		return
	}
	if len(os.Args) > 1 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Printf("error while opening the directory '%s': %v\n", file.Name(), err)
			return
		}
		s, _ := file.Stat()
		if s.IsDir() {
			dir = s.Name()
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return
	}

	a := app.NewWithID("com.github.nicky-ayoub/fyslide")
	a.SetIcon(resourceIconPng)
	ui := &App{app: a, direction: 1}
	ui.UI.MainWin = a.NewWindow("FySlide")
	ui.UI.MainWin.SetIcon(resourceIconPng)
	ui.init()

	ui.UI.MainWin.SetContent(ui.buildMainUI())

	go ui.loadImages(dir)

	ui.UI.MainWin.Resize(fyne.NewSize(ScreenWidth, ScreenHeight))
	ui.UI.MainWin.CenterOnScreen()

	for ui.imageCount() < 1 { // Stupidly wait for something to pop up
		time.Sleep(100 * time.Microsecond)
	}

	ticker := time.NewTicker(2 * time.Second)
	go ui.pauser(ticker)
	go ui.updateTimer()

	ui.DisplayImage()
	ui.UI.MainWin.ShowAndRun()
}

func (a *App) updateTimer() {
	for range time.Tick(time.Second) {
		a.updateTime()
	}
}

func (a *App) pauser(ticker *time.Ticker) {
	for range ticker.C {
		if !a.paused {
			a.nextImage()
		}
	}
}

func getDisplayResolution() (float32, float32) {
	glfw.Init()
	defer glfw.Terminate()
	monitor := glfw.GetPrimaryMonitor()
	return float32(monitor.GetVideoMode().Width), float32(monitor.GetVideoMode().Height)
}
