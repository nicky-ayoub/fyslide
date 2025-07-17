package ui

import (
	"bytes"
	"image"
	"image/png"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/nfnt/resize"
)

const (
	// ThumbnailWidth is the width of the thumbnails in the browser.
	ThumbnailWidth = 100
	// ThumbnailHeight is the height of the thumbnails in the browser.
	ThumbnailHeight = 100
)

// ThumbnailManager handles generation and caching of image thumbnails.
type ThumbnailManager struct {
	cache      map[string]fyne.Resource
	cacheMutex sync.RWMutex
	app        *App // To access services
}

// NewThumbnailManager creates a new thumbnail manager.
func NewThumbnailManager(app *App) *ThumbnailManager {
	return &ThumbnailManager{
		cache: make(map[string]fyne.Resource),
		app:   app,
	}
}

// imageToBytes is a helper to convert image.Image to []byte for Fyne resources.
func imageToBytes(img image.Image) []byte {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		return nil
	}
	return buf.Bytes()
}

// GetThumbnail generates or retrieves a cached thumbnail for a given image path.
// It returns a placeholder resource immediately and calls onComplete with the
// actual thumbnail resource once it's generated.
func (tm *ThumbnailManager) GetThumbnail(path string, onComplete func(fyne.Resource)) fyne.Resource {
	tm.cacheMutex.RLock()
	if res, ok := tm.cache[path]; ok {
		tm.cacheMutex.RUnlock()
		return res
	}
	tm.cacheMutex.RUnlock()

	go func() {
		_, imgDecoded, err := tm.app.ImageService.GetImageInfo(path)
		if err != nil {
			tm.app.addLogMessage("Thumbnail error for " + filepath.Base(path) + ": " + err.Error())
			return
		}

		thumbImg := resize.Thumbnail(ThumbnailWidth, ThumbnailHeight, imgDecoded, resize.Lanczos3)
		thumbBytes := imageToBytes(thumbImg)
		if thumbBytes == nil {
			return
		}
		imgResource := fyne.NewStaticResource(path, thumbBytes)

		tm.cacheMutex.Lock()
		tm.cache[path] = imgResource
		tm.cacheMutex.Unlock()

		fyne.Do(func() {
			onComplete(imgResource)
		})
	}()

	return theme.FileImageIcon()
}
