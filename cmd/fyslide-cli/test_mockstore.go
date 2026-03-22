package main

import (
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/tagging"
	"sync"

	"github.com/spf13/cobra"
)

// MockTagStore is a lightweight in-memory implementation of service. TagStore intended for fast, isolated unit tests.
type MockTagStore struct {
	mu sync.RWMutex
	// maps for bidirectional relationships
	imgToTags map[string]map[string]struct{}
	tagToImgs map[string]map[string]struct{}
}

// NewMockTagStore initializes and returns a new MockTagStore instance.
func NewMockTagStore() *MockTagStore {
	return &MockTagStore{
		imgToTags: make(map[string]map[string]struct{}),
		tagToImgs: make(map[string]map[string]struct{}),
	}
}

// AddTag adds a single tag to an image.
func (m *MockTagStore) AddTag(imagePath, tag string) error {
	return m.AddTagsToImage(imagePath, []string{tag})
}

// AddTagsToImage adds multiple tags to a single image.
func (m *MockTagStore) AddTagsToImage(imagePath string, tags []string) error {
	return m.AddTagsToImageList([]string{imagePath}, tags)
}

// AddTagsToImageList adds multiple tags to multiple images.
func (m *MockTagStore) AddTagsToImageList(imagePaths []string, tags []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, img := range imagePaths {
		if _, ok := m.imgToTags[img]; !ok {
			m.imgToTags[img] = make(map[string]struct{})
		}
		for _, tag := range tags {
			if tag == "" {
				continue
			}
			m.imgToTags[img][tag] = struct{}{}
			if _, ok := m.tagToImgs[tag]; !ok {
				m.tagToImgs[tag] = make(map[string]struct{})
			}
			m.tagToImgs[tag][img] = struct{}{}
		}
	}
	return nil
}

// RemoveTag removes a single tag from an image.
func (m *MockTagStore) RemoveTag(imagePath, tag string) error {
	return m.RemoveTagsFromImage(imagePath, []string{tag})
}

// RemoveTagsFromImage removes multiple tags from a single image.
func (m *MockTagStore) RemoveTagsFromImage(imagePath string, tags []string) error {
	return m.RemoveTagsFromImageList([]string{imagePath}, tags)
}

// RemoveTagsFromImageList removes multiple tags from multiple images.
func (m *MockTagStore) RemoveTagsFromImageList(imagePaths []string, tags []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, img := range imagePaths {
		if _, ok := m.imgToTags[img]; !ok {
			continue
		}
		for _, tag := range tags {
			delete(m.imgToTags[img], tag)
			if imgs, ok := m.tagToImgs[tag]; ok {
				delete(imgs, img)
				if len(imgs) == 0 {
					delete(m.tagToImgs, tag)
				}
			}
		}
		if len(m.imgToTags[img]) == 0 {
			delete(m.imgToTags, img)
		}
	}
	return nil
}

// GetTags returns all tags associated with a given image.
func (m *MockTagStore) GetTags(imagePath string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set := m.imgToTags[imagePath]
	if set == nil {
		return []string{}, nil
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	return out, nil
}

// GetImages returns all images associated with a given tag.
func (m *MockTagStore) GetImages(tag string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set := m.tagToImgs[tag]
	if set == nil {
		return []string{}, nil
	}
	out := make([]string, 0, len(set))
	for img := range set {
		out = append(out, img)
	}
	return out, nil
}

// GetAllTags returns all tags in the store along with their associated image counts.
func (m *MockTagStore) GetAllTags() ([]tagging.TagWithCount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]tagging.TagWithCount, 0, len(m.tagToImgs))
	for tag, imgs := range m.tagToImgs {
		out = append(out, tagging.TagWithCount{Name: tag, Count: len(imgs)})
	}
	return out, nil
}

// ReplaceTag replaces all occurrences of oldTag with newTag across all images.
func (m *MockTagStore) ReplaceTag(oldTag, newTag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	imgs, ok := m.tagToImgs[oldTag]
	if !ok {
		return nil
	}
	for img := range imgs {
		// remove old
		delete(m.imgToTags[img], oldTag)
		// add new
		if _, ok := m.imgToTags[img]; !ok {
			m.imgToTags[img] = make(map[string]struct{})
		}
		m.imgToTags[img][newTag] = struct{}{}
		// add to tag map
		if _, ok := m.tagToImgs[newTag]; !ok {
			m.tagToImgs[newTag] = make(map[string]struct{})
		}
		m.tagToImgs[newTag][img] = struct{}{}
	}
	delete(m.tagToImgs, oldTag)
	return nil
}

// RemoveAllTagsForImage removes all tags associated with a given image.
func (m *MockTagStore) RemoveAllTagsForImage(imagePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tags := m.imgToTags[imagePath]
	for tag := range tags {
		if imgs, ok := m.tagToImgs[tag]; ok {
			delete(imgs, imagePath)
			if len(imgs) == 0 {
				delete(m.tagToImgs, tag)
			}
		}
	}
	delete(m.imgToTags, imagePath)
	return nil
}

// DeleteOrphanedTagKey deletes a tag key that has no associated images. In this mock implementation, we simply delete the tag key from the map.
func (m *MockTagStore) DeleteOrphanedTagKey(tag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tagToImgs, tag)
	return nil
}

// GetAllImagePaths returns a slice of all image paths that have tags in the store.
func (m *MockTagStore) GetAllImagePaths() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.imgToTags))
	for img := range m.imgToTags {
		out = append(out, img)
	}
	return out, nil
}

// Close is a no-op for MockTagStore since it doesn't hold any resources, but it's defined to satisfy the interface.
func (m *MockTagStore) Close() error { return nil }

// StreamAllImagePaths sends all image paths that have tags in the store to the provided channel. It closes the channel when done.
func (m *MockTagStore) StreamAllImagePaths(pathChan chan<- string) {
	defer close(pathChan)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for img := range m.imgToTags {
		pathChan <- img
	}
}

// newTestRootCmdMock returns a root command wired to a fresh MockTagStore.
func newTestRootCmdMock() *cobra.Command {
	mock := NewMockTagStore()
	getSvcAndDB := func(_ string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
		s := service.NewService(mock, &scan.FileScannerImpl{}, logger)
		return s, &tagging.TagDB{}, nil
	}
	return NewRootCmd(getSvcAndDB)
}

// newTestRootCmdMockFactory returns a factory function that creates root commands
// which share a single MockTagStore instance. Use this when a test calls the
// returned function multiple times but expects shared state across invocations.
func newTestRootCmdMockFactory() func() *cobra.Command {
	mock := NewMockTagStore()
	return func() *cobra.Command {
		getSvcAndDB := func(_ string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			s := service.NewService(mock, &scan.FileScannerImpl{}, logger)
			return s, &tagging.TagDB{}, nil
		}
		return NewRootCmd(getSvcAndDB)
	}
}
