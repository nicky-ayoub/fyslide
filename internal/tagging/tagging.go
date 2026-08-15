// Package tagging provides functionality for managing image tags using a BoltDB database.
// It allows adding, removing, and retrieving tags associated with image paths.
// It also provides a way to retrieve all unique tags in the database.
package tagging // Or place within your ui package if preferred

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFileName            = "fyslide_tags.db"
	imagesToTagsBucket    = "ImagesToTags" // Bucket name for image path to tags mapping.
	tagsToImagesBucket    = "TagsToImages" // Bucket name for tag to image paths mapping.
	fingerprintsBucket    = "DuplicateFingerprints"
	duplicateGroupsBucket = "DuplicateGroups"
)

// tagSplitter is a regex to split input strings by common delimiters.
var tagSplitter = regexp.MustCompile(`[,.':;+]`)

// LoggerFunc defines a function signature for logging messages.
// This allows the ui package to provide its logging mechanism.
type LoggerFunc func(message string)

// TagDB manages the tagging database.
type TagDB struct {
	db     *bolt.DB
	logger LoggerFunc
}

var (
	globalTagDBMu sync.Mutex
	globalTagDB   *TagDB
)

// TagWithCount holds a tag name and the number of images associated with it.
type TagWithCount struct {
	Name  string
	Count int
}

// Fingerprint stores a compact signature for an image used for duplicate detection.
type Fingerprint struct {
	Path           string
	FileHash       string
	PerceptualHash string
	Width          int
	Height         int
	Size           int64
	ModTime        time.Time
	UpdatedAt      time.Time
}

// DuplicateGroup stores a set of images that were identified as duplicates.
type DuplicateGroup struct {
	RepresentativePath string
	Members            []string
	MatchType          string
	Confidence         float64
	UpdatedAt          time.Time
}

// NewTagDB creates or opens the tag database file.
// dbDir specifies the directory where the db file should be stored.
// logger is a function that will be used for logging messages.
func NewTagDB(dbDir string, logger LoggerFunc) (*TagDB, error) {
	// Reuse a single long-lived DB instance per process to avoid repeated opens.
	globalTagDBMu.Lock()
	if globalTagDB != nil {
		// update logger if provided
		if logger != nil {
			globalTagDB.logger = logger
		}
		defer globalTagDBMu.Unlock()
		return globalTagDB, nil
	}
	globalTagDBMu.Unlock()

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

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second}) // 0600 permissions: user read/write
	if err != nil {
		return nil, fmt.Errorf("failed to open tag database %s (possible lock or another process using it): %w", dbPath, err)
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
		_, err = tx.CreateBucketIfNotExists([]byte(fingerprintsBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", fingerprintsBucket, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(duplicateGroupsBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", duplicateGroupsBucket, err)
		}
		return nil
	})

	if err != nil {
		db.Close() // Close DB if bucket creation failed
		return nil, err
	}

	tdb := &TagDB{db: db, logger: logger}
	globalTagDBMu.Lock()
	// store singleton reference
	globalTagDB = tdb
	globalTagDBMu.Unlock()
	return tdb, nil
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
	if tdb.db == nil {
		return nil
	}
	err := tdb.db.Close()
	// clear global reference if this is the global DB
	globalTagDBMu.Lock()
	if globalTagDB == tdb {
		globalTagDB = nil
	}
	globalTagDBMu.Unlock()
	return err
}

// --- Helper Functions ---

// ParseTagInput parses a raw input string into a slice of normalized tags.
// It splits by common delimiters (comma, dot, quotes, semicolon, plus),
// trims whitespace, removes surrounding quotes, lowercases, and removes duplicates.

// NormalizeTags takes a raw input string and returns a slice of normalized tags.
func NormalizeTags(rawInput string) []string {
	// The splitter regex is kept as is, assuming multi-word tags are desired
	// and are separated by punctuation, not spaces.
	potentialTags := tagSplitter.Split(rawInput, -1)

	// Pre-allocating slice capacity can improve performance by reducing reallocations.
	normalized := make([]string, 0, len(potentialTags))
	unique := make(map[string]bool)

	for _, pt := range potentialTags {
		// 1. Trim whitespace from the raw split part.
		tag := strings.TrimSpace(pt)
		// 2. Remove any surrounding single or double quotes.
		tag = strings.Trim(tag, `"'`)
		// 3. Trim whitespace again, in case there was space between quotes and the tag.
		tag = strings.TrimSpace(tag)
		// 4. Convert to lowercase for consistency.
		tag = strings.ToLower(tag)

		if tag != "" && !unique[tag] {
			normalized = append(normalized, tag)
			unique[tag] = true
		}
	}
	return normalized
}

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

func encodeValue(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

func decodeValue(data []byte, dest interface{}) error {
	if data == nil {
		return nil
	}
	return json.Unmarshal(data, dest)
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

// _updateTagsForImageList is a private helper to add or remove a list of tags for a list of images.
func (tdb *TagDB) _updateTagsForImageList(tx *bolt.Tx, imagePaths []string, tags []string, add bool) error {
	for _, imagePath := range imagePaths {
		for _, tag := range tags {
			if tag == "" {
				continue
			}
			// Update Image -> Tags mapping
			if _, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(imagePath), tag, add); err != nil {
				return err
			}
			// Update Tag -> Images mapping
			if _, err := tdb._updateStoredList(tx, []byte(tagsToImagesBucket), []byte(tag), imagePath, add); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddTagsToImageList adds a list of tags to a list of images in a single transaction.
func (tdb *TagDB) AddTagsToImageList(imagePaths []string, tags []string) error {
	if len(imagePaths) == 0 || len(tags) == 0 {
		return nil // Nothing to do
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		return tdb._updateTagsForImageList(tx, imagePaths, tags, true)
	})
}

// SaveFingerprint stores or updates a duplicate fingerprint for an image path.
func (tdb *TagDB) SaveFingerprint(fp Fingerprint) error {
	if fp.Path == "" {
		return errors.New("fingerprint path cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(fingerprintsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", fingerprintsBucket)
		}
		data, err := encodeValue(fp)
		if err != nil {
			return fmt.Errorf("encoding fingerprint: %w", err)
		}
		return bucket.Put([]byte(fp.Path), data)
	})
}

// GetFingerprint retrieves a stored fingerprint for an image path.
func (tdb *TagDB) GetFingerprint(path string) (*Fingerprint, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}
	var fp Fingerprint
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(fingerprintsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", fingerprintsBucket)
		}
		data := bucket.Get([]byte(path))
		if data == nil {
			return os.ErrNotExist
		}
		if err := decodeValue(data, &fp); err != nil {
			return fmt.Errorf("decoding fingerprint: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &fp, nil
}

// DeleteFingerprint removes a stored fingerprint for an image path.
func (tdb *TagDB) DeleteFingerprint(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(fingerprintsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", fingerprintsBucket)
		}
		return bucket.Delete([]byte(path))
	})
}

// SaveDuplicateGroup stores or updates a duplicate group.
func (tdb *TagDB) SaveDuplicateGroup(group DuplicateGroup) error {
	if group.RepresentativePath == "" {
		return errors.New("duplicate group representative path cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(duplicateGroupsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", duplicateGroupsBucket)
		}
		data, err := encodeValue(group)
		if err != nil {
			return fmt.Errorf("encoding duplicate group: %w", err)
		}
		return bucket.Put([]byte(group.RepresentativePath), data)
	})
}

// GetDuplicateGroups retrieves all stored duplicate groups.
func (tdb *TagDB) GetDuplicateGroups() ([]DuplicateGroup, error) {
	var groups []DuplicateGroup
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(duplicateGroupsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", duplicateGroupsBucket)
		}
		return bucket.ForEach(func(k, v []byte) error {
			var group DuplicateGroup
			if err := decodeValue(v, &group); err != nil {
				return fmt.Errorf("decoding duplicate group %s: %w", string(k), err)
			}
			groups = append(groups, group)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// --- Core Tagging Functions ---

// AddTag associates a tag with an image path.
func (tdb *TagDB) AddTag(imagePath string, tag string) error {
	if imagePath == "" || tag == "" {
		return fmt.Errorf("image path and tag cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		// 1. Update Image -> Tags mapping
		_, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(imagePath), tag, true)
		if err != nil {
			return fmt.Errorf("updating image->tags for '%s' with tag '%s': %w", imagePath, tag, err)
		}

		// 2. Update Tag -> Images mapping
		_, err = tdb._updateStoredList(tx, []byte(tagsToImagesBucket), []byte(tag), imagePath, true)
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
			_, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(imagePath), tag, true)
			if err != nil {
				return fmt.Errorf("updating image->tags for '%s' with tag '%s': %w", imagePath, tag, err)
			}
			// 2. Update Tag -> Images mapping
			_, err = tdb._updateStoredList(tx, []byte(tagsToImagesBucket), []byte(tag), imagePath, true)
			if err != nil {
				return fmt.Errorf("updating tag->images for '%s' with image '%s': %w", tag, imagePath, err)
			}
		}
		return nil
	})
}

// RemoveTagsFromImage disassociates multiple tags from a single image path within a single transaction.
func (tdb *TagDB) RemoveTagsFromImage(imagePath string, tags []string) error {
	if imagePath == "" || len(tags) == 0 {
		return fmt.Errorf("image path and tags cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		for _, tag := range tags {
			if tag == "" {
				continue
			}
			// 1. Update Image -> Tags mapping
			if _, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(imagePath), tag, false); err != nil {
				return fmt.Errorf("updating image->tags for '%s' removing tag '%s': %w", imagePath, tag, err)
			}
			// 2. Update Tag -> Images mapping
			if _, err := tdb._updateStoredList(tx, []byte(tagsToImagesBucket), []byte(tag), imagePath, false); err != nil {
				return fmt.Errorf("updating tag->images for '%s' removing image '%s': %w", tag, imagePath, err)
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
		_, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(imagePath), tag, false)
		if err != nil {
			return fmt.Errorf("updating image->tags for '%s' removing tag '%s': %w", imagePath, tag, err)
		}

		// 2. Update Tag -> Images mapping
		_, err = tdb._updateStoredList(tx, []byte(tagsToImagesBucket), []byte(tag), imagePath, false)
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

// GetAllTags retrieves a sorted list of all unique tags in the database,
// along with the count of images associated with each tag.
func (tdb *TagDB) GetAllTags() ([]TagWithCount, error) {
	var allTagsInfo []TagWithCount
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tagsToImagesBucket))
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

// ReplaceTag atomically replaces all occurrences of an old tag with a new tag.
func (tdb *TagDB) ReplaceTag(oldTag, newTag string) error {
	if oldTag == "" || newTag == "" {
		return errors.New("old and new tags must not be empty")
	}
	if oldTag == newTag {
		return nil // Nothing to do
	}

	return tdb.db.Update(func(tx *bolt.Tx) error {
		tagsBucket := tx.Bucket([]byte(tagsToImagesBucket))
		//imgBucket := tx.Bucket([]byte(imagesToTagsBucket))

		// 1. Get all images for the old tag.
		oldTagImagesBytes := tagsBucket.Get([]byte(oldTag))
		if oldTagImagesBytes == nil {
			return nil // Old tag does not exist.
		}
		imagePaths, err := decodeList(oldTagImagesBytes)
		if err != nil {
			return fmt.Errorf("failed to decode image list for old tag '%s': %w", oldTag, err)
		}

		// 2. Get the current list of images for the new tag.
		newTagImagesBytes := tagsBucket.Get([]byte(newTag))
		newTagImages, err := decodeList(newTagImagesBytes)
		if err != nil {
			return fmt.Errorf("failed to decode image list for new tag '%s': %w", newTag, err)
		}

		// 3. For each image, update its tag list and add it to the new tag's image list.
		for _, path := range imagePaths {
			// a. Update the image's own tag list: remove old, add new.
			if _, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(path), oldTag, false); err != nil {
				return err // Error will rollback transaction
			}
			if _, err := tdb._updateStoredList(tx, []byte(imagesToTagsBucket), []byte(path), newTag, true); err != nil {
				return err
			}

			// b. Add the image path to the new tag's list of images.
			newTagImages, _ = addToList(newTagImages, path)
		}

		// 4. Save the updated list for the new tag.
		updatedNewTagImagesBytes, err := encodeList(newTagImages)
		if err != nil {
			return fmt.Errorf("failed to encode image list for new tag '%s': %w", newTag, err)
		}
		if err := tagsBucket.Put([]byte(newTag), updatedNewTagImagesBytes); err != nil {
			return fmt.Errorf("failed to put image list for new tag '%s': %w", newTag, err)
		}

		// 5. Delete the old tag key.
		return tagsBucket.Delete([]byte(oldTag))
	})
}

// RemoveAllTagsForImage removes all tag associations for a given imagePath
// and cleans up the image's entry from the ImagesToTags bucket.
func (tdb *TagDB) RemoveAllTagsForImage(imagePath string) error {
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		imgBucket := tx.Bucket([]byte(imagesToTagsBucket))

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
			_, err := tdb._updateStoredList(tx, []byte(tagsToImagesBucket), []byte(tag), imagePath, false)
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
		tagBucket := tx.Bucket([]byte(tagsToImagesBucket))
		if tagBucket == nil {
			// This should not happen if DB is initialized correctly
			return fmt.Errorf("bucket %s not found during DeleteOrphanedTagKey", tagsToImagesBucket)
		}
		// We trust that the caller has determined this tag is orphaned.
		// If the key doesn't exist, Delete does nothing and returns nil.
		if err := tagBucket.Delete([]byte(tag)); err != nil {
			return fmt.Errorf("failed to delete orphaned tag key '%s' from %s bucket: %w", tag, tagsToImagesBucket, err)
		}
		return nil
	})
}

// GetAllImagePaths retrieves all image paths stored in the imagesToTagsBucket.
func (tdb *TagDB) GetAllImagePaths() ([]string, error) {
	var paths []string
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesToTagsBucket))
		if bucket == nil {
			// Bucket doesn't exist, which means no images are tagged.
			return nil // Not an error, just no paths.
		}
		return bucket.ForEach(func(k, _ []byte) error {
			paths = append(paths, string(k))
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all image paths: %w", err)
	}
	return paths, nil
}

// StreamAllImagePaths iterates over all image paths in the database and sends them to a channel.
// It closes the channel when done. This is more memory-efficient than GetAllImagePaths for large datasets.
func (tdb *TagDB) StreamAllImagePaths(pathChan chan<- string) {
	defer close(pathChan)
	err := tdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesToTagsBucket))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, _ []byte) error {
			// As we are in a View transaction, we must copy the key `k`
			// because it's only valid for the life of the transaction.
			pathCopy := make([]byte, len(k))
			copy(pathCopy, k)

			// A blocking send is fine here as the transaction is read-only and short-lived per item.
			pathChan <- string(pathCopy)
			return nil
		})
	})
	if err != nil {
		tdb.logMessage("Error streaming image paths from DB: %v", err)
	}
}

// RemoveTagsFromImageList removes a list of tags from a list of images in a single transaction.
func (tdb *TagDB) RemoveTagsFromImageList(imagePaths []string, tags []string) error {
	if len(imagePaths) == 0 || len(tags) == 0 {
		return nil // Nothing to do
	}
	return tdb.db.Update(func(tx *bolt.Tx) error {
		return tdb._updateTagsForImageList(tx, imagePaths, tags, false)
	})
}
