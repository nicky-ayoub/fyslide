package scan

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileItem struct {
	Size    int64
	ModTime time.Time
	Path    string
}

var mu sync.Mutex

type FileItems []FileItem

func data(p string, fi os.FileInfo) FileItem {
	item :=
		FileItem{
			Size:    fi.Size(),
			ModTime: fi.ModTime(),
			Path:    p,
		}
	return item
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
			*m = append(*m, data(p, fi))
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

	// libRegEx, err := regexp.Compile(`(?i)^.+\.(png|jpg|jpeg|gif)$`)
	// if err != nil {
	// 	return false
	// }
	// return libRegEx.MatchString(n)

	ext := strings.ToLower(filepath.Ext(n))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}
