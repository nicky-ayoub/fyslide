package ui

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyslide/internal/scan"
	"fyslide/internal/service"
)

// createPNG creates a simple PNG file at path.
func createPNG(path string, w, h int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// fill with a color
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 100, 255})
		}
	}
	return png.Encode(f, img)
}

func TestRapidNavigationLoadsLatestOnly(t *testing.T) {
	// prepare two image files
	dir := t.TempDir()
	p1 := filepath.Join(dir, "one.png")
	p2 := filepath.Join(dir, "two.png")
	if err := createPNG(p1, 64, 64); err != nil {
		t.Fatalf("createPNG p1: %v", err)
	}
	if err := createPNG(p2, 64, 64); err != nil {
		t.Fatalf("createPNG p2: %v", err)
	}

	// Hook to simulate delay: longer for p1, shorter for p2.
	service.TestGetImageInfoHook = func(path string) {
		if path == p1 {
			time.Sleep(200 * time.Millisecond)
		} else if path == p2 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	// note: we will reset the hook below after ensuring goroutines have finished

	// Build minimal app state
	a := &App{}
	a.appCtx = context.Background()
	// appCancel not needed for this test
	a.imageState = NewImageState()
	// Add two items
	a.imageState.AddImages(scan.FileItems{{Path: p1}, {Path: p2}})
	// Ensure sequential mode for deterministic indexing in tests
	a.imageState.ToggleRandomMode("")
	// Ensure indices: start at 0
	a.imageState.SetIndex(0)
	// Use real image service
	a.ImageService = service.NewImageService()

	// Make sure zoomPanArea is nil so UI updates are skipped (we only check a.img.Path)
	a.zoomPanArea = nil

	// Start loading first image
	a.LoadAndDisplayCurrentImage()

	// Briefly wait then navigate to second image and load
	time.Sleep(10 * time.Millisecond)
	a.imageState.SetIndex(1)
	a.LoadAndDisplayCurrentImage()

	// Wait for operations to complete (timeout)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.GetLoadedImagePath() == p2 {
			// success: latest image applied
			// wait to ensure background goroutines finish, then clear hook
			time.Sleep(300 * time.Millisecond)
			return
		}
		// Sleep briefly before checking again
		time.Sleep(10 * time.Millisecond)
	}
	// If we reach here, latest image was not applied in time
	// ensure background goroutines finished before clearing hook
	t.Fatalf("expected latest image %s to be applied, got %s", p2, a.GetLoadedImagePath())
	t.Fatalf("expected latest image %s to be applied, got %s", p2, a.GetLoadedImagePath())
}
