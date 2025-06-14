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
		// Define a logger function for the CLI context
		cliLogger := func(message string) {
			// log.Printf is suitable here for messages from the tagging package.
			// It distinguishes these from direct command output via cmd.Printf.
			log.Printf("TagDB: %s", message)
		}
		tagDB, err = tagging.NewTagDB(dbPathFlag, cliLogger)
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
		for _, tagRaw := range tagsToAdd {
			tag := strings.ToLower(tagRaw) // Normalize tag to lowercase
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
		for _, tagRaw := range tagsToRemove {
			tag := strings.ToLower(tagRaw) // Normalize tag to lowercase
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
		tagToFindRaw := args[0]
		tagToFind := strings.ToLower(tagToFindRaw) // Normalize tag to lowercase
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
		for _, tagInfo := range tags { // tags is []tagging.TagWithCount
			cmd.Printf("%s (Count: %d)\n", tagInfo.Name, tagInfo.Count)
		}
		return nil
	},
}

// normalizeCmd represents the command to normalize all tags to lowercase
var normalizeCmd = &cobra.Command{
	Use:   "normalize",
	Short: "Normalize all tags in the database to lowercase",
	Long: `Iterates through all tags in the database. If a tag is found that is not
entirely in lowercase, it will be normalized. This involves:
1. Identifying all images associated with the mixed-case tag.
2. For each such image, removing the mixed-case tag.
3. For each such image, adding the lowercase version of the tag.
This ensures all tag references are consistently lowercase.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		allTagsWithCounts, err := tagDB.GetAllTags() // Fetches []tagging.TagWithCount
		if err != nil {
			return fmt.Errorf("error fetching all tags: %w", err)
		}

		if len(allTagsWithCounts) == 0 {
			cmd.Println("No tags found in the database. Nothing to normalize.")
			return nil
		}

		cmd.Printf("Starting tag normalization process...\n")
		if dryRunFlag {
			cmd.Println("DRY RUN: No changes will be made to the database.")
		}

		mixedCaseTagsFound := 0
		imageTagUpdatesCount := 0
		var firstError error

		// Create a set of unique tag names to process
		uniqueTagNames := make(map[string]struct{})
		for _, tagInfo := range allTagsWithCounts {
			uniqueTagNames[tagInfo.Name] = struct{}{}
		}

		for originalTag := range uniqueTagNames {
			lowerTag := strings.ToLower(originalTag)

			if originalTag == lowerTag {
				// Tag is already lowercase, skip
				continue
			}

			cmd.Printf("Found mixed-case tag: '%s'. Normalizing to '%s'.\n", originalTag, lowerTag)
			mixedCaseTagsFound++

			imagePaths, errGetImages := tagDB.GetImages(originalTag)
			if errGetImages != nil {
				cmd.PrintErrf("  Error getting images for original tag '%s': %v. Skipping this tag.\n", originalTag, errGetImages)
				if firstError == nil {
					firstError = errGetImages
				}
				continue
			}

			for _, imgPath := range imagePaths {
				if dryRunFlag {
					cmd.Printf("  DRY RUN: Would update tag for %s: '%s' -> '%s'\n", imgPath, originalTag, lowerTag)
					imageTagUpdatesCount++
				} else {
					if err := tagDB.RemoveTag(imgPath, originalTag); err != nil {
						cmd.PrintErrf("  Error removing original tag '%s' from %s: %v\n", originalTag, imgPath, err)
						if firstError == nil {
							firstError = err
						}
					}
					if err := tagDB.AddTag(imgPath, lowerTag); err != nil {
						cmd.PrintErrf("  Error adding lowercase tag '%s' to %s: %v\n", lowerTag, imgPath, err)
						if firstError == nil {
							firstError = err
						}
					} else {
						cmd.Printf("  Normalized tag for %s: '%s' -> '%s'\n", imgPath, originalTag, lowerTag)
						imageTagUpdatesCount++
					}
				}
			}
		}

		cmd.Printf("\nNormalization process complete.\n")
		cmd.Printf("Summary:\n")
		cmd.Printf("  Unique mixed-case tags processed for normalization: %d\n", mixedCaseTagsFound)
		cmd.Printf("  Image-tag associations updated/would be updated: %d\n", imageTagUpdatesCount)
		if firstError != nil {
			cmd.PrintErrf("Errors were encountered during the process. Please check the log. First error: %v\n", firstError)
		}
		return firstError
	},
}

// replaceTagCmd represents the command to replace an old tag with a new tag
var replaceTagCmd = &cobra.Command{
	Use:   "replace-tag <oldTag> <newTag>",
	Short: "Replace an existing tag with a new tag across all relevant images",
	Long: `Finds all images tagged with <oldTag>, removes <oldTag>, and adds <newTag> to them.
Both oldTag and newTag are treated as case-insensitive (normalized to lowercase).
If, after normalization, oldTag and newTag are identical, no action is taken.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldTagRaw := args[0]
		newTagRaw := args[1]

		oldTag := strings.ToLower(oldTagRaw)
		newTag := strings.ToLower(newTagRaw)

		if oldTag == newTag {
			cmd.Printf("Old tag ('%s') and new tag ('%s') are the same after normalization. No action taken.\n", oldTagRaw, newTagRaw)
			return nil
		}

		cmd.Printf("Replacing tag '%s' with '%s' (normalized from '%s' and '%s').\n", oldTag, newTag, oldTagRaw, newTagRaw)
		if dryRunFlag {
			cmd.Println("DRY RUN: No changes will be made to the database.")
		}

		imagePaths, err := tagDB.GetImages(oldTag)
		if err != nil {
			return fmt.Errorf("error fetching images for old tag '%s': %w", oldTag, err)
		}

		if len(imagePaths) == 0 {
			cmd.Printf("No images found with the old tag '%s'. Nothing to replace.\n", oldTag)
			return nil
		}

		cmd.Printf("Found %d image(s) with tag '%s'. Proceeding with replacement...\n", len(imagePaths), oldTag)
		var firstError error
		successfulReplacements := 0

		for _, imgPath := range imagePaths {
			if dryRunFlag {
				cmd.Printf("  DRY RUN: Would replace tag on %s: remove '%s', add '%s'\n", imgPath, oldTag, newTag)
				successfulReplacements++
			} else {
				if err := tagDB.RemoveTag(imgPath, oldTag); err != nil {
					cmd.PrintErrf("  Error removing old tag '%s' from %s: %v\n", oldTag, imgPath, err)
					if firstError == nil {
						firstError = err
					}
					continue // Skip adding new tag if old one couldn't be removed
				}
				if err := tagDB.AddTag(imgPath, newTag); err != nil {
					cmd.PrintErrf("  Error adding new tag '%s' to %s: %v\n", newTag, imgPath, err)
					if firstError == nil {
						firstError = err
					}
				} else {
					cmd.Printf("  Successfully replaced tag on %s: removed '%s', added '%s'\n", imgPath, oldTag, newTag)
					successfulReplacements++
				}
			}
		}

		cmd.Printf("\nTag replacement process complete.\n")
		cmd.Printf("Summary:\n")
		cmd.Printf("  Images processed for replacement: %d\n", len(imagePaths))
		cmd.Printf("  Tag replacements performed/would be performed: %d\n", successfulReplacements)
		if firstError != nil {
			cmd.PrintErrf("Errors were encountered during the process. Please check the log. First error: %v\n", firstError)
		}
		return firstError
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
		tagsRaw := args[1:]
		var tagsNormalized []string
		for _, tRaw := range tagsRaw {
			tagsNormalized = append(tagsNormalized, strings.ToLower(tRaw)) // Normalize tags
		}
		return processFilesInDirectory(cmd, absDirPath, tagsNormalized, tagDB.AddTag, "Added", "add", dryRunFlag, false /* no confirmation for add */, forceFlag)
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
		tagsToRemoveRaw := args[1:]
		var tagsToRemoveNormalized []string
		for _, tRaw := range tagsToRemoveRaw {
			tagsToRemoveNormalized = append(tagsToRemoveNormalized, strings.ToLower(tRaw)) // Normalize tags
		}

		absDirPath, err := filepath.Abs(dirPath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for directory %s: %w", dirPath, err)
		}

		return processFilesInDirectory(cmd, absDirPath, tagsToRemoveNormalized, tagDB.RemoveTag, "Removed", "remove", dryRunFlag, true /* needs confirmation */, forceFlag)
	},
}

// cleanCmd represents the database cleanup command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean the tag database by removing stale entries",
	Long: `Performs cleanup operations on the tag database:
1. Removes tag entries for image files that no longer exist on the filesystem.
2. Removes tags that are no longer associated with any images (orphaned tags).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Starting database cleanup...")
		if dryRunFlag {
			cmd.Println("DRY RUN: No changes will be made to the database.")
		}

		var firstError error
		actualFilesCleaned := 0
		actualTagsCleaned := 0
		potentialFilesToClean := 0
		potentialTagsToClean := 0

		// Phase 1: Clean tags for non-existent image files
		cmd.Println("\nPhase 1: Checking for non-existent image files and their tags...")
		imagePathsFromDB, err := tagDB.GetAllImagePaths()
		if err != nil {
			cmd.PrintErrf("  Error reading image paths from DB: %v\n", err)
			return fmt.Errorf("failed to get image paths for cleanup: %w", err)
		}

		for _, imagePath := range imagePathsFromDB {
			_, statErr := os.Stat(imagePath)
			if os.IsNotExist(statErr) {
				potentialFilesToClean++
				if dryRunFlag {
					cmd.Printf("  DRY RUN: Would remove all tags for non-existent file: %s\n", imagePath)
				} else {
					cmd.Printf("  Removing all tags for non-existent file: %s\n", imagePath)
					if err := tagDB.RemoveAllTagsForImage(imagePath); err != nil {
						cmd.PrintErrf("    Error removing tags for %s: %v\n", imagePath, err)
						if firstError == nil {
							firstError = err
						}
					} else {
						cmd.Printf("    Successfully removed tags for %s.\n", imagePath)
						actualFilesCleaned++
					}
				}
			}
		}

		// Phase 2: Clean orphaned tags (tags with no images)
		cmd.Println("\nPhase 2: Checking for orphaned tags...")
		allTagsWithCounts, err := tagDB.GetAllTags() // This reads from TagsToImages
		if err != nil {
			cmd.PrintErrf("  Error getting all tags for orphan check: %v\n", err)
			if firstError == nil {
				firstError = err
			}
		} else {
			for _, tagInfo := range allTagsWithCounts {
				if tagInfo.Count == 0 { // Tag exists in TagsToImages but has an empty image list
					potentialTagsToClean++
					if dryRunFlag {
						cmd.Printf("  DRY RUN: Would remove orphaned tag: %s\n", tagInfo.Name)
					} else {
						cmd.Printf("  Removing orphaned tag: %s\n", tagInfo.Name)
						if errDel := tagDB.DeleteOrphanedTagKey(tagInfo.Name); errDel != nil {
							cmd.PrintErrf("    Error removing orphaned tag '%s': %v\n", tagInfo.Name, errDel)
							if firstError == nil {
								firstError = errDel
							}
						} else {
							cmd.Printf("    Successfully removed orphaned tag: %s\n", tagInfo.Name)
							actualTagsCleaned++
						}
					}
				}
			}
		}

		cmd.Printf("\nCleanup process complete.\n")
		cmd.Printf("Summary:\n")
		if dryRunFlag {
			cmd.Printf("  Non-existent image file entries that would be processed: %d\n", potentialFilesToClean)
			cmd.Printf("  Orphaned tags that would be removed: %d\n", potentialTagsToClean)
		} else {
			cmd.Printf("  Non-existent image file entries processed: %d\n", actualFilesCleaned)
			cmd.Printf("  Orphaned tags removed: %d\n", actualTagsCleaned)
		}
		return firstError
	},
}

// addToTaggedCmd represents the command to add new tags to files already having a specific tag
var addToTaggedCmd = &cobra.Command{
	Use:   "add-to-tagged <initialTag> <tagToAdd1> [tagToAdd2...]",
	Short: "Add new tags to all files that already have <initialTag>",
	Long: `Finds all image files currently tagged with <initialTag>.
Then, for each of these files, it adds <tagToAdd1>, <tagToAdd2>, etc.
All tags are treated as case-insensitive (normalized to lowercase).`,
	Args: cobra.MinimumNArgs(2), // Requires initialTag and at least one tagToAdd
	RunE: func(cmd *cobra.Command, args []string) error {
		initialTagRaw := args[0]
		tagsToAddRaw := args[1:]

		initialTag := strings.ToLower(initialTagRaw)
		var tagsToAdd []string
		for _, tRaw := range tagsToAddRaw {
			tagsToAdd = append(tagsToAdd, strings.ToLower(tRaw))
		}

		cmd.Printf("Identifying files with initial tag '%s' to add new tag(s): [%s]\n", initialTag, strings.Join(tagsToAdd, ", "))
		if dryRunFlag {
			cmd.Println("DRY RUN: No changes will be made to the database.")
		}

		imagePaths, err := tagDB.GetImages(initialTag)
		if err != nil {
			return fmt.Errorf("error fetching images for initial tag '%s': %w", initialTag, err)
		}

		if len(imagePaths) == 0 {
			cmd.Printf("No images found with the initial tag '%s'. No new tags will be added.\n", initialTag)
			return nil
		}

		cmd.Printf("Found %d image(s) with tag '%s'. Proceeding to add new tags...\n", len(imagePaths), initialTag)
		var firstError error
		successfulAdditions := 0

		for _, imgPath := range imagePaths {
			for _, tag := range tagsToAdd {
				if dryRunFlag {
					cmd.Printf("  DRY RUN: Would add tag '%s' to %s (which has '%s')\n", tag, imgPath, initialTag)
					successfulAdditions++ // Count as if it were applied for dry run summary
				} else {
					if err := tagDB.AddTag(imgPath, tag); err != nil {
						cmd.PrintErrf("  Error adding tag '%s' to %s: %v\n", tag, imgPath, err)
						if firstError == nil {
							firstError = err
						}
					} else {
						cmd.Printf("  Added tag '%s' to %s\n", tag, imgPath)
						successfulAdditions++
					}
				}
			}
		}

		summaryPrefix := "Finished"
		if dryRunFlag {
			summaryPrefix = "DRY RUN: Finished simulation of"
		}
		cmd.Printf("\n%s adding tags. Processed %d image files. Added %d new tag instances.\n", summaryPrefix, len(imagePaths), successfulAdditions)
		if firstError != nil {
			cmd.PrintErrf("Errors were encountered during the process. Please check the log. First error: %v\n", firstError)
		}
		return firstError
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
	normalizeCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Simulate the normalization process without making changes.")
	replaceTagCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Simulate the tag replacement process without making changes.")
	cleanCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Simulate the cleanup process without making changes.")
	addToTaggedCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Simulate adding new tags without making changes.")

	// Add subcommands to the root command
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(findByTagCmd)
	rootCmd.AddCommand(listAllTagsCmd)
	rootCmd.AddCommand(batchAddCmd)
	rootCmd.AddCommand(batchRemoveCmd)
	rootCmd.AddCommand(replaceTagCmd)
	rootCmd.AddCommand(normalizeCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(addToTaggedCmd)
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
