package service

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"fyslide/internal/tagging"
)

func TestMergeDuplicateGroupHardLinksAndMergesTags(t *testing.T) {
	dir := t.TempDir()
	repPath := filepath.Join(dir, "rep.png")
	dupPath := filepath.Join(dir, "dup.png")

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	writePNG := func(path string) {
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("os.Create(%q) error = %v", path, err)
		}
		defer file.Close()
		if err := png.Encode(file, img); err != nil {
			t.Fatalf("png.Encode(%q) error = %v", path, err)
		}
	}

	writePNG(repPath)
	writePNG(dupPath)

	tagDB, err := tagging.NewTagDB(dir, nil)
	if err != nil {
		t.Fatalf("NewTagDB() error = %v", err)
	}
	defer tagDB.Close()

	svc := NewService(tagDB, nil, nil)
	svc.ImageService = NewImageService()

	if err := svc.AddTagsToImage(repPath, []string{"alpha"}); err != nil {
		t.Fatalf("AddTagsToImage(rep) error = %v", err)
	}
	if err := svc.AddTagsToImage(dupPath, []string{"beta"}); err != nil {
		t.Fatalf("AddTagsToImage(dup) error = %v", err)
	}

	result, err := svc.MergeDuplicateGroup(tagging.DuplicateGroup{
		RepresentativePath: repPath,
		Members:            []string{repPath, dupPath},
		MatchType:          "exact",
		Confidence:         1.0,
	})
	if err != nil {
		t.Fatalf("MergeDuplicateGroup() error = %v", err)
	}
	if result.MergedPaths[0] != dupPath {
		t.Fatalf("expected duplicate path %q to be merged, got %q", dupPath, result.MergedPaths[0])
	}

	repInfo, err := os.Stat(repPath)
	if err != nil {
		t.Fatalf("os.Stat(repPath) error = %v", err)
	}
	dupInfo, err := os.Stat(dupPath)
	if err != nil {
		t.Fatalf("os.Stat(dupPath) error = %v", err)
	}
	if repInfo.Sys().(*syscall.Stat_t).Ino != dupInfo.Sys().(*syscall.Stat_t).Ino {
		t.Fatalf("expected hard-linked files to share inode, got %d and %d", repInfo.Sys().(*syscall.Stat_t).Ino, dupInfo.Sys().(*syscall.Stat_t).Ino)
	}

	repTags, err := svc.TagDB.GetTags(repPath)
	if err != nil {
		t.Fatalf("GetTags(repPath) error = %v", err)
	}
	if len(repTags) != 2 {
		t.Fatalf("expected 2 merged tags on representative, got %v", repTags)
	}
	dupTags, err := svc.TagDB.GetTags(dupPath)
	if err != nil {
		t.Fatalf("GetTags(dupPath) error = %v", err)
	}
	if len(dupTags) != 0 {
		t.Fatalf("expected duplicate path to have no remaining tags, got %v", dupTags)
	}
}

func TestFindDuplicatesCreatesExactGroup(t *testing.T) {
	dir := t.TempDir()
	pathOne := filepath.Join(dir, "one.png")
	pathTwo := filepath.Join(dir, "two.png")

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	writePNG := func(path string) {
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("os.Create(%q) error = %v", path, err)
		}
		defer file.Close()
		if err := png.Encode(file, img); err != nil {
			t.Fatalf("png.Encode(%q) error = %v", path, err)
		}
	}

	writePNG(pathOne)
	writePNG(pathTwo)

	tagDB, err := tagging.NewTagDB(dir, nil)
	if err != nil {
		t.Fatalf("NewTagDB() error = %v", err)
	}
	defer tagDB.Close()

	svc := NewService(tagDB, nil, nil)
	svc.ImageService = NewImageService()

	groups, err := svc.FindDuplicates(context.Background(), []string{pathOne, pathTwo})
	if err != nil {
		t.Fatalf("FindDuplicates() error = %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if groups[0].MatchType != "exact" {
		t.Fatalf("expected exact match type, got %q", groups[0].MatchType)
	}
	if len(groups[0].Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(groups[0].Members))
	}
}
