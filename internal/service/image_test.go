package service

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestGetImageFingerprint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.png")

	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x < 8 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
			}
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create() error = %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	svc := NewImageService()
	fingerprint, err := svc.GetImageFingerprint(path)
	if err != nil {
		t.Fatalf("GetImageFingerprint() error = %v", err)
	}

	if fingerprint.Path != path {
		t.Fatalf("fingerprint path = %q, want %q", fingerprint.Path, path)
	}
	if fingerprint.FileHash == "" {
		t.Fatal("fingerprint file hash should not be empty")
	}
	if fingerprint.PerceptualHash == "" {
		t.Fatal("fingerprint perceptual hash should not be empty")
	}
	if fingerprint.Width != 16 || fingerprint.Height != 16 {
		t.Fatalf("fingerprint dimensions = %dx%d, want 16x16", fingerprint.Width, fingerprint.Height)
	}
}
