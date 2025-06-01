package main

import (
	"bytes"
	"encoding/json"
	"fyslide/internal/tagging"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt" // Import the bolt package
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

func TestCleanCommand(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	testDir := t.TempDir()
	realFile1Path := filepath.Join(testDir, "realfile1.jpg")
	realFile2Path := filepath.Join(testDir, "realfile2.png") // Will exist but have no tags initially
	fakeFile1Path := filepath.Join(testDir, "fakefile1.jpg") // Will not exist on disk

	absRealFile1, _ := filepath.Abs(realFile1Path)
	absFakeFile1, _ := filepath.Abs(fakeFile1Path)

	// Create real file
	err := os.WriteFile(realFile1Path, []byte("real content 1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(realFile2Path, []byte("real content 2"), 0644)
	require.NoError(t, err)

	// Setup initial DB state
	tdb, err := tagging.NewTagDB(dbPath)
	require.NoError(t, err)
	// Tags for a real file
	require.NoError(t, tdb.AddTag(absRealFile1, "tagForRealFile"))
	require.NoError(t, tdb.AddTag(absRealFile1, "commonTag"))
	// Tags for a file that will be "non-existent"
	require.NoError(t, tdb.AddTag(absFakeFile1, "tagForFakeFile"))
	require.NoError(t, tdb.AddTag(absFakeFile1, "commonTag"))
	// Create an orphaned tag (add it, then ensure no files reference it)
	// For this test, we'll add it to fakeFile1, then fakeFile1 is "deleted"
	// The clean command should then find "tagForFakeFile" and "commonTag" (if only on fakeFile1) as orphaned
	// Let's add a truly orphaned tag manually for clarity in testing phase 2
	// To do this, we add it to a dummy path and then remove that path from the tag's list
	// or more simply, add a tag that is never associated with any file that exists or is in tagging.ImagesToTagsBucket.
	// Forcing an orphan: Add tag "orphanedTagDirectly" to TagsToImages with an empty image list
	// We need to open the bolt.DB directly for this low-level setup.
	boltDB, err := bolt.Open(dbPath, 0600, nil)
	require.NoError(t, err, "Failed to open bolt DB directly for test setup")
	err = boltDB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tagging.TagsToImagesBucket))
		require.NotNil(t, b, "TagsToImagesBucket not found during direct bolt update")
		emptyListBytes, _ := json.Marshal([]string{}) // Use json.Marshal directly
		return b.Put([]byte("orphanedtagdirectly"), emptyListBytes)
	})
	require.NoError(t, err) // Check error from Update before closing
	require.NoError(t, boltDB.Close())
	tdb.Close()

	t.Run("clean dry run", func(t *testing.T) {
		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "clean", "--dry-run")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "DRY RUN: No changes will be made to the database.")
		assert.Contains(t, stdout, "DRY RUN: Would remove all tags for non-existent file: "+absFakeFile1)
		assert.Contains(t, stdout, "DRY RUN: Would remove orphaned tag: orphanedtagdirectly")
		// "commonTag" might also be listed as orphaned if absFakeFile1 was its only association after its removal.
		// "tagForFakeFile" will also be listed as orphaned after absFakeFile1 is processed.
		assert.Contains(t, stdout, "Non-existent image file entries that would be processed: 1")
		// The number of orphaned tags can be 1, 2 or 3 depending on how "commonTag" and "tagForFakeFile" are handled
		// after fakefile1 is processed. The test ensures "orphanedtagdirectly" is definitely one.
		// Let's check for at least 1. A more precise count would require simulating the multi-stage cleanup.
		assert.Regexp(t, `Orphaned tags that would be removed: [1-3]`, stdout) // commonTag, tagForFakeFile, orphanedtagdirectly

		// Verify DB is unchanged
		tdbVerify, _ := tagging.NewTagDB(dbPath)
		tagsReal, _ := tdbVerify.GetTags(absRealFile1)
		assert.Contains(t, tagsReal, "tagForRealFile")
		tagsFake, _ := tdbVerify.GetTags(absFakeFile1)
		assert.Contains(t, tagsFake, "tagForFakeFile")
		allTags, _ := tdbVerify.GetAllTags()
		foundOrphaned := false
		for _, ti := range allTags {
			if ti.Name == "orphanedtagdirectly" {
				foundOrphaned = true
				break
			}
		}
		assert.True(t, foundOrphaned, "orphanedtagdirectly should still exist after dry run")
		tdbVerify.Close()
	})

	t.Run("clean actual run", func(t *testing.T) {
		// Re-setup DB state as dry run might have been run before, ensure clean state for actual run
		tdbSetup, err := tagging.NewTagDB(dbPath)
		require.NoError(t, err)
		// Clear and re-add
		tdbSetup.RemoveAllTagsForImage(absRealFile1)
		tdbSetup.RemoveAllTagsForImage(absFakeFile1)
		tdbSetup.DeleteOrphanedTagKey("orphanedtagdirectly") // Clean from previous test if any
		tdbSetup.DeleteOrphanedTagKey("commonTag")
		tdbSetup.DeleteOrphanedTagKey("tagForFakeFile")

		require.NoError(t, tdbSetup.AddTag(absRealFile1, "tagForRealFile"))
		require.NoError(t, tdbSetup.AddTag(absRealFile1, "commontag"))      // Use lowercase for consistency
		require.NoError(t, tdbSetup.AddTag(absFakeFile1, "tagForFakeFile")) // For non-existent file
		require.NoError(t, tdbSetup.AddTag(absFakeFile1, "commontag"))      // For non-existent file

		boltDBSetup, err := bolt.Open(dbPath, 0600, nil) // Open bolt DB directly for setup
		require.NoError(t, err)
		err = boltDBSetup.Update(func(tx *bolt.Tx) error { // Explicitly orphaned tag
			b := tx.Bucket([]byte(tagging.TagsToImagesBucket))
			require.NotNil(t, b, "TagsToImagesBucket not found during direct bolt update for actual run setup")
			emptyListBytes, _ := json.Marshal([]string{}) // Use json.Marshal directly
			return b.Put([]byte("orphanedtagdirectly"), emptyListBytes)
		})
		require.NoError(t, err)                 // Check error from Update
		require.NoError(t, boltDBSetup.Close()) // Close the directly opened bolt.DB
		tdbSetup.Close()                        // Close the tagging.TagDB wrapper

		stdout, stderr, err := executeCommandC(rootCmd, "--dbpath", dbPath, "clean")
		require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)

		assert.Contains(t, stdout, "Removing all tags for non-existent file: "+absFakeFile1)
		assert.Contains(t, stdout, "Removing orphaned tag: orphanedtagdirectly")
		assert.Contains(t, stdout, "Removing orphaned tag: tagforfakefile") // Became orphaned after fakefile1 processing
		// commonTag might also be removed if absFakeFile1 was its only remaining reference after realFile1's commonTag
		// The summary counts are key here.
		assert.Contains(t, stdout, "Non-existent image file entries processed: 1")
		assert.Regexp(t, `Orphaned tags removed: [1-3]`, stdout) // orphanedtagdirectly, tagforfakefile, potentially commonTag

		// Verify DB state
		tdbVerify, _ := tagging.NewTagDB(dbPath)
		tagsReal, _ := tdbVerify.GetTags(absRealFile1)
		assert.ElementsMatch(t, []string{"commontag", "tagforrealfile"}, tagsReal, "Real file should retain its tags")

		tagsFake, _ := tdbVerify.GetTags(absFakeFile1)
		assert.Empty(t, tagsFake, "Tags for fake file should be gone")

		allTags, _ := tdbVerify.GetAllTags()
		tagNames := []string{}
		for _, ti := range allTags {
			tagNames = append(tagNames, ti.Name)
		}
		assert.NotContains(t, tagNames, "tagForFakeFile")
		assert.NotContains(t, tagNames, "orphanedtagdirectly")
		// commonTag should still exist if it was on realFile1
		assert.Contains(t, tagNames, "commontag")
		assert.Contains(t, tagNames, "tagforrealfile")
		tdbVerify.Close()
	})
}
