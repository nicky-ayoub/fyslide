package main

import (
	"fmt"
	"fyslide/internal/tagging"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// dbPathFlag is used to store the value of the --dbpath flag
	dbPathFlag string
	// tagDB is our global instance of the tag database
	tagDB *tagging.TagDB
	// Flags for batch operations
	dryRunFlag bool
	forceFlag  bool
)

var supportedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "fyslide-cli",
	Short: "A CLI for managing tags for fyslide images",
	Long: `fyslide-cli is a command-line tool to add, remove, list,
and search tags associated with image files used by fyslide.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize the TagDB. If dbPathFlag is empty, NewTagDB uses its default.
		var err error
		tagDB, err = tagging.NewTagDB(dbPathFlag)
		if err != nil {
			return fmt.Errorf("failed to initialize tag database: %w", err)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if tagDB != nil {
			if err := tagDB.Close(); err != nil {
				// Use log.Printf or cmd.PrintErrf as cobra might have already handled exit.
				log.Printf("Error closing tag database: %v", err)
			}
		}
	},
}

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <filepath> <tag1> [tag2...]",
	Short: "Add one or more tags to a file",
	Long:  "Adds the specified tags to the given image file. The filepath should be the path to the image.",
	Args:  cobra.MinimumNArgs(2), // Requires filepath and at least one tag
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		tagsToAdd := args[1:]

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for %s: %w", filePath, err)
		}

		var firstError error
		for _, tag := range tagsToAdd {
			if err := tagDB.AddTag(absPath, tag); err != nil {
				cmd.PrintErrf("Error adding tag '%s' to %s: %v\n", tag, absPath, err)
				if firstError == nil {
					firstError = err // Capture the first error to return
				}
			} else {
				cmd.Printf("Added tag '%s' to %s\n", tag, absPath)
			}
		}
		return firstError // Return the first error encountered, if any
	},
}

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove <filepath> <tag1> [tag2...]",
	Short: "Remove one or more tags from a file",
	Long:  "Removes the specified tags from the given image file.",
	Args:  cobra.MinimumNArgs(2), // Requires filepath and at least one tag
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		tagsToRemove := args[1:]

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for %s: %w", filePath, err)
		}

		var firstError error
		for _, tag := range tagsToRemove {
			if err := tagDB.RemoveTag(absPath, tag); err != nil {
				cmd.PrintErrf("Error removing tag '%s' from %s: %v\n", tag, absPath, err)
				if firstError == nil {
					firstError = err
				}
			} else {
				cmd.Printf("Removed tag '%s' from %s\n", tag, absPath)
			}
		}
		return firstError
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list <filepath>",
	Short: "List tags for a specific file",
	Long:  "Displays all tags associated with the given image file.",
	Args:  cobra.ExactArgs(1), // Requires exactly one filepath
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for %s: %w", filePath, err)
		}

		tags, err := tagDB.GetTags(absPath)
		if err != nil {
			return fmt.Errorf("error listing tags for %s: %w", absPath, err)
		}

		if len(tags) == 0 {
			cmd.Printf("No tags found for %s\n", absPath)
			return nil
		}
		cmd.Printf("Tags for %s: %s\n", absPath, strings.Join(tags, ", "))
		return nil
	},
}

// findByTagCmd represents the find-by-tag command
var findByTagCmd = &cobra.Command{
	Use:   "find-by-tag <tag>",
	Short: "List files associated with a specific tag",
	Long:  "Finds and displays all image files that have the given tag.",
	Args:  cobra.ExactArgs(1), // Requires exactly one tag
	RunE: func(cmd *cobra.Command, args []string) error {
		tagToFind := args[0]
		images, err := tagDB.GetImages(tagToFind)
		if err != nil {
			return fmt.Errorf("error finding images for tag '%s': %w", tagToFind, err)
		}

		if len(images) == 0 {
			cmd.Printf("No images found with tag '%s'\n", tagToFind)
			return nil
		}

		cmd.Printf("Images with tag '%s':\n", tagToFind)
		for _, imgPath := range images {
			cmd.Println(imgPath)
		}
		return nil
	},
}

// listAllTagsCmd represents the list-all-tags command
var listAllTagsCmd = &cobra.Command{
	Use:   "list-all-tags",
	Short: "List all unique tags in the database",
	Long:  "Displays a list of all unique tags currently stored in the tag database.",
	Args:  cobra.NoArgs, // Takes no arguments
	RunE: func(cmd *cobra.Command, args []string) error {
		tags, err := tagDB.GetAllTags()
		if err != nil {
			return fmt.Errorf("error listing all tags: %w", err)
		}

		if len(tags) == 0 {
			cmd.Println("No tags found in the database.")
			return nil
		}

		cmd.Println("All tags in database:")
		for _, tag := range tags {
			cmd.Println(tag)
		}
		return nil
	},
}

// batchAddCmd represents the batch-add command
var batchAddCmd = &cobra.Command{
	Use:   "batch-add <directory> <tag1> [tag2...]",
	Short: "Add one or more tags to all image files in a directory",
	Long: `Adds the specified tags to all supported image files (jpg, jpeg, png, gif)
found directly within the given directory. This command does not recurse into subdirectories.`,
	Args: cobra.MinimumNArgs(2), // Requires directory and at least one tag
	RunE: func(cmd *cobra.Command, args []string) error {
		dirPath := args[0]
		absDirPath, err := filepath.Abs(dirPath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for directory %s: %w", dirPath, err)
		}
		tags := args[1:]
		return processFilesInDirectory(cmd, absDirPath, tags, tagDB.AddTag, "Added", "add", dryRunFlag, false /* no confirmation for add */, forceFlag)
	},
}

// batchRemoveCmd represents the batch-remove command
var batchRemoveCmd = &cobra.Command{
	Use:   "batch-remove <directory> <tag1> [tag2...]",
	Short: "Remove one or more tags from all image files in a directory",
	Long: `Removes the specified tags from all supported image files (jpg, jpeg, png, gif)
found directly within the given directory. This command does not recurse into subdirectories.`,
	Args: cobra.MinimumNArgs(2), // Requires directory and at least one tag
	RunE: func(cmd *cobra.Command, args []string) error {
		dirPath := args[0]
		tagsToRemove := args[1:]

		absDirPath, err := filepath.Abs(dirPath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for directory %s: %w", dirPath, err)
		}

		return processFilesInDirectory(cmd, absDirPath, tagsToRemove, tagDB.RemoveTag, "Removed", "remove", dryRunFlag, true /* needs confirmation */, forceFlag)
	},
}

func init() {
	// Add persistent flags to the root command (available to all subcommands)
	// The default value for dbPathFlag is "", which means tagging.NewTagDB will use its internal default.
	rootCmd.PersistentFlags().StringVar(&dbPathFlag, "dbpath", "", "Path to the tag database file (e.g., /path/to/tags.db). If empty, uses default location.")

	// Add flags for batch commands
	batchAddCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Simulate the batch add operation without making changes.")
	batchRemoveCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Simulate the batch remove operation without making changes.")
	batchRemoveCmd.Flags().BoolVar(&forceFlag, "force", false, "Force batch removal without confirmation.")

	// Add subcommands to the root command
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(findByTagCmd)
	rootCmd.AddCommand(listAllTagsCmd)
	rootCmd.AddCommand(batchAddCmd)
	rootCmd.AddCommand(batchRemoveCmd)
}

// processFilesInDirectory is a helper function to reduce duplication between batch-add and batch-remove
func processFilesInDirectory(cmd *cobra.Command, dirPath string, tagsToProcess []string,
	tagAction func(filePath, tag string) error, actionVerb, operationName string,
	isDryRun, needsConfirmation, isForced bool) error {

	if needsConfirmation && !isForced && !isDryRun {
		cmd.Printf("WARNING: You are about to %s %d tag(s) from all supported images in directory:\n%s\n",
			operationName, len(tagsToProcess), dirPath)
		cmd.Print("This action cannot be undone for the actual files.\nAre you sure you want to continue? (yes/no): ")
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || strings.ToLower(strings.TrimSpace(response)) != "yes" {
			if err != nil && err.Error() != "unexpected newline" && err.Error() != "EOF" { // Handle actual Scanln errors
				cmd.PrintErrf("Error reading confirmation: %v\n", err)
			}
			cmd.Println("Operation cancelled by user.")
			return nil
		}
	}

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %w", dirPath, err)
	}

	var firstError error
	filesProcessed := 0
	tagsAppliedCount := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if !supportedImageExtensions[ext] {
			continue
		}

		filesProcessed++
		filePath := filepath.Join(dirPath, file.Name())
		for _, tag := range tagsToProcess {
			if isDryRun {
				cmd.Printf("DRY RUN: Would %s tag '%s' for %s\n", operationName, tag, filePath)
				tagsAppliedCount++ // Count as if it were applied for dry run summary
			} else {
				if err := tagAction(filePath, tag); err != nil {
					cmd.PrintErrf("Error %sing tag '%s' for %s: %v\n", operationName, tag, filePath, err)
					if firstError == nil {
						firstError = err
					}
				} else {
					cmd.Printf("%s tag '%s' for %s\n", actionVerb, tag, filePath)
					tagsAppliedCount++
				}
			}
		}
	}

	summaryPrefix := "Finished"
	if isDryRun {
		summaryPrefix = "DRY RUN: Finished simulation of"
	}
	cmd.Printf("%s batch %s. Processed %d image files. %s %d tag instances in %s.\n", summaryPrefix, operationName, filesProcessed, actionVerb, tagsAppliedCount, dirPath)
	return firstError
}
func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra prints the error, so we just exit
		os.Exit(1)
	}
}
