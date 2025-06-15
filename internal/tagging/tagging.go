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
	ImagesToTagsBucket = "ImagesToTags" // Bucket name for image path to tags mapping.
	TagsToImagesBucket = "TagsToImages" // Bucket name for tag to image paths mapping.
)

// LoggerFunc defines a function signature for logging messages.
// This allows the ui package to provide its logging mechanism.
type LoggerFunc func(message string)

// TagDB manages the tagging database.
type TagDB struct {
	db     *bolt.DB
	logger LoggerFunc
}

// TagWithCount holds a tag name and the number of images associated with it.
type TagWithCount struct {
	Name  string
	Count int
}

// NewTagDB creates or opens the tag database file.
// dbDir specifies the directory where the db file should be stored.
// logger is a function that will be used for logging messages.
func NewTagDB(dbDir string, logger LoggerFunc) (*TagDB, error) {
	if dbDir == "" {
		// Default to user config directory or current directory if needed
		configDir, err := os.UserConfigDir()
		if err != nil {
			log.Printf("Warning: Could not get user config dir: %v. Using current dir.", err)
			dbDir = "." // Fallback to current directory
		} else {
			appName := "fyslide" // Consider making this a constant if used elsewhere
			// Attempt to get executable name for the app folder if possible, otherwise default.
			// For simplicity, we'll stick to a fixed name.
			appConfigDir := filepath.Join(configDir, appName) // App specific subfolder

			// Ensure the directory exists
			if err := os.MkdirAll(appConfigDir, 0750); err != nil {
				return nil, fmt.Errorf("failed to create config directory %s: %w", appConfigDir, err)
			}
			dbDir = appConfigDir
		}
	}

	dbPath := filepath.Join(dbDir, dbFileName)
	// Use the provided logger if available for this initial message
	if logger != nil {
		logger(fmt.Sprintf("Using tag database at: %s", dbPath))
	} else {
		log.Printf("Using tag database at: %s (logger not provided at init)", dbPath)
	}

	db, err := bolt.Open(dbPath, 0600, nil) // 0600 permissions: user read/write
	if err != nil {
		return nil, fmt.Errorf("failed to open tag database %s: %w", dbPath, err)
	}

	// Ensure buckets exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(ImagesToTagsBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", ImagesToTagsBucket, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(TagsToImagesBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", TagsToImagesBucket, err)
		}
		return nil
	})

	if err != nil {
		db.Close() // Close DB if bucket creation failed
		return nil, err
	}

	return &TagDB{db: db, logger: logger}, nil
}

// logMessage is a helper to use the configured logger or fallback to standard log.
func (tdb *TagDB) logMessage(format string, args ...interface{}) {
	if tdb.logger != nil {
		tdb.logger(fmt.Sprintf(format, args...))
	} else {
		log.Printf(format, args...) // Fallback if logger wasn't provided
	}
}

// Close closes the database connection.
func (tdb *TagDB) Close() error {
	if tdb.db != nil {
		return tdb.db.Close()
	}
	return nil
}

// --- Helper Functions ---

// encodeList marshals a list of strings into a JSON byte slice.
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

// removeFromList removes an item from a list. Returns the modified list.
func removeFromList(list []string, item string) []string {
	newList := list[:0] // Re-slice with 0 length but keep capacity
	for _, existing := range list {
		if existing != item {
			newList = append(newList, existing)
		}
	}
	return newList
}

// _updateStoredList manages adding or removing an item from a JSON-encoded string list stored in a BoltDB bucket.
// If 'add' is true, item is added. If 'add' is false, item is removed.
// If 'add' is false and the list becomes empty after removal, the key is deleted from the bucket.
// Returns true if the list was actually modified, false otherwise. An error is returned on failure.
func (tdb *TagDB) _updateStoredList(tx *bolt.Tx, bucketName []byte, key []byte, item string, add bool) (bool, error) {
	bucket := tx.Bucket(bucketName)
	if bucket == nil {
		// This should ideally not happen if NewTagDB ensures buckets exist.
		return false, fmt.Errorf("bucket %s not found in _updateStoredList", string(bucketName))
	}

	currentListBytes := bucket.Get(key)
	currentList, err := decodeList(currentListBytes)
	if err != nil {
		return false, fmt.Errorf("failed to decode list for key '%s' in bucket '%s': %w", string(key), string(bucketName), err)
	}

	var updatedList []string
	var changed bool

	if add {
		updatedList, changed = addToList(currentList, item)
	} else {
		originalLength := len(currentList)
		updatedList = removeFromList(currentList, item)
		changed = len(updatedList) != originalLength
	}

	if changed {
		if !add && len(updatedList) == 0 { // Removing and list became empty
			if err := bucket.Delete(key); err != nil {
				return true, fmt.Errorf("failed to delete empty list for key '%s' in bucket '%s': %w", string(key), string(bucketName), err)
			}
		} else {
			updatedListBytes, err := encodeList(updatedList)
			if err != nil {
				return true, fmt.Errorf("failed to encode updated list for key '%s' in bucket '%s': %w", string(key), string(bucketName), err)
			}
			if err := bucket.Put(key, updatedListBytes); err != nil {
				return true, fmt.Errorf("failed to put updated list for key '%s' in bucket '%s': %w", string(key), string(bucketName), err)
			}
		}
	}
	return changed, nil
}

// --- Core Tagging Functions ---

// AddTag associates a tag with an image path.
func (tdb *TagDB) AddTag(imagePath string, tag string) error {
	if imagePath == "" || tag == "" {
		return fmt.Errorf("image path and tag cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		// 1. Update Image -> Tags mapping
		_, err := tdb._updateStoredList(tx, []byte(ImagesToTagsBucket), []byte(imagePath), tag, true)
		if err != nil {
			return fmt.Errorf("updating image->tags for '%s' with tag '%s': %w", imagePath, tag, err)
		}

		// 2. Update Tag -> Images mapping
		_, err = tdb._updateStoredList(tx, []byte(TagsToImagesBucket), []byte(tag), imagePath, true)
		if err != nil {
			return fmt.Errorf("updating tag->images for '%s' with image '%s': %w", tag, imagePath, err)
		}
		return nil
	})
}

// AddTagsToImage associates multiple tags with a single image path within a single transaction.
func (tdb *TagDB) AddTagsToImage(imagePath string, tags []string) error {
	if imagePath == "" || len(tags) == 0 {
		return fmt.Errorf("image path and tags cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		for _, tag := range tags {
			if tag == "" { // Skip empty tags in the list
				continue
			}
			// 1. Update Image -> Tags mapping
			_, err := tdb._updateStoredList(tx, []byte(ImagesToTagsBucket), []byte(imagePath), tag, true)
			if err != nil {
				return fmt.Errorf("updating image->tags for '%s' with tag '%s': %w", imagePath, tag, err)
			}
			// 2. Update Tag -> Images mapping
			_, err = tdb._updateStoredList(tx, []byte(TagsToImagesBucket), []byte(tag), imagePath, true)
			if err != nil {
				return fmt.Errorf("updating tag->images for '%s' with image '%s': %w", tag, imagePath, err)
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
		// 1. Update Image -> Tags mapping
		_, err := tdb._updateStoredList(tx, []byte(ImagesToTagsBucket), []byte(imagePath), tag, false)
		if err != nil {
			return fmt.Errorf("updating image->tags for '%s' removing tag '%s': %w", imagePath, tag, err)
		}

		// 2. Update Tag -> Images mapping
		_, err = tdb._updateStoredList(tx, []byte(TagsToImagesBucket), []byte(tag), imagePath, false)
		if err != nil {
			return fmt.Errorf("updating tag->images for '%s' removing image '%s': %w", tag, imagePath, err)
		}
		return nil
	})
}

// GetTags retrieves all tags associated with a given image path.
func (tdb *TagDB) GetTags(imagePath string) ([]string, error) {
	var tags []string
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ImagesToTagsBucket))
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
		bucket := tx.Bucket([]byte(TagsToImagesBucket))
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

// GetAllTags retrieves a sorted list of all unique tags in the database,
// along with the count of images associated with each tag.
func (tdb *TagDB) GetAllTags() ([]TagWithCount, error) {
	var allTagsInfo []TagWithCount
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(TagsToImagesBucket))
		return bucket.ForEach(func(k, v []byte) error { // k is tag name, v is list of image paths
			tagName := string(k)
			imageList, err := decodeList(v)
			if err != nil {
				tdb.logMessage("Error decoding image list for tag '%s', skipping: %v", tagName, err)
				return nil // Continue to the next tag
			}
			count := len(imageList)
			allTagsInfo = append(allTagsInfo, TagWithCount{Name: tagName, Count: count})
			return nil // continue iteration
		})
	})
	if err != nil {
		return nil, err
	}

	// Sort the results by tag name
	sort.Slice(allTagsInfo, func(i, j int) bool {
		return allTagsInfo[i].Name < allTagsInfo[j].Name
	})
	return allTagsInfo, nil
}

// RemoveAllTagsForImage removes all tag associations for a given imagePath
// and cleans up the image's entry from the ImagesToTags bucket.
func (tdb *TagDB) RemoveAllTagsForImage(imagePath string) error {
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		imgBucket := tx.Bucket([]byte(ImagesToTagsBucket))

		// 1. Get all tags currently associated with the image
		currentTagsBytes := imgBucket.Get([]byte(imagePath))
		if currentTagsBytes == nil {
			// Image has no tags, nothing to do for its specific tags.
			// It might still be listed under some tags if data is inconsistent,
			// but RemoveTag below would handle that if called.
			// For a full cleanup, we'd iterate all tags and check, but that's less efficient.
			// This function assumes we primarily care about removing the image's own tag list
			// and its references from tags it knows it has.
			return nil
		}
		currentTags, err := decodeList(currentTagsBytes)
		if err != nil {
			return fmt.Errorf("failed to decode tags for image %s during cleanup: %w", imagePath, err)
		}

		// 2. For each tag, remove the imagePath from that tag's list of images
		for _, tag := range currentTags {
			// The _updateStoredList helper handles decoding, removing, encoding, and deleting the key if the list becomes empty.
			// It also handles the case where the tag might not exist or its list is already empty.
			// The 'changed' boolean return isn't strictly needed here but the error is.
			_, err := tdb._updateStoredList(tx, []byte(TagsToImagesBucket), []byte(tag), imagePath, false)
			if err != nil {
				// If one update fails, the transaction will be rolled back.
				return fmt.Errorf("failed to remove image '%s' from tag '%s' during cleanup: %w", imagePath, tag, err)
			}
		}

		// 3. Remove the imagePath key from the imagesToTagsBucket
		if err := imgBucket.Delete([]byte(imagePath)); err != nil {
			return fmt.Errorf("failed to delete image key %s from images bucket: %w", imagePath, err)
		}
		return nil
	})
}

// DeleteOrphanedTagKey directly removes a tag key from the TagsToImages bucket.
// This is intended for cleanup scenarios where a tag is known to be orphaned
// (i.e., its list of associated images is empty, as determined by the caller).
func (tdb *TagDB) DeleteOrphanedTagKey(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty for DeleteOrphanedTagKey")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		tagBucket := tx.Bucket([]byte(TagsToImagesBucket))
		if tagBucket == nil {
			// This should not happen if DB is initialized correctly
			return fmt.Errorf("bucket %s not found during DeleteOrphanedTagKey", TagsToImagesBucket)
		}
		// We trust that the caller has determined this tag is orphaned.
		// If the key doesn't exist, Delete does nothing and returns nil.
		if err := tagBucket.Delete([]byte(tag)); err != nil {
			return fmt.Errorf("failed to delete orphaned tag key '%s' from %s bucket: %w", tag, TagsToImagesBucket, err)
		}
		return nil
	})
}

// GetAllImagePaths retrieves all image paths stored in the ImagesToTagsBucket.
func (tdb *TagDB) GetAllImagePaths() ([]string, error) {
	var paths []string
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ImagesToTagsBucket))
		if bucket == nil {
			// Bucket doesn't exist, which means no images are tagged.
			return nil // Not an error, just no paths.
		}
		return bucket.ForEach(func(k, v []byte) error {
			paths = append(paths, string(k))
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all image paths: %w", err)
	}
	return paths, nil
}
