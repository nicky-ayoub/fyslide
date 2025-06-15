package main

import (
	"fmt"
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

func main() {
	rootCmd := &cobra.Command{
		Use:   "fyslide-cli",
		Short: "FySlide CLI - manage image tags",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			tagDB, err = tagging.NewTagDB(dbPathFlag, cliLogger)
			if err != nil {
				return fmt.Errorf("failed to open tag DB: %w", err)
			}
			svc = service.NewService(tagDB, nil, cliLogger)
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if tagDB != nil {
				tagDB.Close()
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&dbPathFlag, "dbpath", "", "Path to tag database")

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
			fmt.Println(strings.Join(tags, ", "))
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
				fmt.Println(img)
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
				fmt.Printf("%s (%d)\n", tag.Name, tag.Count)
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
