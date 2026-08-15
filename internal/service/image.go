// Package service provides image loading and metadata extraction services.
package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // Register GIF decoder
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"os"
	"path/filepath"
	"strconv"
	"time"

	"log"

	"fyslide/internal/tagging"

	"github.com/rwcarlsen/goexif/exif"
)

// ImageInfo holds metadata about an image.
type ImageInfo struct {
	Width    int
	Height   int
	Size     int64
	ModTime  time.Time
	EXIFData map[string]string
}

// ImageService provides methods for loading and decoding images.

// ImageService provides an optional per-instance test hook for GetImageInfo.
// The hook is unexported and intended for tests only; leave nil in production.
type ImageService struct {
	testHook func(string)
}

// NewImageServiceWithHook creates an ImageService with a test hook.
func NewImageServiceWithHook(h func(string)) *ImageService {
	return &ImageService{testHook: h}
}

// NewImageService creates a new ImageService.
func NewImageService() *ImageService {
	return &ImageService{}
}

// GetImageInfo reads an image file, decodes it, and extracts metadata.
func (is *ImageService) GetImageInfo(path string) (info *ImageInfo, img image.Image, err error) {
	// Allow tests to inject delays or other behavior via per-instance hook
	if is.testHook != nil {
		is.testHook(path)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC recovered in GetImageInfo for path %s: %v", path, r)
			info = nil
			img = nil
			err = fmt.Errorf("recovered from panic processing image file %s: %v", filepath.Base(path), r)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	img, _, err = image.Decode(file)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding image: %w", err)
	}

	// Reset file pointer to read EXIF data
	if _, err := file.Seek(0, 0); err != nil {
		return nil, nil, fmt.Errorf("seeking file for exif: %w", err)
	}

	exifData, _ := exif.Decode(file) // Ignore error, EXIF might not be present

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("getting file stats: %w", err)
	}

	info = &ImageInfo{
		Width:    img.Bounds().Dx(),
		Height:   img.Bounds().Dy(),
		Size:     fileInfo.Size(),
		ModTime:  fileInfo.ModTime(),
		EXIFData: make(map[string]string),
	}

	if exifData != nil {
		// Extract specific EXIF fields
		if camModel, err := exifData.Get(exif.Model); err == nil {
			info.EXIFData["Camera Model"] = camModel.String()
		}
		if fNum, err := exifData.Get(exif.FNumber); err == nil {
			numer, denom, _ := fNum.Rat2(0)
			info.EXIFData["F-Number"] = fmt.Sprintf("f/%.1f", float64(numer)/float64(denom))
		}
		if expTime, err := exifData.Get(exif.ExposureTime); err == nil {
			numer, denom, _ := expTime.Rat2(0)
			info.EXIFData["Exposure Time"] = fmt.Sprintf("%d/%d s", numer, denom)
		}
	}

	return info, img, nil
}

// GetImageFingerprint generates a compact fingerprint for an image suitable for duplicate detection.
func (is *ImageService) GetImageFingerprint(path string) (*tagging.Fingerprint, error) {
	if path == "" {
		return nil, fmt.Errorf("image path cannot be empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("getting file stats: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file bytes: %w", err)
	}

	hash := sha256.Sum256(data)
	phash := computePerceptualHash(img)

	return &tagging.Fingerprint{
		Path:           path,
		FileHash:       hex.EncodeToString(hash[:]),
		PerceptualHash: phash,
		Width:          img.Bounds().Dx(),
		Height:         img.Bounds().Dy(),
		Size:           fileInfo.Size(),
		ModTime:        fileInfo.ModTime(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

func computePerceptualHash(img image.Image) string {
	resized := resizeImage(img, 8, 8)
	gray := convertToGray(resized)
	avg := averageBrightness(gray)

	var bits []string
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if gray[y][x] >= avg {
				bits = append(bits, "1")
			} else {
				bits = append(bits, "0")
			}
		}
	}

	var hash uint64
	for i, bit := range bits {
		if bit == "1" {
			hash |= 1 << uint(i)
		}
	}
	return strconv.FormatUint(hash, 16)
}

func resizeImage(img image.Image, width, height int) [][]uint8 {
	bounds := img.Bounds()
	out := make([][]uint8, height)
	for y := 0; y < height; y++ {
		out[y] = make([]uint8, width)
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + (x*bounds.Dx())/width
			srcY := bounds.Min.Y + (y*bounds.Dy())/height
			c := color.NRGBAModel.Convert(img.At(srcX, srcY)).(color.NRGBA)
			out[y][x] = uint8((c.R + c.G + c.B) / 3)
		}
	}
	return out
}

func convertToGray(img [][]uint8) [][]uint8 {
	return img
}

func averageBrightness(gray [][]uint8) uint8 {
	total := 0
	for _, row := range gray {
		for _, value := range row {
			total += int(value)
		}
	}
	count := len(gray) * len(gray[0])
	return uint8(total / count)
}

// GetEmbeddedThumbnail attempts to read an embedded EXIF thumbnail from an image file.
// It returns the decoded thumbnail image or an error if one is not found or cannot be decoded.
func (is *ImageService) GetEmbeddedThumbnail(path string) (img image.Image, err error) {
	// Defer a function to recover from panics that might occur, e.g., from the goexif library
	// with corrupted files. This prevents the entire application from crashing.
	defer func() {
		if r := recover(); r != nil {
			// A panic occurred. Log it and set a descriptive error to be returned.
			log.Printf("PANIC recovered in GetEmbeddedThumbnail for path %s: %v", path, r)
			err = fmt.Errorf("recovered from panic processing EXIF for %s: %v", filepath.Base(path), r)
			img = nil
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file for thumbnail: %w", err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("no EXIF data found in %s", filepath.Base(path))
	}

	// This is the call that can panic with corrupted files. The defer/recover block will catch it.
	thumbBytes, err := x.JpegThumbnail()
	if err != nil {
		return nil, fmt.Errorf("no JPEG thumbnail in EXIF: %w", err)
	}

	img, _, err = image.Decode(bytes.NewReader(thumbBytes))
	return img, err
}
