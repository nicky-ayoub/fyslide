package main

import (
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/tagging"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	dbPathFlag string
	tagDB      *tagging.TagDB
	svc        *service.Service
	dryRunFlag bool
	forceFlag  bool
)

func cliLogger(msg string) {
	log.Printf("[fyslide-cli] %s", msg)
}

// NewRootCmd creates the root command for the CLI application.
// It takes a function `getServiceAndDB` which is responsible for initializing
// and returning the service and tagDB instances. This allows tests to inject mocks
// or test-specific instances.
func NewRootCmd(getServiceAndDB func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error)) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "fyslide-cli",
		Short: "FySlide CLI - manage image tags",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			// Use the provided function to get/initialize svc and tagDB
			svc, tagDB, err = getServiceAndDB(dbPathFlag, cliLogger)
			if err != nil {
				return fmt.Errorf("failed to initialize service and tagDB: %w", err)
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if tagDB != nil {
				tagDB.Close()
			}
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

	// Delete files by tag
	deleteCmd := &cobra.Command{
		Use:   "delete [tag]",
		Short: "Delete all files and their tag database entries that match the given tag.",
		Long: `Delete all files from the file system that match the given tag, and remove their tag database entries.
WARNING: This operation is irreversible. There is NO recovery from deletion. Use --dryrun to preview what will be deleted.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]
			images, err := svc.ListImagesForTag(tag)
			if err != nil {
				return fmt.Errorf("failed to list images for tag '%s': %w", tag, err)
			}
			if len(images) == 0 {
				cmd.Printf("No images found for tag '%s'.\n", tag)
				return nil
			}

			// Always show the summary before any destructive action
			cmd.Printf("The following files will be deleted for tag '%s':\n", tag)
			for _, img := range images {
				cmd.Printf("  %s\n", img)
				tags, err := svc.ListTagsForImage(img)
				if err == nil && len(tags) > 0 {
					cmd.Printf("    Tags: %s\n", strings.Join(tags, ", "))
				}
			}

			// Default to dry run unless --force is specified
			if !forceFlag {
				cmd.Printf("[DRY RUN] No files or tags were deleted. Use --force to actually delete.\n")
				return nil
			}

			cmd.Printf("WARNING: This operation is IRREVERSIBLE. There is NO recovery from deletion.\n")
			cmd.Printf("Type 'delete' to confirm and proceed: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(strings.TrimSpace(response)) != "delete" {
				cmd.Println("Aborted.")
				return nil
			}

			// Actual deletion (no confirmation needed if --force is set)
			for _, img := range images {
				err := svc.DeleteImageFile(img)
				if err != nil {
					cmd.Printf("Failed to delete %s: %v\n", img, err)
				} else {
					cmd.Printf("Deleted %s and its tags.\n", img)
				}
			}
			return nil
		},
	}
	deleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Bypass confirmation prompt and delete files immediately")
	deleteCmd.Flags().BoolVar(&dryRunFlag, "dryrun", false, "Show what would be deleted but do not delete anything")
	rootCmd.AddCommand(deleteCmd)

	// Define persistent flags on the rootCmd returned by NewRootCmd
	// This ensures flags are available when NewRootCmd is called from main or tests.
	rootCmd.PersistentFlags().StringVar(&dbPathFlag, "dbpath", "", "Path to tag database")

	return rootCmd
}

func main() {
	// Define the actual service and DB initialization logic for the main application
	getSvcAndDBFunc := func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error) {
		tdb, err := tagging.NewTagDB(dbPath, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open tag DB: %w", err)
		}
		s := service.NewService(tdb, &scan.FileScannerImpl{}, logger)
		return s, tdb, nil
	}
	rootCmd := NewRootCmd(getSvcAndDBFunc)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
