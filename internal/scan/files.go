package scan

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FileItem struct {
	Path string
}

var mu sync.Mutex

type FileItems []FileItem

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

func Run(dir string, m *FileItems) {
	searchTree(dir, m)
}

func IsImage(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}
