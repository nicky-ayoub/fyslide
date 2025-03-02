package ui

import (
	"fmt"
	"fyslide/internal/scan"
	"image"
	"net/url"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

type Img struct {
	OriginalImage image.Image
	EditedImage   *image.RGBA
	Path          string
	Directory     string
}

// App represents the whole application with all its windows, widgets and functions
type App struct {
	app     fyne.App
	MainWin fyne.Window

	fileTree binding.URITree

	images scan.FileItems
	index  int
	img    Img
	image  *canvas.Image

	direction int

	mainModKey fyne.KeyModifier

	split       *container.Split
	countLabel  *widget.Label
	widthLabel  *widget.Label
	heightLabel *widget.Label
	imgSize     *widget.Label
	imgLastMod  *widget.Label
	statusBar   *fyne.Container
	first       *widget.Button
	leftArrow   *widget.Button
	pauseBtn    *widget.Button
	rightArrow  *widget.Button
	last        *widget.Button
	deleteBtn   *widget.Button
	tagBtn      *widget.Button
	statusLabel *widget.Label
}

func parseURL(urlStr string) *url.URL {
	link, err := url.Parse(urlStr)
	if err != nil {
		fyne.LogError("Could not parse URL", err)
	}
	return link

}

// func pathToURI(path string) (fyne.URI, error) {
// 	absPath, _ := filepath.Abs(path)
// 	fileURI := storage.NewFileURI(absPath)
// 	return fileURI, nil
// }

func (a *App) loadImages(root string) {
	scan.Run(root, &a.images)

	// for _, img := range a.images {
	// 	uri, err := pathToURI(img.Path)
	// 	if err != nil {
	// 		fmt.Println("Error:", err)
	// 		return
	// 	}
	// 	a.fileTree.Append(binding.DataTreeRootID, uri.String(), uri)
	// }
}
func (a *App) ImageCount() int {
	return len(a.images)
}

func (a *App) init() {
	a.img = Img{}
}

func CreateApplication() {
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

	a := app.NewWithID("com.github.nicky-ayoub/fyslide")
	w := a.NewWindow("FySlide")
	a.SetIcon(resourceIconPng)
	w.SetIcon(resourceIconPng)
	ui := &App{app: a, MainWin: w, direction: 1}
	ui.init()

	w.SetContent(ui.buildMainUI())

	go func() { ui.loadImages(dir) }()

	w.Resize(fyne.NewSize(1400, 700))
	w.CenterOnScreen()

	for ui.ImageCount() < 1 { // Stupidly wait for something to pop up
		time.Sleep(100 * time.Microsecond)
	}
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for range ticker.C {
			ui.nextImage()
		}
	}()
	ui.DisplayImage()
	w.ShowAndRun()
}

/*
From https://github.com/fyne-io/fyne/issues/2307

	Maybe this is helpful. It shows the main app window in the middle, covering 80% of the screen area.
	I use 'github.com/kbinani/screenshot' to get the screen size.

		...
		a := app.New()
		w := a.NewWindow("MyApp")
		...
		w.Resize(windowSize(0.8))
		w.CenterOnScreen()
		w.ShowAndRun()
		...

	func windowSize(part float32) fyne.Size {
		if screenshot.NumActiveDisplays() > 0 {
			// #0 is the main monitor
			bounds := screenshot.GetDisplayBounds(0)
			return fyne.NewSize(float32(bounds.Dx())*part, float32(bounds.Dy())*part)
		}
		return fyne.NewSize(800, 800)
	}
*/
