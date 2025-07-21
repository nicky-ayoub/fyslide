package service

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ImageInfo holds metadata about an image file.
type ImageInfo struct {
	Width    int
	Height   int
	Size     int64
	ModTime  time.Time
	EXIFData map[string]string
}

// ImageService provides image loading and metadata extraction.
type ImageService struct {
}

// NewImageService creates a new ImageService.
func NewImageService() *ImageService {
	return &ImageService{}
}

// GetEXIF extracts a few common EXIF fields from an image file.
func (is *ImageService) GetEXIF(r io.Reader) (map[string]string, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return nil, nil // Not all images have EXIF; not an error for non-JPEGs
	}
	result := make(map[string]string)
	for _, field := range []string{
		"DateTime", "Model", "Make", "ExposureTime", "FNumber", "ISOSpeedRatings", "FocalLength",
	} {
		tag, err := x.Get(exif.FieldName(field))
		if err == nil && tag != nil {
			result[field] = tag.String()
		}
	}
	return result, nil
}

// GetImageInfo returns width, height, file size, mod time, and EXIF data.
func (is *ImageService) GetImageInfo(path string) (*ImageInfo, image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open image for info: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat image file: %w", err)
	}

	exifData, _ := is.GetEXIF(f) // Pass the file reader, EXIF is optional

	// Seek back to the beginning of the file for image decoding
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("failed to seek in image file: %w", err)
	}

	img, _, err := image.Decode(f) // Decode the image using the same file handle
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode image for info: %w", err)
	}

	bounds := img.Bounds()
	return &ImageInfo{
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Size:     fi.Size(),
		ModTime:  fi.ModTime(),
		EXIFData: exifData,
	}, img, nil
}
