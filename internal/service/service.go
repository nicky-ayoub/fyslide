package service

import (
	"context"
	"errors"
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/tagging"
	"os"
	"sort"
	"strings"
	"syscall"
)

// TagStore abstracts the tagging DB for easier testing and decoupling.
type TagStore interface {
	AddTag(imagePath, tag string) error
	AddTagsToImage(imagePath string, tags []string) error // New method for batch adding to a single image
	AddTagsToImageList(imagePaths []string, tags []string) error
	RemoveTag(imagePath, tag string) error
	RemoveTagsFromImage(imagePath string, tags []string) error
	RemoveTagsFromImageList(imagePaths []string, tags []string) error
	GetTags(imagePath string) ([]string, error)
	GetImages(tag string) ([]string, error)
	GetAllTags() ([]tagging.TagWithCount, error)
	ReplaceTag(oldTag, newTag string) error
	RemoveAllTagsForImage(imagePath string) error
	DeleteOrphanedTagKey(tag string) error
	GetAllImagePaths() ([]string, error)
	Close() error
	StreamAllImagePaths(pathChan chan<- string)
}

// FileScanner abstracts file scanning.
type FileScanner interface {
	Run(ctx context.Context, dir string, logger scan.LoggerFunc) <-chan scan.FileItem
}

// Service is the main entry point for business logic.
type Service struct {
	TagDB        TagStore
	FileScan     FileScanner
	Logger       func(string)
	ImageService *ImageService
}

// MergeResult describes the outcome of merging a duplicate group.
type MergeResult struct {
	RepresentativePath string
	MergedPaths        []string
	MergedTags         []string
}

// NewService constructs a new Service.
func NewService(tagDB TagStore, fileScan FileScanner, logger func(string)) *Service {
	return &Service{
		TagDB:        tagDB,
		FileScan:     fileScan,
		Logger:       logger,
		ImageService: NewImageService(),
	}
}

// FindDuplicates scans a set of image paths, computes fingerprints, and returns duplicate groups.
func (s *Service) FindDuplicates(ctx context.Context, paths []string) ([]tagging.DuplicateGroup, error) {
	if len(paths) == 0 {
		return []tagging.DuplicateGroup{}, nil
	}
	if s.ImageService == nil {
		s.ImageService = NewImageService()
	}
	if s.TagDB == nil {
		return nil, errors.New("tag database not configured")
	}

	fingerprints := make(map[string]tagging.Fingerprint, len(paths))
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if path == "" {
			continue
		}
		fp, err := s.ImageService.GetImageFingerprint(path)
		if err != nil {
			continue
		}
		fingerprints[fp.Path] = *fp
		if db, ok := s.TagDB.(interface {
			SaveFingerprint(tagging.Fingerprint) error
		}); ok {
			if err := db.SaveFingerprint(*fp); err != nil && s.Logger != nil {
				s.Logger(fmt.Sprintf("failed to save fingerprint for %s: %v", path, err))
			}
		}
	}

	groupByHash := make(map[string][]string)
	for _, fp := range fingerprints {
		if fp.FileHash != "" {
			groupByHash[fp.FileHash] = append(groupByHash[fp.FileHash], fp.Path)
		}
	}

	var groups []tagging.DuplicateGroup
	for _, members := range groupByHash {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		groups = append(groups, tagging.DuplicateGroup{
			RepresentativePath: members[0],
			Members:            members,
			MatchType:          "exact",
			Confidence:         1.0,
		})
	}

	for _, group := range groups {
		if db, ok := s.TagDB.(interface {
			SaveDuplicateGroup(tagging.DuplicateGroup) error
		}); ok {
			if err := db.SaveDuplicateGroup(group); err != nil && s.Logger != nil {
				s.Logger(fmt.Sprintf("failed to save duplicate group for %s: %v", group.RepresentativePath, err))
			}
		}
	}

	return groups, nil
}

// MergeDuplicateGroup hard-links exact duplicate files on the same filesystem and merges tags onto the representative.
func (s *Service) MergeDuplicateGroup(group tagging.DuplicateGroup) (*MergeResult, error) {
	if group.RepresentativePath == "" || len(group.Members) < 2 {
		return nil, errors.New("duplicate group must contain a representative and at least one other member")
	}
	if s.TagDB == nil {
		return nil, errors.New("tag database not configured")
	}
	if s.ImageService == nil {
		s.ImageService = NewImageService()
	}

	representative := group.RepresentativePath
	repFingerprint, err := s.ImageService.GetImageFingerprint(representative)
	if err != nil {
		return nil, fmt.Errorf("reading representative fingerprint: %w", err)
	}

	var mergedPaths []string
	var mergedTags []string
	seenTags := make(map[string]struct{})

	representativeInfo, err := os.Stat(representative)
	if err != nil {
		return nil, fmt.Errorf("statting representative: %w", err)
	}
	repStat, ok := representativeInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, errors.New("failed to read representative filesystem metadata")
	}

	for _, member := range group.Members {
		if member == "" || member == representative {
			continue
		}

		memberFingerprint, err := s.ImageService.GetImageFingerprint(member)
		if err != nil {
			return nil, fmt.Errorf("reading fingerprint for %s: %w", member, err)
		}
		if memberFingerprint.FileHash != repFingerprint.FileHash {
			return nil, fmt.Errorf("member %s does not match the representative file hash", member)
		}

		memberInfo, err := os.Stat(member)
		if err != nil {
			return nil, fmt.Errorf("statting member %s: %w", member, err)
		}
		memberStat, ok := memberInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("failed to read filesystem metadata for %s", member)
		}
		if repStat.Dev != memberStat.Dev {
			return nil, fmt.Errorf("member %s is not on the same filesystem as the representative", member)
		}

		if err := os.Remove(member); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("removing existing member %s before hard-link: %w", member, err)
		}
		if err := os.Link(representative, member); err != nil {
			return nil, fmt.Errorf("hard-linking %s to %s: %w", representative, member, err)
		}
		mergedPaths = append(mergedPaths, member)

		memberTags, err := s.TagDB.GetTags(member)
		if err != nil {
			return nil, fmt.Errorf("getting tags for %s: %w", member, err)
		}
		for _, tag := range memberTags {
			if _, ok := seenTags[tag]; ok {
				continue
			}
			seenTags[tag] = struct{}{}
			mergedTags = append(mergedTags, tag)
		}
		if err := s.TagDB.RemoveAllTagsForImage(member); err != nil {
			return nil, fmt.Errorf("removing tags from %s: %w", member, err)
		}
	}

	if len(mergedTags) > 0 {
		if err := s.AddTagsToImageList([]string{representative}, mergedTags); err != nil {
			return nil, fmt.Errorf("adding merged tags to representative: %w", err)
		}
	}

	return &MergeResult{RepresentativePath: representative, MergedPaths: mergedPaths, MergedTags: mergedTags}, nil
}

// AddTagsToImage adds one or more tags to an image.
func (s *Service) AddTagsToImage(imagePath string, tags []string) error {
	if imagePath == "" || len(tags) == 0 {
		return errors.New("image path and tags required")
	}
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}
	return s.TagDB.AddTagsToImage(imagePath, lowerTags)
}

// RemoveTagsFromImage removes one or more tags from an image.
func (s *Service) RemoveTagsFromImage(imagePath string, tags []string) error {
	if imagePath == "" || len(tags) == 0 {
		return errors.New("image path and tags required")
	}
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}
	// This now uses a single transaction at the DB layer.
	return s.TagDB.RemoveTagsFromImage(imagePath, lowerTags)
}

// ListTagsForImage returns all tags for a given image.
func (s *Service) ListTagsForImage(imagePath string) ([]string, error) {
	return s.TagDB.GetTags(imagePath)
	// Tags are stored in lowercase, so they will be retrieved in lowercase.
}

// ListImagesForTag returns all images for a given tag.
func (s *Service) ListImagesForTag(tag string) ([]string, error) {
	// Queries should also use lowercase for consistency.
	return s.TagDB.GetImages(strings.ToLower(tag))
}

// ListAllTags returns all tags with their image counts.
func (s *Service) ListAllTags() ([]tagging.TagWithCount, error) {
	// Tags are stored in lowercase, so they will be retrieved in lowercase.
	return s.TagDB.GetAllTags()
}

// GetAllImagePaths retrieves all unique image paths from the database.
func (s *Service) GetAllImagePaths() ([]string, error) {
	return s.TagDB.GetAllImagePaths()
}

// StreamAllImagePaths returns a channel that streams all known image paths from the database.
func (s *Service) StreamAllImagePaths() <-chan string {
	pathChan := make(chan string, 100) // Buffered channel
	go s.TagDB.StreamAllImagePaths(pathChan)
	return pathChan
}

// FindImagesByTags finds images that have ALL of the given tags.
func (s *Service) FindImagesByTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}

	// Normalize all tags to lowercase for the query
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}

	// Get initial set of images from the first tag
	initialPaths, err := s.TagDB.GetImages(lowerTags[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get images for tag '%s': %w", lowerTags[0], err)
	}

	if len(initialPaths) == 0 {
		return []string{}, nil
	}

	// Create a set for efficient lookups
	pathSet := make(map[string]struct{}, len(initialPaths))
	for _, path := range initialPaths {
		pathSet[path] = struct{}{}
	}

	// Intersect with images from subsequent tags
	for i := 1; i < len(lowerTags); i++ {
		tag := lowerTags[i]
		nextTagPaths, err := s.TagDB.GetImages(tag)
		if err != nil {
			return nil, fmt.Errorf("failed to get images for tag '%s': %w", tag, err)
		}

		intersection := make(map[string]struct{})
		for _, path := range nextTagPaths {
			if _, ok := pathSet[path]; ok {
				intersection[path] = struct{}{}
			}
		}
		pathSet = intersection

		if len(pathSet) == 0 {
			return []string{}, nil // Early exit if intersection is empty
		}
	}

	// Convert the final set back to a slice
	finalPaths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		finalPaths = append(finalPaths, path)
	}
	sort.Strings(finalPaths) // For consistent output

	return finalPaths, nil
}

// BatchAddTagsToDirectory adds tags to all supported images in a directory (recursive).
func (s *Service) BatchAddTagsToDirectory(ctx context.Context, dir string, tags []string) error {
	if dir == "" || len(tags) == 0 {
		return errors.New("directory and tags required")
	}
	files, err := s.scanDirectoryForImages(ctx, dir) // Use service method
	if err != nil {
		return err
	}
	return s.AddTagsToImageList(files, tags)
}

// ReplaceTag replaces oldTag with newTag across all images.
func (s *Service) ReplaceTag(oldTag, newTag string) error {
	if oldTag == "" || newTag == "" || oldTag == newTag {
		return errors.New("invalid tags")
	}
	// Normalization is handled at the DB layer.
	return s.TagDB.ReplaceTag(strings.ToLower(oldTag), strings.ToLower(newTag))
}

// RemoveTagGlobally removes a tag from all images in the database.
func (s *Service) RemoveTagGlobally(tag string) (int, int, error) {
	if tag == "" {
		return 0, 0, errors.New("tag cannot be empty")
	}
	lowerTag := strings.ToLower(tag)
	imagePaths, err := s.TagDB.GetImages(lowerTag)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get images for tag '%s': %w", tag, err)
	}
	successfulRemovals := 0
	errorsEncountered := 0
	for _, path := range imagePaths {
		if err := s.TagDB.RemoveTag(path, lowerTag); err != nil { // Use lowerTag
			s.Logger(fmt.Sprintf("Error removing tag '%s' from %s: %v", lowerTag, path, err))
			errorsEncountered++
		} else {
			successfulRemovals++
		}
	}
	return successfulRemovals, errorsEncountered, nil
}

// CleanDatabase performs maintenance on the tag database.
// It runs in two phases:
// 1. It removes all tag entries for image files that no longer exist on disk.

// BatchRemoveTagsFromDirectory removes tags from all supported images in a directory (recursive).
func (s *Service) BatchRemoveTagsFromDirectory(ctx context.Context, dir string, tags []string) error {
	if dir == "" || len(tags) == 0 {
		return errors.New("directory and tags required")
	}
	files, err := s.scanDirectoryForImages(ctx, dir) // Use service method
	if err != nil {
		return err
	}
	return s.RemoveTagsFromImageList(files, tags)
}

// 2. It removes tag keys that are no longer associated with any images (orphaned tags).
// It returns the number of files and tags cleaned, and any error encountered.

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

// AddTagsToTaggedImages finds all images with `existingTag` and adds `tagsToAdd` to them.
// This is useful for creating tag hierarchies or groups.
// AddTagsToTaggedImages adds new tags to all images that already have a specific tag.
func (s *Service) AddTagsToTaggedImages(existingTag string, tagsToAdd []string) (int, error) {
	if existingTag == "" || len(tagsToAdd) == 0 {
		return 0, errors.New("existing tag and tags to add required")
	}
	imagePaths, err := s.ListImagesForTag(existingTag) // This already handles lowercasing
	if err != nil {
		return 0, err
	}
	if len(imagePaths) == 0 {
		return 0, nil
	}
	if err := s.AddTagsToImageList(imagePaths, tagsToAdd); err != nil {
		return 0, err
	}
	return len(imagePaths) * len(tagsToAdd), nil
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

// AddTagsToImageList adds a list of tags to a list of images.
func (s *Service) AddTagsToImageList(imagePaths []string, tags []string) error {
	if len(imagePaths) == 0 || len(tags) == 0 {
		return nil
	}
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}
	return s.TagDB.AddTagsToImageList(imagePaths, lowerTags)
}

// RemoveTagsFromImageList removes a list of tags from a list of images.
func (s *Service) RemoveTagsFromImageList(imagePaths []string, tags []string) error {
	if len(imagePaths) == 0 || len(tags) == 0 {
		return nil
	}
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}
	return s.TagDB.RemoveTagsFromImageList(imagePaths, lowerTags)
}

// scanDirectoryForImages lists supported image files in a directory (recursive due to s.FileScan.Run).
func (s *Service) scanDirectoryForImages(ctx context.Context, dir string) ([]string, error) {
	var files []string
	// Use s.FileScan for testability and s.Logger for logging
	items := s.FileScan.Run(ctx, dir, func(msg string) { s.Logger(fmt.Sprintf("scanDirectoryForImages: %s", msg)) })
	for item := range items {
		files = append(files, item.Path)
	}
	return files, nil
}
