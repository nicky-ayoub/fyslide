// Package scan operates on files in a directory and its subdirectories
package scan

import (
	"io/fs"
	"log"
	"path/filepath"
	"strings"
)

// FileItem represents a file item. Just a path for now
type FileItem struct {
	Path string
	Info fs.FileInfo
}

// FileItems is a slice of FileItem
type FileItems []FileItem

// NewFileItem creates a new FileItem
func NewFileItem(p string, fi fs.FileInfo) FileItem {
	return FileItem{
		Path: p,
		Info: fi,
	}

}

// findImageFiles recursively scans dir for image files and sends them to the out channel.
// It closes the out channel when done.
func findImageFiles(dir string, out chan<- FileItem) {
	defer close(out) // Ensure channel is closed when WalkDir finishes or panics

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			if d != nil && d.IsDir() && path != dir { // Don't skip the root dir on error
				return filepath.SkipDir // Skip problematic directory
			}
			return nil // Continue if possible, or return err to stop
		}

		if !d.IsDir() && isImage(d.Name()) {
			// Get FileInfo. d.Info() is efficient.
			info, infoErr := d.Info()
			if infoErr != nil {
				log.Printf("Error getting FileInfo for %q: %v\n", path, infoErr)
				return nil // Skip this file
			}
			if info.Size() > 0 { // Ensure it's not an empty file
				out <- NewFileItem(path, info)
			}
		}
		return nil
	})

	if err != nil {
		// Log the error from WalkDir itself, if any.
		// The channel will still be closed by defer.
		log.Printf("Error walking directory %s: %v", dir, err)
	}
}

// Run is the entry point for the package. It now returns a channel
// from which FileItems can be read. The scanning happens in a new goroutine.
func Run(dir string) <-chan FileItem {
	out := make(chan FileItem, 100) // Buffered channel for some decoupling

	go func() {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			log.Printf("Error getting absolute path for %s: %v. Scanning original path.", dir, err)
			absDir = dir // Proceed with original if Abs fails
		}
		findImageFiles(absDir, out)
	}()

	return out
}

func isImage(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}
