package main

import (
	"bytes"
	"fyslide/internal/tagging"
	"io"
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
	t.Helper()
	tempDir := t.TempDir() // Go's testing package will auto-cleanup this directory
	dbPath := filepath.Join(tempDir, "test_tags.db")

	// Ensure the database file is created and schema initialized by NewTagDB.
	// This makes setupTestDB more active in ensuring a usable DB state.
	tdb, err := tagging.NewTagDB(dbPath)
	require.NoError(t, err, "setupTestDB: failed to initialize test database at %s", dbPath)
	require.NoError(t, tdb.Close(), "setupTestDB: failed to close test database after initialization")

	cleanup := func() {
		// os.Remove(dbPath) // Not strictly necessary due to t.TempDir()
	}
	return dbPath, cleanup
}

// executeCommandC executes a cobra command and captures its output.
// It sets the arguments for the rootCmd and then executes it.
// Standard output and standard error are captured.
func executeCommandC(root *cobra.Command, args ...string) (string, string, error) {
	// Reset global flags that might be sticky from manual runs or other tests,
	// though Cobra's flag parsing per Execute() should handle this.
	dryRunFlag = false
	forceFlag = false
	// dbPathFlag is set via args like "--dbpath"

	actualStdout := new(bytes.Buffer)
	actualStderr := new(bytes.Buffer)
	root.SetOut(actualStdout)
	root.SetErr(actualStderr)
	root.SetArgs(args)

	err := root.Execute()

	return actualStdout.String(), actualStderr.String(), err
}

func TestRootHelp(t *testing.T) {
	stdout, stderr, err := executeCommandC(rootCmd, "--help")
	require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "Usage:")
	assert.Contains(t, stdout, "fyslide-cli [command]")
}

func TestListAllTagsCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("no tags", func(t *testing.T) {
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "list-all-tags")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "No tags found in the database.")
	})

	t.Run("with tags", func(t *testing.T) {
		// Add some tags directly to the test DB for setup
		tdb, err := tagging.NewTagDB(dbPath)
		require.NoError(t, err)
		require.NoError(t, tdb.AddTag(filepath.Join(t.TempDir(), "file1.jpg"), "tagA"))
		require.NoError(t, tdb.AddTag(filepath.Join(t.TempDir(), "file2.png"), "tagB"))
		require.NoError(t, tdb.AddTag(filepath.Join(t.TempDir(), "file3.gif"), "tagA")) // Duplicate tag on different file
		tdb.Close()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "list-all-tags")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "All tags in database:")
		assert.Contains(t, stdout, "tagA")
		assert.Contains(t, stdout, "tagB")
		// Ensure order doesn't matter for assertion if GetAllTags sorts them (which it does)
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		assert.Contains(t, lines, "tagA")
		assert.Contains(t, lines, "tagB")
		assert.Len(t, lines, 3) // "All tags...", "tagA", "tagB"
	})
}

func TestAddCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	dummyFileDir := t.TempDir()
	dummyFilePath := filepath.Join(dummyFileDir, "test_image.jpg")
	err := os.WriteFile(dummyFilePath, []byte("dummy image content"), 0644)
	require.NoError(t, err)

	absDummyFilePath, err := filepath.Abs(dummyFilePath)
	require.NoError(t, err)

	t.Run("add single tag", func(t *testing.T) {
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "add", dummyFilePath, "newTag1")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Added tag 'newTag1' to "+absDummyFilePath)

		// Verify in DB
		tdb, _ := tagging.NewTagDB(dbPath)
		tags, _ := tdb.GetTags(absDummyFilePath)
		tdb.Close()
		assert.Contains(t, tags, "newTag1")
	})

	t.Run("add multiple tags", func(t *testing.T) {
		// Ensure the file is clean of these specific tags for this subtest
		tdbClean, _ := tagging.NewTagDB(dbPath)
		tdbClean.RemoveTag(absDummyFilePath, "multiTagA")
		tdbClean.RemoveTag(absDummyFilePath, "multiTagB")
		tdbClean.Close()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "add", dummyFilePath, "multiTagA", "multiTagB")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Added tag 'multiTagA' to "+absDummyFilePath)
		assert.Contains(t, stdout, "Added tag 'multiTagB' to "+absDummyFilePath)

		// Verify in DB
		tdb, _ := tagging.NewTagDB(dbPath)
		tags, _ := tdb.GetTags(absDummyFilePath)
		tdb.Close()
		assert.Contains(t, tags, "multiTagA")
		assert.Contains(t, tags, "multiTagB")
	})

	t.Run("add to non-existent file path", func(t *testing.T) {
		// AddTag in tagging.go doesn't check file existence, so CLI should succeed.
		nonExistentPath := filepath.Join(dummyFileDir, "non_existent.jpg")
		absNonExistentPath, _ := filepath.Abs(nonExistentPath)

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "add", nonExistentPath, "ghostTag")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Added tag 'ghostTag' to "+absNonExistentPath)

		tdb, _ := tagging.NewTagDB(dbPath)
		tags, _ := tdb.GetTags(absNonExistentPath)
		tdb.Close()
		assert.Contains(t, tags, "ghostTag")
	})
}

func TestRemoveCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	dummyFileDir := t.TempDir()
	dummyFilePath := filepath.Join(dummyFileDir, "remove_test.jpg")
	err := os.WriteFile(dummyFilePath, []byte("dummy content"), 0644)
	require.NoError(t, err)
	absDummyFilePath, _ := filepath.Abs(dummyFilePath)

	// Pre-populate for each sub-test to ensure isolation
	setupSubTest := func() {
		tdb, _ := tagging.NewTagDB(dbPath)
		// Clear all tags for this file first
		currentTags, _ := tdb.GetTags(absDummyFilePath)
		for _, tag := range currentTags {
			tdb.RemoveTag(absDummyFilePath, tag)
		}
		// Add specific tags for the test
		require.NoError(t, tdb.AddTag(absDummyFilePath, "tagToRemove1"))
		require.NoError(t, tdb.AddTag(absDummyFilePath, "tagToRemove2"))
		require.NoError(t, tdb.AddTag(absDummyFilePath, "tagToKeep"))
		tdb.Close()
	}

	t.Run("remove single tag", func(t *testing.T) {
		setupSubTest()
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "remove", dummyFilePath, "tagToRemove1")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Removed tag 'tagToRemove1' from "+absDummyFilePath)

		tdb, _ := tagging.NewTagDB(dbPath)
		tags, _ := tdb.GetTags(absDummyFilePath)
		tdb.Close()
		assert.NotContains(t, tags, "tagToRemove1")
		assert.Contains(t, tags, "tagToRemove2")
		assert.Contains(t, tags, "tagToKeep")
	})

	t.Run("remove multiple tags", func(t *testing.T) {
		setupSubTest()
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "remove", dummyFilePath, "tagToRemove1", "tagToKeep")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Removed tag 'tagToRemove1' from "+absDummyFilePath)
		assert.Contains(t, stdout, "Removed tag 'tagToKeep' from "+absDummyFilePath)

		tdb, _ := tagging.NewTagDB(dbPath)
		tags, _ := tdb.GetTags(absDummyFilePath)
		tdb.Close()
		assert.NotContains(t, tags, "tagToRemove1")
		assert.NotContains(t, tags, "tagToKeep")
		assert.Contains(t, tags, "tagToRemove2") // This should be the only one left
	})

	t.Run("remove non-existent tag from file", func(t *testing.T) {
		setupSubTest() // Ensure file has some tags, but not "nonExistentTag"
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "remove", dummyFilePath, "nonExistentTag")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		// tagging.RemoveTag is idempotent, so CLI reports success.
		assert.Contains(t, stdout, "Removed tag 'nonExistentTag' from "+absDummyFilePath)

		tdb, _ := tagging.NewTagDB(dbPath)
		tags, _ := tdb.GetTags(absDummyFilePath)
		tdb.Close()
		assert.Contains(t, tags, "tagToRemove1") // Ensure other tags are still there
	})
}

func TestListCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	dummyFileDir := t.TempDir()
	dummyFilePath := filepath.Join(dummyFileDir, "list_test.jpg")
	err := os.WriteFile(dummyFilePath, []byte("dummy content"), 0644)
	require.NoError(t, err)
	absDummyFilePath, _ := filepath.Abs(dummyFilePath)

	t.Run("list no tags", func(t *testing.T) {
		// Ensure no tags for this file
		tdb, _ := tagging.NewTagDB(dbPath)
		currentTags, _ := tdb.GetTags(absDummyFilePath)
		for _, tag := range currentTags {
			tdb.RemoveTag(absDummyFilePath, tag)
		}
		tdb.Close()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "list", dummyFilePath)
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "No tags found for "+absDummyFilePath)
	})

	t.Run("list with tags", func(t *testing.T) {
		tdb, _ := tagging.NewTagDB(dbPath)
		require.NoError(t, tdb.AddTag(absDummyFilePath, "listTag1"))
		require.NoError(t, tdb.AddTag(absDummyFilePath, "listTag2"))
		tdb.Close()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "list", dummyFilePath)
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		// Order of tags in output is sorted by GetTags
		assert.Contains(t, stdout, "Tags for "+absDummyFilePath+": listTag1, listTag2")
	})
}

func TestFindByTagCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	dummyFileDir := t.TempDir()
	file1 := filepath.Join(dummyFileDir, "find_file1.jpg")
	file2 := filepath.Join(dummyFileDir, "find_file2.png")
	file3 := filepath.Join(dummyFileDir, "find_file3.gif") // Has a different tag
	os.WriteFile(file1, []byte("1"), 0644)
	os.WriteFile(file2, []byte("2"), 0644)
	os.WriteFile(file3, []byte("3"), 0644)

	absFile1, _ := filepath.Abs(file1)
	absFile2, _ := filepath.Abs(file2)
	absFile3, _ := filepath.Abs(file3)

	tdb, _ := tagging.NewTagDB(dbPath)
	require.NoError(t, tdb.AddTag(absFile1, "findThisTag"))
	require.NoError(t, tdb.AddTag(absFile2, "findThisTag"))
	require.NoError(t, tdb.AddTag(absFile3, "anotherTag"))
	tdb.Close()

	t.Run("find existing tag", func(t *testing.T) {
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "find-by-tag", "findThisTag")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Images with tag 'findThisTag':")
		// Order from GetImages is sorted by path
		if strings.Compare(absFile1, absFile2) < 0 {
			assert.Regexp(t, absFile1+"\n"+absFile2, stdout)
		} else {
			assert.Regexp(t, absFile2+"\n"+absFile1, stdout)
		}
		assert.NotContains(t, stdout, absFile3)
	})

	t.Run("find non-existent tag", func(t *testing.T) {
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "find-by-tag", "tagThatDoesNotExist")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "No images found with tag 'tagThatDoesNotExist'")
	})
}

func TestBatchAddCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	batchDir := t.TempDir()
	img1Path := filepath.Join(batchDir, "img1.jpg")
	img2Path := filepath.Join(batchDir, "img2.png")
	nonImgPath := filepath.Join(batchDir, "text.txt") // Should be ignored
	os.WriteFile(img1Path, []byte("img1"), 0644)
	os.WriteFile(img2Path, []byte("img2"), 0644)
	os.WriteFile(nonImgPath, []byte("text"), 0644)

	absImg1Path, _ := filepath.Abs(img1Path)
	absImg2Path, _ := filepath.Abs(img2Path)
	absBatchDir, _ := filepath.Abs(batchDir)

	t.Run("batch add tags", func(t *testing.T) {
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "batch-add", batchDir, "batchTag1", "batchTag2")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "Added tag 'batchTag1' to "+absImg1Path)
		assert.Contains(t, stdout, "Added tag 'batchTag2' to "+absImg1Path)
		assert.Contains(t, stdout, "Added tag 'batchTag1' to "+absImg2Path)
		assert.Contains(t, stdout, "Added tag 'batchTag2' to "+absImg2Path)
		assert.Contains(t, stdout, "Finished batch add. Processed 2 image files. Added 4 tag instances in "+absBatchDir)

		tdb, _ := tagging.NewTagDB(dbPath)
		tags1, _ := tdb.GetTags(absImg1Path)
		tags2, _ := tdb.GetTags(absImg2Path)
		tdb.Close()
		assert.ElementsMatch(t, []string{"batchTag1", "batchTag2"}, tags1)
		assert.ElementsMatch(t, []string{"batchTag1", "batchTag2"}, tags2)
	})

	t.Run("batch add tags dry run", func(t *testing.T) {
		// Clear previous tags for a clean dry-run test
		tdbClear, _ := tagging.NewTagDB(dbPath)
		tdbClear.RemoveTag(absImg1Path, "batchTag1")
		tdbClear.RemoveTag(absImg1Path, "batchTag2")
		tdbClear.RemoveTag(absImg2Path, "batchTag1")
		tdbClear.RemoveTag(absImg2Path, "batchTag2")
		tdbClear.Close()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "batch-add", "--dry-run", batchDir, "dryTag1")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "DRY RUN: Would add tag 'dryTag1' for "+absImg1Path)
		assert.Contains(t, stdout, "DRY RUN: Would add tag 'dryTag1' for "+absImg2Path)
		assert.Contains(t, stdout, "DRY RUN: Finished simulation of batch add. Processed 2 image files. Added 2 tag instances in "+absBatchDir)

		tdb, _ := tagging.NewTagDB(dbPath)
		tags1, _ := tdb.GetTags(absImg1Path)
		tags2, _ := tdb.GetTags(absImg2Path)
		tdb.Close()
		assert.Empty(t, tags1)
		assert.Empty(t, tags2)
	})
}

func TestBatchRemoveCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	batchDir := t.TempDir()
	img1Path := filepath.Join(batchDir, "r_img1.jpg")
	img2Path := filepath.Join(batchDir, "r_img2.png")
	os.WriteFile(img1Path, []byte("r_img1"), 0644)
	os.WriteFile(img2Path, []byte("r_img2"), 0644)

	absImg1Path, _ := filepath.Abs(img1Path)
	absImg2Path, _ := filepath.Abs(img2Path)
	absBatchDir, _ := filepath.Abs(batchDir)

	setupBatchRemoveSubTest := func() {
		tdb, _ := tagging.NewTagDB(dbPath)
		// Clear existing tags on these files
		tags1, _ := tdb.GetTags(absImg1Path)
		for _, tg := range tags1 {
			tdb.RemoveTag(absImg1Path, tg)
		}
		tags2, _ := tdb.GetTags(absImg2Path)
		for _, tg := range tags2 {
			tdb.RemoveTag(absImg2Path, tg)
		}
		// Add fresh tags
		require.NoError(t, tdb.AddTag(absImg1Path, "commonTag"))
		require.NoError(t, tdb.AddTag(absImg1Path, "uniqueTag1"))
		require.NoError(t, tdb.AddTag(absImg2Path, "commonTag"))
		require.NoError(t, tdb.AddTag(absImg2Path, "uniqueTag2"))
		tdb.Close()
	}

	t.Run("batch remove tags with force", func(t *testing.T) {
		setupBatchRemoveSubTest()
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "batch-remove", "--force", batchDir, "commonTag")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "Removed tag 'commonTag' for "+absImg1Path)
		assert.Contains(t, stdout, "Removed tag 'commonTag' for "+absImg2Path)
		assert.Contains(t, stdout, "Finished batch remove. Processed 2 image files. Removed 2 tag instances in "+absBatchDir)

		tdbVerify, _ := tagging.NewTagDB(dbPath)
		tags1, _ := tdbVerify.GetTags(absImg1Path)
		tags2, _ := tdbVerify.GetTags(absImg2Path)
		tdbVerify.Close()
		assert.NotContains(t, tags1, "commonTag")
		assert.Contains(t, tags1, "uniqueTag1")
		assert.NotContains(t, tags2, "commonTag")
		assert.Contains(t, tags2, "uniqueTag2")
	})

	t.Run("batch remove tags dry run", func(t *testing.T) {
		setupBatchRemoveSubTest() // Ensure tags are present for dry run to report
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "batch-remove", "--dry-run", batchDir, "commonTag", "uniqueTag1")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "DRY RUN: Would remove tag 'commonTag' for "+absImg1Path)
		assert.Contains(t, stdout, "DRY RUN: Would remove tag 'uniqueTag1' for "+absImg1Path)
		assert.Contains(t, stdout, "DRY RUN: Would remove tag 'commonTag' for "+absImg2Path)
		assert.Contains(t, stdout, "DRY RUN: Would remove tag 'uniqueTag1' for "+absImg2Path) // Will attempt, even if not present on img2
		assert.Contains(t, stdout, "DRY RUN: Finished simulation of batch remove. Processed 2 image files. Removed 4 tag instances in "+absBatchDir)

		tdbVerify, _ := tagging.NewTagDB(dbPath)
		tags1, _ := tdbVerify.GetTags(absImg1Path)
		tags2, _ := tdbVerify.GetTags(absImg2Path)
		tdbVerify.Close()
		assert.Contains(t, tags1, "commonTag")
		assert.Contains(t, tags1, "uniqueTag1")
		assert.Contains(t, tags2, "commonTag")
		assert.Contains(t, tags2, "uniqueTag2")
	})

	t.Run("batch remove with confirmation prompt - cancel", func(t *testing.T) {
		setupBatchRemoveSubTest()
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		defer func() { os.Stdin = oldStdin; w.Close(); r.Close() }()

		go func() {
			io.WriteString(w, "no\n")
			w.Close() // Close writer to signal EOF to Scanln if it reads past "no\n"
		}()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "batch-remove", batchDir, "commonTag")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, "Operation cancelled by user.")

		tdbVerify, _ := tagging.NewTagDB(dbPath)
		tags1, _ := tdbVerify.GetTags(absImg1Path)
		tags2, _ := tdbVerify.GetTags(absImg2Path)
		tdbVerify.Close()
		assert.Contains(t, tags1, "commonTag")
		assert.Contains(t, tags2, "commonTag")
	})

	t.Run("batch remove with confirmation prompt - confirm", func(t *testing.T) {
		setupBatchRemoveSubTest()
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		defer func() { os.Stdin = oldStdin; w.Close(); r.Close() }()

		go func() {
			io.WriteString(w, "yes\n")
			w.Close()
		}()

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "batch-remove", batchDir, "commonTag")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "Removed tag 'commonTag' for "+absImg1Path)
		assert.Contains(t, stdout, "Removed tag 'commonTag' for "+absImg2Path)

		tdbVerify, _ := tagging.NewTagDB(dbPath)
		tags1, _ := tdbVerify.GetTags(absImg1Path)
		tags2, _ := tdbVerify.GetTags(absImg2Path)
		tdbVerify.Close()
		assert.NotContains(t, tags1, "commonTag")
		assert.NotContains(t, tags2, "commonTag")
	})
}
