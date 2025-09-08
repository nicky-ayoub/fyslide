package ui

import (
	"bytes"
	"fmt"
	"fyslide/internal/service"
	"image"
	"image/jpeg"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"image/png"

	"github.com/nfnt/resize"
)

const (
	// ThumbnailWidth is the width of the thumbnails in the browser.
	ThumbnailWidth = 100
	// ThumbnailHeight is the height of the thumbnails in the browser.
	ThumbnailHeight = 100
)

// ThumbnailFormat defines the encoding format for thumbnails.
type ThumbnailFormat int

const (
	// PNG uses the PNG encoder (lossless, slower).
	PNG ThumbnailFormat = iota
	// JPEG uses the JPEG encoder (lossy, faster).
	JPEG
)

// ThumbnailManager handles generation and caching of image thumbnails.
type ThumbnailManager struct {
	cache        map[string]fyne.Resource
	cacheMutex   sync.RWMutex
	imageService *service.ImageService
	logger       func(string)
	format       ThumbnailFormat
}

// NewThumbnailManager creates a new thumbnail manager.
func NewThumbnailManager(imageService *service.ImageService, logger func(string)) *ThumbnailManager {
	return &ThumbnailManager{
		cache:        make(map[string]fyne.Resource),
		imageService: imageService,
		logger:       logger,
		format:       PNG, // Default to PNG
	}
}

// SetFormat sets the encoding format for new thumbnails and clears the cache.
func (tm *ThumbnailManager) SetFormat(format ThumbnailFormat) {
	tm.cacheMutex.Lock()
	defer tm.cacheMutex.Unlock()
	if tm.format != format {
		tm.format = format
		tm.cache = make(map[string]fyne.Resource) // Clear cache as format has changed
		tm.logger(fmt.Sprintf("Thumbnail format changed, cache cleared."))
	}
}

// imageToBytes converts an image.Image to a byte slice using the configured format.
func (tm *ThumbnailManager) imageToBytes(img image.Image) []byte {
	buf := new(bytes.Buffer)
	var err error
	if tm.format == JPEG {
		err = jpeg.Encode(buf, img, &jpeg.Options{Quality: 80})
	} else {
		err = png.Encode(buf, img)
	}
	if err != nil {
		tm.logger(fmt.Sprintf("Error encoding thumbnail: %v", err))
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
		var thumbImg image.Image

		// First, try to get the fast, embedded EXIF thumbnail.
		embeddedThumb, err := tm.imageService.GetEmbeddedThumbnail(path)
		if err != nil {
			// This is not a fatal error, just means no embedded thumb. Fall back to full decode.
			tm.logger("No embedded thumb for " + filepath.Base(path) + ", falling back to full decode.")

			// Fallback: Load and decode the entire image.
			_, imgDecoded, err := tm.imageService.GetImageInfo(path)
			if err != nil {
				tm.logger("Thumbnail error for " + filepath.Base(path) + ": " + err.Error())
				return
			}

			// Resize the full image.
			thumbImg = resize.Thumbnail(ThumbnailWidth, ThumbnailHeight, imgDecoded, resize.Lanczos3)
		} else {
			// Success! Resize the embedded thumbnail to ensure consistent dimensions.
			thumbImg = resize.Thumbnail(ThumbnailWidth, ThumbnailHeight, embeddedThumb, resize.Lanczos3)
		}

		thumbBytes := tm.imageToBytes(thumbImg)
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
