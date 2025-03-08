package ui

import (
	"fmt"
	"fyslide/internal/scan"
	"image"
	"net/url"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
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

	//fileTree binding.URITree

	images scan.FileItems
	index  int
	img    Img
	image  *canvas.Image

	paused    bool
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
	toolbar     *widget.Toolbar
}

func parseURL(urlStr string) *url.URL {
	link, err := url.Parse(urlStr)
	if err != nil {
		fyne.LogError("Could not parse URL", err)
	}
	return link

}

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

	a.imgSize.SetText(fmt.Sprintf("Size: %d bytes", fileInfo.Size()))
	a.imgLastMod.SetText(fmt.Sprintf("Last modified: \n%s", fileInfo.ModTime().Format("02-01-2006")))
	w := fmt.Sprintf("Width:   %dpx", a.img.OriginalImage.Bounds().Max.X)
	h := fmt.Sprintf("Height: %dpx", a.img.OriginalImage.Bounds().Max.Y)
	c := fmt.Sprintf("Count: %d", a.ImageCount())
	a.widthLabel.SetText(w)
	a.heightLabel.SetText(h)
	a.countLabel.SetText(c)

	a.MainWin.SetTitle(fmt.Sprintf("FySlide - %v", (strings.Split(a.img.Path, "/")[len(strings.Split(a.img.Path, "/"))-1])))
	a.statusLabel.SetText(fmt.Sprintf("Image %s, %d of %d", a.img.Path, a.index+1, len(a.images)))

	a.leftArrow.Enable()
	a.rightArrow.Enable()
	a.first.Enable()
	a.last.Enable()
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

func (a *App) tagFile() {
	dialog.ShowCustom("TAGGER", "Ok", container.NewVBox(
		widget.NewLabel("Add image tag."),
		widget.NewHyperlink("Help and more information on Github", parseURL("https://github.com/nicky-ayoub/fyslide")),
		widget.NewLabel("v1.2 | License: MIT"),
	), a.MainWin)
}
func (a *App) deleteFileCheck() {
	dialog.ShowConfirm("Delete file!", "Are you sure?\n This action can't be undone.", func(b bool) {
		if b {
			a.deleteFile()
		}
	}, a.MainWin)
}

func (a *App) togglePlay() {
	if a.paused {
		a.paused = false
		a.pauseBtn.SetIcon(theme.MediaPauseIcon())
		a.toolbar.Items[2].(*widget.ToolbarAction).SetIcon(theme.MediaPauseIcon())
	} else {
		a.paused = true
		a.pauseBtn.SetIcon(theme.MediaPlayIcon())
		a.toolbar.Items[2].(*widget.ToolbarAction).SetIcon(theme.MediaPlayIcon())
	}
}

func (a *App) deleteFile() {
	if err := os.Remove(a.img.Path); err != nil {
		dialog.NewError(err, a.MainWin)
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
		a.rightArrow.Disable()
		a.leftArrow.Disable()
		a.deleteBtn.Disable()
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
			if !ui.paused {
				ui.nextImage()
			}
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
