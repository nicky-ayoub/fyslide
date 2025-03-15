// filepath: /home/nicky/src/go/fyslide/internal/scan/files_test.go
package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileItem(t *testing.T) {
	path := "test/path"
	item := NewFileItem(path)
	if item.Path != path {
		t.Errorf("expected %s, got %s", path, item.Path)
	}
}

func TestIsImage(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"image.PNG", true},
		{"image.jpg", true},
		{"image.jpeg", true},
		{"image.gif", true},
		{"image.txt", false},
		{"image", false},
	}

	for _, test := range tests {
		result := isImage(test.name)
		if result != test.expected {
			t.Errorf("isImage(%s) = %v; want %v", test.name, result, test.expected)
		}
	}
}

func TestSearchDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test files
	files := []string{"image1.png", "image2.jpg", "text.txt"}
	for _, file := range files {
		err := os.WriteFile(filepath.Join(dir, file), []byte("test"), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	var items FileItems
	err = searchDir(dir, &items)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestSearchTree(t *testing.T) {
	dir, err := os.MkdirTemp("", "testtree")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test files and directories
	subDir := filepath.Join(dir, "subdir")
	err = os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	files := []string{"image1.png", "image2.jpg", "text.txt", "subdir/image3.gif"}
	for _, file := range files {
		err := os.WriteFile(filepath.Join(dir, file), []byte("test"), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	var items FileItems
	err = searchTree(dir, &items)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestRun(t *testing.T) {
	dir, err := os.MkdirTemp("", "testrun")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test files
	files := []string{"image1.png", "image2.jpg", "text.txt"}
	for _, file := range files {
		err := os.WriteFile(filepath.Join(dir, file), []byte("test"), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	var items FileItems
	Run(dir, &items)

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}
