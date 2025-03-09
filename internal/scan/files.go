// Package scan operates on files in a directory and its subdirectories
package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileItem represents a file item. Just a path for now
type FileItem struct {
	Path string
}

// FileItems is a slice of FileItem
type FileItems []FileItem

// NewFileItem creates a new FileItem
func NewFileItem(p string) FileItem {
	return FileItem{
		Path: p,
	}

}

func searchDir(dir string, m *FileItems) error {

	visit := func(p string, d fs.DirEntry, err error) error {
		if err != nil && err != os.ErrNotExist {
			return err
		}

		// ignore dir itself to avoid an infinite loop!
		if d.IsDir() && p != dir {
			searchTree(p, m)
			return filepath.SkipDir
		}

		if !d.IsDir() && isImage(p) {
			//mu.Lock()
			*m = append(*m, NewFileItem(p))
			//mu.Unlock()
		}

		return nil
	}

	return filepath.WalkDir(dir, visit)
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

		if fi.Mode().IsRegular() && fi.Size() > 0 && isImage(p) {
			*m = append(*m, NewFileItem(p))
		}

		return nil
	}

	return filepath.Walk(dir, visit)
}

// Run is the entry point for the package
func Run(dir string, m *FileItems) {
	searchDir(dir, m)
}

func isImage(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}
