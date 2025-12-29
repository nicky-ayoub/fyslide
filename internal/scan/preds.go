package scan

import (
	"path/filepath"
	"strings"
)

// Predicate defines a function signature for file path predicates.
type Predicate func(s string) bool

// IsImage returns truw on image extensions
func IsImage(fileName string) bool {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}

// IsAny always returns true
func IsAny(_ string) bool {
	return true
}
