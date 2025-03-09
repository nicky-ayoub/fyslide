// Package scan operates on files in a directory and its subdirectories
package scan

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileItem represents a file item. Just a path for now
type FileItem struct {
	Path string
}

var mu sync.Mutex

// FileItems is a slice of FileItem
type FileItems []FileItem

// NewFileItem creates a new FileItem
func NewFileItem(p string) FileItem {
	return FileItem{
		Path: p,
	}

}

func searchTree(dir string, m *FileItems) error {

	visit := func(p string, fi os.FileInfo, err error) error {
		if err != nil && err != os.ErrNotExist {
			return err
		}

		// ignore dir itself to avoid an infinite loop!
		if fi.Mode().IsDir() && p != dir {
			searchTree(p, m)
			return filepath.SkipDir
		}

		if fi.Mode().IsRegular() && fi.Size() > 0 && IsImage(p) {
			mu.Lock()
			*m = append(*m, NewFileItem(p))
			mu.Unlock()
		}

		return nil
	}

	return filepath.Walk(dir, visit)
}

// Run is the entry point for the package
func Run(dir string, m *FileItems) {
	searchTree(dir, m)
}

// IsImage checks if a file is an image
func IsImage(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}
