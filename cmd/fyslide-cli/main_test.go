package main

import (
	"bytes"
	"fyslide/internal/service"
	"fyslide/internal/tagging"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

var (
	testTagDB *tagging.TagDB
	testSvc   *service.Service
)

// setupTestDB creates a temporary database file for testing and returns its path
// and a cleanup function.
func setupTestDB(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	require.NoError(t, err)
	db.Close()
	return dbPath, func() { os.Remove(dbPath) }
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
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	// Set up TagDB and Service
	tagDB, err := tagging.NewTagDB(dbPath, testLogger)
	require.NoError(t, err)
	defer tagDB.Close()
	svc := service.NewService(tagDB, nil, testLogger)

	// Prepare CLI root command using the service
	rootCmd := buildTestRootCmd(svc, tagDB)

	// Create a dummy image file
	tmpImg := filepath.Join(t.TempDir(), "img1.jpg")
	require.NoError(t, os.WriteFile(tmpImg, []byte("dummy"), 0644))

	// Add a tag
	out, err := executeCommandC(rootCmd, "add", tmpImg, "cat")
	assert.NoError(t, err, out)

	// List tags
	out, err = executeCommandC(rootCmd, "list", tmpImg)
	assert.NoError(t, err, out)
	assert.Contains(t, out, "cat")
}

func TestBatchAddAndFindByTag(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	tagDB, err := tagging.NewTagDB(dbPath, testLogger)
	require.NoError(t, err)
	defer tagDB.Close()
	svc := service.NewService(tagDB, nil, testLogger)

	rootCmd := buildTestRootCmd(svc, tagDB)

	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "a.jpg")
	img2 := filepath.Join(tmpDir, "b.jpg")
	require.NoError(t, os.WriteFile(img1, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(img2, []byte("b"), 0644))

	// Batch add
	out, err := executeCommandC(rootCmd, "batch-add", tmpDir, "dog,pet")
	assert.NoError(t, err, out)

	// Find by tag
	out, err = executeCommandC(rootCmd, "find-by-tag", "dog")
	assert.NoError(t, err, out)
	assert.Contains(t, out, "a.jpg")
	assert.Contains(t, out, "b.jpg")
}

func TestReplaceTag(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	tagDB, err := tagging.NewTagDB(dbPath, testLogger)
	require.NoError(t, err)
	defer tagDB.Close()
	svc := service.NewService(tagDB, nil, testLogger)

	rootCmd := buildTestRootCmd(svc, tagDB)

	tmpImg := filepath.Join(t.TempDir(), "img2.jpg")
	require.NoError(t, os.WriteFile(tmpImg, []byte("dummy"), 0644))

	_, err = executeCommandC(rootCmd, "add", tmpImg, "oldtag")
	assert.NoError(t, err)

	_, err = executeCommandC(rootCmd, "replace-tag", "oldtag", "newtag")
	assert.NoError(t, err)

	out, err := executeCommandC(rootCmd, "list", tmpImg)
	assert.NoError(t, err)
	assert.Contains(t, out, "newtag")
	assert.NotContains(t, out, "oldtag")
}

// buildTestRootCmd builds a root cobra.Command for testing, using the provided service and tagDB.
func buildTestRootCmd(svc *service.Service, tagDB *tagging.TagDB) *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "fyslide-cli-test",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// no-op for tests
		},
	}

	// Add command
	addCmd := &cobra.Command{
		Use:   "add [image] [tag]",
		Short: "Add a tag to an image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imagePath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			return svc.AddTagsToImage(imagePath, []string{tag})
		},
	}
	rootCmd.AddCommand(addCmd)

	// Remove command
	removeCmd := &cobra.Command{
		Use:   "remove [image] [tag]",
		Short: "Remove a tag from an image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imagePath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			return svc.RemoveTagsFromImage(imagePath, []string{tag})
		},
	}
	rootCmd.AddCommand(removeCmd)

	// List tags for image
	listCmd := &cobra.Command{
		Use:   "list [image]",
		Short: "List tags for an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imagePath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tags, err := svc.ListTagsForImage(imagePath)
			if err != nil {
				return err
			}
			cmd.Println(strings.Join(tags, ", "))
			return nil
		},
	}
	rootCmd.AddCommand(listCmd)

	// Find images by tag
	findByTagCmd := &cobra.Command{
		Use:   "find-by-tag [tag]",
		Short: "List images with a given tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]
			images, err := svc.ListImagesForTag(tag)
			if err != nil {
				return err
			}
			for _, img := range images {
				cmd.Println(img)
			}
			return nil
		},
	}
	rootCmd.AddCommand(findByTagCmd)

	// List all tags
	listAllTagsCmd := &cobra.Command{
		Use:   "list-all-tags",
		Short: "List all tags with image counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, err := svc.ListAllTags()
			if err != nil {
				return err
			}
			for _, tag := range tags {
				cmd.Printf("%s (%d)\n", tag.Name, tag.Count)
			}
			return nil
		},
	}
	rootCmd.AddCommand(listAllTagsCmd)

	// Normalize all tags
	normalizeCmd := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize all tags to lowercase",
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.NormalizeAllTags()
		},
	}
	rootCmd.AddCommand(normalizeCmd)

	// Replace tag
	replaceTagCmd := &cobra.Command{
		Use:   "replace-tag [old] [new]",
		Short: "Replace an old tag with a new tag",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.ReplaceTag(args[0], args[1])
		},
	}
	rootCmd.AddCommand(replaceTagCmd)

	// Batch add tags to directory
	batchAddCmd := &cobra.Command{
		Use:   "batch-add [directory] [tag1,tag2,...]",
		Short: "Add tags to all images in a directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tags := strings.Split(args[1], ",")
			return svc.BatchAddTagsToDirectory(dir, tags)
		},
	}
	rootCmd.AddCommand(batchAddCmd)

	// Batch remove tags from directory
	batchRemoveCmd := &cobra.Command{
		Use:   "batch-remove [directory] [tag1,tag2,...]",
		Short: "Remove tags from all images in a directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tags := strings.Split(args[1], ",")
			_, _, err = svc.BatchRemoveTagsFromDirectory(dir, tags)
			return err
		},
	}
	rootCmd.AddCommand(batchRemoveCmd)

	// Clean database
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove tags for missing files and orphaned tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, err := svc.CleanDatabase()
			return err
		},
	}
	rootCmd.AddCommand(cleanCmd)

	// Add tags to all images with a specific tag
	addToTaggedCmd := &cobra.Command{
		Use:   "add-to-tagged [existingTag] [tag1,tag2,...]",
		Short: "Add new tags to all images that already have a specific tag",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := strings.Split(args[1], ",")
			_, err := svc.AddTagsToTaggedImages(args[0], tags)
			return err
		},
	}
	rootCmd.AddCommand(addToTaggedCmd)

	return rootCmd
}
