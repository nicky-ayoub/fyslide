// Package tagging provides functionality for managing image tags using a BoltDB database.
// It allows adding, removing, and retrieving tags associated with image paths.
// It also provides a way to retrieve all unique tags in the database.
package tagging // Or place within your ui package if preferred

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFileName         = "fyslide_tags.db"
	imagesToTagsBucket = "ImagesToTags"
	tagsToImagesBucket = "TagsToImages"
)

// TagDB manages the tagging database.
type TagDB struct {
	db *bolt.DB
}

// NewTagDB creates or opens the tag database file.
// dbDir specifies the directory where the db file should be stored.
func NewTagDB(dbDir string) (*TagDB, error) {
	if dbDir == "" {
		// Default to user config directory or current directory if needed
		configDir, err := os.UserConfigDir()
		if err != nil {
			log.Printf("Warning: Could not get user config dir: %v. Using current dir.", err)
			dbDir = "." // Fallback to current directory
		} else {
			dbDir = filepath.Join(configDir, "fyslide") // App specific subfolder
			// Ensure the directory exists
			if err := os.MkdirAll(dbDir, 0750); err != nil {
				return nil, fmt.Errorf("failed to create config directory %s: %w", dbDir, err)
			}
		}
	}

	dbPath := filepath.Join(dbDir, dbFileName)
	log.Printf("Using tag database at: %s", dbPath)

	db, err := bolt.Open(dbPath, 0600, nil) // 0600 permissions: user read/write
	if err != nil {
		return nil, fmt.Errorf("failed to open tag database %s: %w", dbPath, err)
	}

	// Ensure buckets exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(imagesToTagsBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", imagesToTagsBucket, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(tagsToImagesBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", tagsToImagesBucket, err)
		}
		return nil
	})

	if err != nil {
		db.Close() // Close DB if bucket creation failed
		return nil, err
	}

	return &TagDB{db: db}, nil
}

// Close closes the database connection.
func (tdb *TagDB) Close() error {
	if tdb.db != nil {
		return tdb.db.Close()
	}
	return nil
}

// --- Helper Functions ---

func encodeList(list []string) ([]byte, error) {
	return json.Marshal(list)
}

func decodeList(data []byte) ([]string, error) {
	var list []string
	if data == nil { // Handle case where key doesn't exist yet
		return []string{}, nil
	}
	err := json.Unmarshal(data, &list)
	return list, err
}

// Adds an item to a list only if it's not already present. Returns true if added.
func addToList(list []string, item string) ([]string, bool) {
	for _, existing := range list {
		if existing == item {
			return list, false // Already exists
		}
	}
	return append(list, item), true
}

// Removes an item from a list. Returns the modified list.
func removeFromList(list []string, item string) []string {
	newList := list[:0] // Re-slice with 0 length but keep capacity
	for _, existing := range list {
		if existing != item {
			newList = append(newList, existing)
		}
	}
	return newList
}

// --- Core Tagging Functions ---

// AddTag associates a tag with an image path.
func (tdb *TagDB) AddTag(imagePath string, tag string) error {
	if imagePath == "" || tag == "" {
		return fmt.Errorf("image path and tag cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		imgBucket := tx.Bucket([]byte(imagesToTagsBucket))
		tagBucket := tx.Bucket([]byte(tagsToImagesBucket))

		// 1. Update Image -> Tags mapping
		currentTagsBytes := imgBucket.Get([]byte(imagePath))
		currentTags, err := decodeList(currentTagsBytes)
		if err != nil {
			return fmt.Errorf("failed to decode tags for image %s: %w", imagePath, err)
		}

		updatedTags, added := addToList(currentTags, tag)
		if added { // Only update if the tag was actually added
			updatedTagsBytes, err := encodeList(updatedTags)
			if err != nil {
				return fmt.Errorf("failed to encode updated tags for image %s: %w", imagePath, err)
			}
			if err := imgBucket.Put([]byte(imagePath), updatedTagsBytes); err != nil {
				return fmt.Errorf("failed to put updated tags for image %s: %w", imagePath, err)
			}
		}

		// 2. Update Tag -> Images mapping
		currentImagesBytes := tagBucket.Get([]byte(tag))
		currentImages, err := decodeList(currentImagesBytes)
		if err != nil {
			return fmt.Errorf("failed to decode images for tag %s: %w", tag, err)
		}

		updatedImages, added := addToList(currentImages, imagePath)
		if added { // Only update if the image path was actually added
			updatedImagesBytes, err := encodeList(updatedImages)
			if err != nil {
				return fmt.Errorf("failed to encode updated images for tag %s: %w", tag, err)
			}
			if err := tagBucket.Put([]byte(tag), updatedImagesBytes); err != nil {
				return fmt.Errorf("failed to put updated images for tag %s: %w", tag, err)
			}
		}

		return nil
	})
}

// RemoveTag disassociates a tag from an image path.
func (tdb *TagDB) RemoveTag(imagePath string, tag string) error {
	if imagePath == "" || tag == "" {
		return fmt.Errorf("image path and tag cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		imgBucket := tx.Bucket([]byte(imagesToTagsBucket))
		tagBucket := tx.Bucket([]byte(tagsToImagesBucket))

		// 1. Update Image -> Tags mapping
		currentTagsBytes := imgBucket.Get([]byte(imagePath))
		currentTags, err := decodeList(currentTagsBytes)
		if err != nil {
			return fmt.Errorf("failed to decode tags for image %s: %w", imagePath, err)
		}

		updatedTags := removeFromList(currentTags, tag)
		// Only update if the list actually changed
		if len(updatedTags) != len(currentTags) {
			updatedTagsBytes, err := encodeList(updatedTags)
			if err != nil {
				return fmt.Errorf("failed to encode updated tags for image %s: %w", imagePath, err)
			}
			// If list is empty after removal, delete the key, otherwise update it
			if len(updatedTags) == 0 {
				if err := imgBucket.Delete([]byte(imagePath)); err != nil {
					return fmt.Errorf("failed to delete empty tag list for image %s: %w", imagePath, err)
				}
			} else {
				if err := imgBucket.Put([]byte(imagePath), updatedTagsBytes); err != nil {
					return fmt.Errorf("failed to put updated tags for image %s: %w", imagePath, err)
				}
			}
		}

		// 2. Update Tag -> Images mapping
		currentImagesBytes := tagBucket.Get([]byte(tag))
		currentImages, err := decodeList(currentImagesBytes)
		if err != nil {
			return fmt.Errorf("failed to decode images for tag %s: %w", tag, err)
		}

		updatedImages := removeFromList(currentImages, imagePath)
		// Only update if the list actually changed
		if len(updatedImages) != len(currentImages) {
			updatedImagesBytes, err := encodeList(updatedImages)
			if err != nil {
				return fmt.Errorf("failed to encode updated images for tag %s: %w", tag, err)
			}
			// If list is empty after removal, delete the key, otherwise update it
			if len(updatedImages) == 0 {
				if err := tagBucket.Delete([]byte(tag)); err != nil {
					return fmt.Errorf("failed to delete empty image list for tag %s: %w", tag, err)
				}
			} else {
				if err := tagBucket.Put([]byte(tag), updatedImagesBytes); err != nil {
					return fmt.Errorf("failed to put updated images for tag %s: %w", tag, err)
				}
			}
		}

		return nil
	})
}

// GetTags retrieves all tags associated with a given image path.
func (tdb *TagDB) GetTags(imagePath string) ([]string, error) {
	var tags []string
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesToTagsBucket))
		tagsBytes := bucket.Get([]byte(imagePath))
		if tagsBytes == nil {
			tags = []string{} // No tags found, return empty list
			return nil
		}
		var err error
		tags, err = decodeList(tagsBytes)
		if err != nil {
			return fmt.Errorf("failed to decode tags for image %s: %w", imagePath, err)
		}
		return nil
	})
	sort.Strings(tags) // Keep it tidy
	return tags, err
}

// GetImages retrieves all image paths associated with a given tag.
func (tdb *TagDB) GetImages(tag string) ([]string, error) {
	var images []string
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tagsToImagesBucket))
		imagesBytes := bucket.Get([]byte(tag))
		if imagesBytes == nil {
			images = []string{} // No images found, return empty list
			return nil
		}
		var err error
		images, err = decodeList(imagesBytes)
		if err != nil {
			return fmt.Errorf("failed to decode images for tag %s: %w", tag, err)
		}
		return nil
	})
	sort.Strings(images) // Keep it tidy
	return images, err
}

// GetAllTags retrieves a sorted list of all unique tags in the database.
func (tdb *TagDB) GetAllTags() ([]string, error) {
	var allTags []string
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tagsToImagesBucket))
		return bucket.ForEach(func(k, _ []byte) error { // the _ was v
			// k is the tag name
			allTags = append(allTags, string(k))
			return nil // continue iteration
		})
	})
	sort.Strings(allTags)
	return allTags, err
}
