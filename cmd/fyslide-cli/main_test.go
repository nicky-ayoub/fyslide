package main

import (
	"bytes"
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/tagging"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary database file for testing and returns its path
// and a cleanup function.
func setupTestDB(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	// NewTagDB will create the actual db file inside this directory
	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

// executeCommandC executes a cobra command and captures its output.
// It sets the arguments for the rootCmd and then executes it.
// Standard output and standard error are captured.
func executeCommandC(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	_, err = root.ExecuteC()
	return buf.String(), err
}

// testLogger is a no-op logger for tests.
func testLogger(msg string) {}

func TestAddAndListTags(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgFileDir := t.TempDir()
	tmpImg := filepath.Join(imgFileDir, "img1.jpg")
	require.NoError(t, os.WriteFile(tmpImg, []byte("dummy"), 0644))

	// Add a tag
	rootCmdAdd := newTestRootCmd()
	out, err := executeCommandC(rootCmdAdd, "add", tmpImg, "cat")
	assert.NoError(t, err, out)

	// List tags
	rootCmdList := newTestRootCmd()
	out, err = executeCommandC(rootCmdList, "list", tmpImg)
	assert.NoError(t, err, out)
	assert.Contains(t, out, "cat")
}

func TestRemoveTag(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgFileDir := t.TempDir()
	tmpImg := filepath.Join(imgFileDir, "img_remove.jpg")
	require.NoError(t, os.WriteFile(tmpImg, []byte("dummy-remove"), 0644))

	// Add a tag
	rootCmdAdd := newTestRootCmd()
	out, err := executeCommandC(rootCmdAdd, "add", tmpImg, "transient")
	assert.NoError(t, err, out)

	// Verify tag is present
	rootCmdList1 := newTestRootCmd()
	out, err = executeCommandC(rootCmdList1, "list", tmpImg)
	assert.NoError(t, err, out)
	assert.Contains(t, out, "transient")

	// Remove the tag
	rootCmdRemove := newTestRootCmd()
	out, err = executeCommandC(rootCmdRemove, "remove", tmpImg, "transient")
	assert.NoError(t, err, out)

	// Verify tag is gone
	rootCmdList2 := newTestRootCmd()
	out, err = executeCommandC(rootCmdList2, "list", tmpImg)
	assert.NoError(t, err, out)
	assert.NotContains(t, out, "transient")
	// Depending on output for no tags, might be empty string or a specific message
	assert.Equal(t, "\n", out, "Output should be empty or just a newline if no tags are present")
}

func TestBatchAddAndFindByTag(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imageContentDir := t.TempDir()
	img1 := filepath.Join(imageContentDir, "a.jpg")
	img2 := filepath.Join(imageContentDir, "b.png") // Different extension
	require.NoError(t, os.WriteFile(img1, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(img2, []byte("b"), 0644))

	// Batch add
	rootCmdBatchAdd := newTestRootCmd()
	out, err := executeCommandC(rootCmdBatchAdd, "batch-add", imageContentDir, "dog,pet")
	assert.NoError(t, err, out)

	// Find by tag "dog"
	rootCmdFindByTagDog := newTestRootCmd()
	out, err = executeCommandC(rootCmdFindByTagDog, "find-by-tag", "dog")
	assert.NoError(t, err, out)
	absImg1, _ := filepath.Abs(img1) // Service layer works with absolute paths
	absImg2, _ := filepath.Abs(img2)
	assert.Contains(t, out, absImg1)
	assert.Contains(t, out, absImg2)

	// Find by tag "pet"
	rootCmdFindByTagPet := newTestRootCmd()
	out, err = executeCommandC(rootCmdFindByTagPet, "find-by-tag", "pet")
	assert.NoError(t, err, out)
	assert.Contains(t, out, absImg1)
	assert.Contains(t, out, absImg2)
}

func TestReplaceTag(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgFileDir := t.TempDir()
	tmpImg := filepath.Join(imgFileDir, "img_replace.gif")
	require.NoError(t, os.WriteFile(tmpImg, []byte("dummy-replace"), 0644))

	// Add initial tag
	rootCmdAdd := newTestRootCmd()
	_, err := executeCommandC(rootCmdAdd, "add", tmpImg, "oldtag")
	assert.NoError(t, err)

	// Replace tag
	rootCmdReplace := newTestRootCmd()
	_, err = executeCommandC(rootCmdReplace, "replace-tag", "oldtag", "newtag")
	assert.NoError(t, err)

	// List tags to verify
	rootCmdList := newTestRootCmd()
	out, err := executeCommandC(rootCmdList, "list", tmpImg)
	assert.NoError(t, err, out)
	assert.Contains(t, out, "newtag")
	assert.NotContains(t, out, "oldtag")
}

func TestListAllTags(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgDir := t.TempDir()
	imgA := filepath.Join(imgDir, "imgA.jpg")
	imgB := filepath.Join(imgDir, "imgB.jpg")
	require.NoError(t, os.WriteFile(imgA, []byte("A"), 0644))
	require.NoError(t, os.WriteFile(imgB, []byte("B"), 0644))

	// Add tags
	cmdAdd1 := newTestRootCmd()
	_, err := executeCommandC(cmdAdd1, "add", imgA, "alpha")
	require.NoError(t, err)
	cmdAdd2 := newTestRootCmd()
	_, err = executeCommandC(cmdAdd2, "add", imgA, "shared")
	require.NoError(t, err)
	cmdAdd3 := newTestRootCmd()
	_, err = executeCommandC(cmdAdd3, "add", imgB, "beta")
	require.NoError(t, err)
	cmdAdd4 := newTestRootCmd()
	_, err = executeCommandC(cmdAdd4, "add", imgB, "shared")
	require.NoError(t, err)

	// List all tags
	cmdListAll := newTestRootCmd()
	out, err := executeCommandC(cmdListAll, "list-all-tags")
	assert.NoError(t, err)

	// Normalize output for consistent checking (order might vary)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expectedTagsWithCounts := map[string]string{
		"alpha":  "alpha (1)",
		"beta":   "beta (1)",
		"shared": "shared (2)",
	}

	assert.Len(t, lines, len(expectedTagsWithCounts), "Number of tags listed should match")
	for _, line := range lines {
		found := false
		for _, expectedLine := range expectedTagsWithCounts {
			if line == expectedLine {
				found = true
				break
			}
		}
		assert.True(t, found, fmt.Sprintf("Unexpected line in list-all-tags output: %s", line))
	}
}

func TestNormalizeTags(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgDir := t.TempDir()
	imgA := filepath.Join(imgDir, "imgNorm.jpg")
	require.NoError(t, os.WriteFile(imgA, []byte("Norm"), 0644))

	// Add a tag with uppercase letters
	cmdAdd := newTestRootCmd()
	_, err := executeCommandC(cmdAdd, "add", imgA, "UpperCaseTag")
	require.NoError(t, err)

	// Normalize all tags
	cmdNormalize := newTestRootCmd()
	_, err = executeCommandC(cmdNormalize, "normalize")
	assert.NoError(t, err)

	// List tags for the image to check if it's lowercased
	cmdList := newTestRootCmd()
	out, err := executeCommandC(cmdList, "list", imgA)
	assert.NoError(t, err)
	assert.Contains(t, out, "uppercasetag")
	assert.NotContains(t, out, "UpperCaseTag")

	// List all tags to ensure the old one is gone and new one is present
	cmdListAll := newTestRootCmd()
	outAll, err := executeCommandC(cmdListAll, "list-all-tags")
	assert.NoError(t, err)
	assert.Contains(t, outAll, "uppercasetag (1)")
	assert.NotContains(t, outAll, "UpperCaseTag")
}

func TestBatchRemoveTags(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imageContentDir := t.TempDir()
	img1 := filepath.Join(imageContentDir, "br1.jpg")
	img2 := filepath.Join(imageContentDir, "br2.jpg")
	require.NoError(t, os.WriteFile(img1, []byte("br1"), 0644))
	require.NoError(t, os.WriteFile(img2, []byte("br2"), 0644))

	// Batch add tags
	cmdBatchAdd := newTestRootCmd()
	_, err := executeCommandC(cmdBatchAdd, "batch-add", imageContentDir, "tagA,tagB,tagC")
	require.NoError(t, err)

	// Batch remove "tagA" and "tagC"
	cmdBatchRemove := newTestRootCmd()
	_, err = executeCommandC(cmdBatchRemove, "batch-remove", imageContentDir, "tagA,tagC")
	assert.NoError(t, err)

	// Check tags for img1
	cmdList1 := newTestRootCmd()
	out1, err := executeCommandC(cmdList1, "list", img1)
	assert.NoError(t, err)
	assert.Contains(t, out1, "tagb")
	assert.NotContains(t, out1, "taga")
	assert.NotContains(t, out1, "tagc")

	// Check tags for img2
	cmdList2 := newTestRootCmd()
	out2, err := executeCommandC(cmdList2, "list", img2)
	assert.NoError(t, err)
	assert.Contains(t, out2, "tagb")
	assert.NotContains(t, out2, "taga")
	assert.NotContains(t, out2, "tagc")
}

func TestCleanDatabase(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgDir := t.TempDir()
	existingImg := filepath.Join(imgDir, "exists.jpg")
	missingImgPath := filepath.Join(imgDir, "missing.jpg") // This file won't be created
	require.NoError(t, os.WriteFile(existingImg, []byte("exists"), 0644))

	// Add tags to both, one will become orphaned regarding the file
	cmdAdd1 := newTestRootCmd()
	_, err := executeCommandC(cmdAdd1, "add", existingImg, "kepttag")
	require.NoError(t, err)
	cmdAdd2 := newTestRootCmd()
	_, err = executeCommandC(cmdAdd2, "add", missingImgPath, "orphanedfiletag") // Tag a non-existent file path
	require.NoError(t, err)
	cmdAdd3 := newTestRootCmd() // Add a tag that will become orphaned (no images)
	_, err = executeCommandC(cmdAdd3, "add", existingImg, "temp-orphan")
	require.NoError(t, err)
	cmdRemoveImgFromTag := newTestRootCmd() // Make 'temp-orphan' an orphaned tag
	_, err = executeCommandC(cmdRemoveImgFromTag, "remove", existingImg, "temp-orphan")
	require.NoError(t, err)

	// Clean database
	cmdClean := newTestRootCmd()
	_, err = executeCommandC(cmdClean, "clean")
	assert.NoError(t, err)

	// Verify state
	cmdListAll := newTestRootCmd()
	out, err := executeCommandC(cmdListAll, "list-all-tags")
	assert.NoError(t, err)
	assert.Contains(t, out, "kepttag (1)")
	assert.NotContains(t, out, "orphanedfiletag") // Should be removed as file is missing
	assert.NotContains(t, out, "temp-orphan")     // Should be removed as it has no images

	// Verify the missing image path is no longer in the DB (indirectly, by checking its tag is gone)
	// A more direct check would involve querying the DB, but list-all-tags is a good proxy.
}

func TestAddToTagged(t *testing.T) {
	finalTestDir, cleanup := setupTestDB(t)
	defer cleanup()

	newTestRootCmd := func() *cobra.Command {
		return NewRootCmd(func(cliDbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
			tdb, err := tagging.NewTagDB(finalTestDir, logger)
			if err != nil {
				return nil, nil, err
			}
			s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
			return s, tdb, nil
		})
	}

	imgDir := t.TempDir()
	imgX := filepath.Join(imgDir, "imgX.jpg")
	imgY := filepath.Join(imgDir, "imgY.jpg") // This one won't have 'baseTag'
	require.NoError(t, os.WriteFile(imgX, []byte("X"), 0644))
	require.NoError(t, os.WriteFile(imgY, []byte("Y"), 0644))

	// Add 'baseTag' to imgX
	cmdAddBase := newTestRootCmd()
	_, err := executeCommandC(cmdAddBase, "add", imgX, "baseTag")
	require.NoError(t, err)

	// Add 'otherTag' to imgY
	cmdAddOther := newTestRootCmd()
	_, err = executeCommandC(cmdAddOther, "add", imgY, "otherTag")
	require.NoError(t, err)

	// Add 'newTag1,newTag2' to all images tagged with 'baseTag'
	cmdAddToTagged := newTestRootCmd()
	_, err = executeCommandC(cmdAddToTagged, "add-to-tagged", "baseTag", "newTag1,newTag2")
	assert.NoError(t, err)

	// Check tags for imgX
	cmdListX := newTestRootCmd()
	outX, err := executeCommandC(cmdListX, "list", imgX)
	assert.NoError(t, err)
	assert.Contains(t, outX, "basetag")
	assert.Contains(t, outX, "newtag1")
	assert.Contains(t, outX, "newtag2")

	// Check tags for imgY (should not have newTag1, newTag2)
	cmdListY := newTestRootCmd()
	outY, err := executeCommandC(cmdListY, "list", imgY)
	assert.NoError(t, err)
	assert.Contains(t, outY, "othertag")
	assert.NotContains(t, outY, "newtag1")
	assert.NotContains(t, outY, "newtag2")
}
