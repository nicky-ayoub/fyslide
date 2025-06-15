package service

import (
	"errors"
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/tagging"
	"os"
	"path/filepath"
	"strings"
)

// TagStore abstracts the tagging DB for easier testing and decoupling.
type TagStore interface {
	AddTag(imagePath, tag string) error
	RemoveTag(imagePath, tag string) error
	GetTags(imagePath string) ([]string, error)
	GetImages(tag string) ([]string, error)
	GetAllTags() ([]tagging.TagWithCount, error)
	RemoveAllTagsForImage(imagePath string) error
	DeleteOrphanedTagKey(tag string) error
	GetAllImagePaths() ([]string, error)
	Close() error
}

// FileScanner abstracts file scanning.
type FileScanner interface {
	Run(dir string, logger scan.LoggerFunc) <-chan scan.FileItem
}

// Service is the main entry point for business logic.
type Service struct {
	TagDB      TagStore
	FileScan   FileScanner
	Logger     func(string)
	Extensions map[string]bool // Supported image extensions
}

// NewService constructs a new Service.
func NewService(tagDB TagStore, fileScan FileScanner, logger func(string)) *Service {
	return &Service{
		TagDB:      tagDB,
		FileScan:   fileScan,
		Logger:     logger,
		Extensions: map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true},
	}
}

// AddTagsToImage adds one or more tags to an image.
func (s *Service) AddTagsToImage(imagePath string, tags []string) error {
	if imagePath == "" || len(tags) == 0 {
		return errors.New("image path and tags required")
	}
	for _, tag := range tags {
		if err := s.TagDB.AddTag(imagePath, tag); err != nil {
			return err
		}
	}
	return nil
}

// RemoveTagsFromImage removes one or more tags from an image.
func (s *Service) RemoveTagsFromImage(imagePath string, tags []string) error {
	if imagePath == "" || len(tags) == 0 {
		return errors.New("image path and tags required")
	}
	for _, tag := range tags {
		if err := s.TagDB.RemoveTag(imagePath, tag); err != nil {
			return err
		}
	}
	return nil
}

// ListTagsForImage returns all tags for a given image.
func (s *Service) ListTagsForImage(imagePath string) ([]string, error) {
	return s.TagDB.GetTags(imagePath)
}

// ListImagesForTag returns all images for a given tag.
func (s *Service) ListImagesForTag(tag string) ([]string, error) {
	return s.TagDB.GetImages(tag)
}

// ListAllTags returns all tags with their image counts.
func (s *Service) ListAllTags() ([]tagging.TagWithCount, error) {
	return s.TagDB.GetAllTags()
}

// BatchAddTagsToDirectory adds tags to all supported images in a directory (non-recursive).
func (s *Service) BatchAddTagsToDirectory(dir string, tags []string) error {
	if dir == "" || len(tags) == 0 {
		return errors.New("directory and tags required")
	}
	files, err := s.scanDirectoryForImages(dir) // Use service method
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := s.AddTagsToImage(file, tags); err != nil {
			s.Logger(fmt.Sprintf("Failed to tag %s: %v", file, err))
		}
	}
	return nil
}

// scanDirectoryForImages lists supported image files in a directory (recursive due to s.FileScan.Run).
func (s *Service) scanDirectoryForImages(dir string) ([]string, error) {
	var files []string
	// Use s.FileScan for testability and s.Logger for logging
	items := s.FileScan.Run(dir, func(msg string) { s.Logger(fmt.Sprintf("scanDirectoryForImages: %s", msg)) })
	for item := range items {
		ext := filepath.Ext(item.Path)
		if s.Extensions[ext] { // Use s.Extensions
			files = append(files, item.Path)
		}
	}
	return files, nil
}

// NormalizeAllTags lowercases all tags in the DB.
func (s *Service) NormalizeAllTags() error {
	allTags, err := s.TagDB.GetAllTags()
	if err != nil {
		return err
	}
	var firstErr error
	for _, tagInfo := range allTags {
		originalTag := tagInfo.Name
		lowerTag := strings.ToLower(originalTag)

		if lowerTag != originalTag {
			images, err := s.TagDB.GetImages(originalTag)
			if err != nil {
				s.Logger(fmt.Sprintf("NormalizeAllTags: failed to get images for tag '%s': %v", originalTag, err))
				if firstErr == nil {
					firstErr = fmt.Errorf("getting images for tag '%s': %w", originalTag, err)
				}
				continue // Skip to next tag
			}
			for _, img := range images {
				if err := s.TagDB.RemoveTag(img, originalTag); err != nil {
					s.Logger(fmt.Sprintf("NormalizeAllTags: failed to remove old tag '%s' from '%s': %v", originalTag, img, err))
					// Consider if this error should be aggregated or returned immediately
				}
				if err := s.TagDB.AddTag(img, lowerTag); err != nil {
					s.Logger(fmt.Sprintf("NormalizeAllTags: failed to add new tag '%s' to '%s': %v", lowerTag, img, err))
					// Consider if this error should be aggregated or returned immediately
				}
			}
			if err := s.TagDB.DeleteOrphanedTagKey(originalTag); err != nil {
				s.Logger(fmt.Sprintf("NormalizeAllTags: failed to delete orphaned old tag '%s': %v", originalTag, err))
			}
		}
	}
	return firstErr
}

// ReplaceTag replaces oldTag with newTag across all images.
func (s *Service) ReplaceTag(oldTag, newTag string) error {
	if oldTag == "" || newTag == "" || oldTag == newTag {
		return errors.New("invalid tags")
	}
	images, err := s.TagDB.GetImages(oldTag)
	if err != nil {
		return err
	}
	var firstErr error
	for _, img := range images {
		if err := s.TagDB.RemoveTag(img, oldTag); err != nil {
			s.Logger(fmt.Sprintf("ReplaceTag: failed to remove old tag '%s' from '%s': %v", oldTag, img, err))
			if firstErr == nil {
				firstErr = fmt.Errorf("removing old tag '%s' from '%s': %w", oldTag, img, err)
			}
		}
		if err := s.TagDB.AddTag(img, newTag); err != nil {
			s.Logger(fmt.Sprintf("ReplaceTag: failed to add new tag '%s' to '%s': %v", newTag, img, err))
			if firstErr == nil {
				firstErr = fmt.Errorf("adding new tag '%s' to '%s': %w", newTag, img, err)
			}
		}
	}
	s.TagDB.DeleteOrphanedTagKey(oldTag) // Error for this is logged by DeleteOrphanedTagKey if it occurs
	return firstErr
}

// RemoveTagGlobally removes a tag from all images in the database.
func (s *Service) RemoveTagGlobally(tag string) (int, int, error) {
	if tag == "" {
		return 0, 0, errors.New("tag cannot be empty")
	}
	imagePaths, err := s.TagDB.GetImages(tag)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get images for tag '%s': %w", tag, err)
	}
	successfulRemovals := 0
	errorsEncountered := 0
	for _, path := range imagePaths {
		if err := s.TagDB.RemoveTag(path, tag); err != nil {
			s.Logger(fmt.Sprintf("Error removing tag '%s' from %s: %v", tag, path, err))
			errorsEncountered++
		} else {
			successfulRemovals++
		}
	}
	return successfulRemovals, errorsEncountered, nil
}

// BatchRemoveTagsFromDirectory removes tags from all supported images in a directory (non-recursive).
func (s *Service) BatchRemoveTagsFromDirectory(dir string, tags []string) (int, int, error) {
	if dir == "" || len(tags) == 0 {
		return 0, 0, errors.New("directory and tags required")
	}
	files, err := s.scanDirectoryForImages(dir) // Use service method
	if err != nil {
		return 0, 0, err
	}
	successfulRemovals := 0
	errorsEncountered := 0
	for _, file := range files {
		for _, tag := range tags {
			if err := s.TagDB.RemoveTag(file, tag); err != nil {
				s.Logger(fmt.Sprintf("Error removing tag '%s' from %s: %v", tag, file, err))
				errorsEncountered++
			} else {
				successfulRemovals++
			}
		}
	}
	return successfulRemovals, errorsEncountered, nil
}

// CleanDatabase removes tags for non-existent files and deletes orphaned tags.
func (s *Service) CleanDatabase() (filesCleaned, tagsCleaned int, err error) {
	// Phase 1: Remove tags for non-existent files
	imagePaths, err := s.TagDB.GetAllImagePaths()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get image paths: %w", err)
	}
	for _, imagePath := range imagePaths {
		if _, statErr := os.Stat(imagePath); os.IsNotExist(statErr) {
			if err := s.TagDB.RemoveAllTagsForImage(imagePath); err != nil {
				s.Logger(fmt.Sprintf("Error removing tags for non-existent file %s: %v", imagePath, err))
			} else {
				filesCleaned++
			}
		}
	}

	// Phase 2: Remove orphaned tags
	allTags, err := s.TagDB.GetAllTags()
	if err != nil {
		return filesCleaned, 0, fmt.Errorf("failed to get all tags: %w", err)
	}
	for _, tagInfo := range allTags {
		if tagInfo.Count == 0 {
			if err := s.TagDB.DeleteOrphanedTagKey(tagInfo.Name); err != nil {
				s.Logger(fmt.Sprintf("Error removing orphaned tag '%s': %v", tagInfo.Name, err))
			} else {
				tagsCleaned++
			}
		}
	}
	return filesCleaned, tagsCleaned, nil
}

// DeleteImageFile deletes an image file from disk and removes all its tags from the database.
func (s *Service) DeleteImageFile(imagePath string) error {
	if imagePath == "" {
		return errors.New("image path required")
	}
	if err := os.Remove(imagePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", imagePath, err)
	}
	if err := s.TagDB.RemoveAllTagsForImage(imagePath); err != nil {
		return fmt.Errorf("failed to remove tags for deleted file %s: %w", imagePath, err)
	}
	return nil
}

// AddTagsToTaggedImages adds new tags to all images that already have a specific tag.
func (s *Service) AddTagsToTaggedImages(existingTag string, tagsToAdd []string) (int, error) {
	if existingTag == "" || len(tagsToAdd) == 0 {
		return 0, errors.New("existing tag and tags to add required")
	}
	imagePaths, err := s.TagDB.GetImages(existingTag)
	if err != nil {
		return 0, fmt.Errorf("failed to get images for tag '%s': %w", existingTag, err)
	}
	added := 0
	for _, img := range imagePaths {
		for _, tag := range tagsToAdd {
			if err := s.TagDB.AddTag(img, tag); err != nil {
				s.Logger(fmt.Sprintf("Error adding tag '%s' to %s: %v", tag, img, err))
			} else {
				added++
			}
		}
	}
	return added, nil
}

// ApplyTagsToSingleImage applies a list of tags to a single image path.
// It returns the number of successful additions, errors encountered, and the first error (if any).
func (s *Service) ApplyTagsToSingleImage(imagePath string, tagsToAdd []string, filesAffected map[string]bool) (successfulAdditions int, errorsEncountered int, firstError error) {
	s.Logger(fmt.Sprintf("Applying tag(s) [%s] to %s", strings.Join(tagsToAdd, ", "), filepath.Base(imagePath)))
	for _, tag := range tagsToAdd {
		errAdd := s.TagDB.AddTag(imagePath, tag) // More direct for single tag
		if errAdd != nil {
			errorsEncountered++
			if firstError == nil {
				firstError = fmt.Errorf("failed to add tag '%s' to %s: %w", tag, filepath.Base(imagePath), errAdd)
			}
		} else {
			successfulAdditions++
			filesAffected[imagePath] = true
		}
	}
	s.Logger(fmt.Sprintf("Applied tags to %s. Successes: %d, Errors: %d", filepath.Base(imagePath), successfulAdditions, errorsEncountered))
	return
}
