package tagging

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestDuplicateFingerprintPersistence(t *testing.T) {
	dbDir := t.TempDir()
	db, err := NewTagDB(dbDir, nil)
	if err != nil {
		t.Fatalf("NewTagDB() error = %v", err)
	}
	defer db.Close()

	fingerprint := Fingerprint{
		Path:           "/tmp/example.jpg",
		FileHash:       "abc123",
		PerceptualHash: "def456",
		Width:          1920,
		Height:         1080,
		Size:           2048,
		ModTime:        time.Unix(1700000000, 0).UTC(),
		UpdatedAt:      time.Unix(1700000100, 0).UTC(),
	}

	if err := db.SaveFingerprint(fingerprint); err != nil {
		t.Fatalf("SaveFingerprint() error = %v", err)
	}

	stored, err := db.GetFingerprint(fingerprint.Path)
	if err != nil {
		t.Fatalf("GetFingerprint() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetFingerprint() returned nil fingerprint")
	}
	if stored.Path != fingerprint.Path || stored.FileHash != fingerprint.FileHash || stored.PerceptualHash != fingerprint.PerceptualHash {
		t.Fatalf("stored fingerprint mismatch: got %+v want %+v", stored, fingerprint)
	}

	group := DuplicateGroup{
		RepresentativePath: fingerprint.Path,
		Members:            []string{fingerprint.Path},
		MatchType:          "exact",
		Confidence:         1.0,
		UpdatedAt:          time.Unix(1700000200, 0).UTC(),
	}

	if err := db.SaveDuplicateGroup(group); err != nil {
		t.Fatalf("SaveDuplicateGroup() error = %v", err)
	}

	storedGroups, err := db.GetDuplicateGroups()
	if err != nil {
		t.Fatalf("GetDuplicateGroups() error = %v", err)
	}
	if len(storedGroups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(storedGroups))
	}
	if storedGroups[0].RepresentativePath != group.RepresentativePath || storedGroups[0].MatchType != group.MatchType {
		t.Fatalf("stored duplicate group mismatch: got %+v want %+v", storedGroups[0], group)
	}

	if err := db.DeleteFingerprint(fingerprint.Path); err != nil {
		t.Fatalf("DeleteFingerprint() error = %v", err)
	}

	if _, err := db.GetFingerprint(fingerprint.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist after delete, got %v", err)
	}
}
