// Package main implements the command-line interface for FySlide.
package main

import (
	"context"
	"fmt"
	"fyslide/internal/scan"
	"fyslide/internal/service"
	"fyslide/internal/tagging"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	dbDirFlag           string
	tagDB               *tagging.TagDB
	svc                 *service.Service
	dryRunFlag          bool // Flag for delete dry-run
	forceFlag           bool // Flag for delete force
	verboseFlag         bool // Flag for verbose output
	mergeDuplicatesFlag bool // Flag for duplicates command to execute changes
)

func cliLogger(msg string) {
	if verboseFlag {
		log.Printf("[fyslide-cli] %s", msg)
	}
}

// this directive tells revive to ignore unused parameters in the following function
// revive:disable:unused-parameter

// NewRootCmd creates the root command for the CLI application.
// It takes a function `getServiceAndDB` which is responsible for initializing
// and returning the service and tagDB instances. This allows tests to inject mocks
// or test-specific instances.
func NewRootCmd(getServiceAndDB func(dbPath string, logger tagging.LoggerFunc) (*service.Service, *tagging.TagDB, error)) *cobra.Command {
	ctx := context.Background() // Create a context to pass to service methods
	var rootCmd = &cobra.Command{
		Use:   "fyslide-cli",
		Short: "FySlide CLI - manage image tags",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			// Use the provided function to get/initialize svc and tagDB
			svc, tagDB, err = getServiceAndDB(dbDirFlag, cliLogger)
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

	// --- Command Groups for better UX ---
	rootCmd.AddGroup(&cobra.Group{
		ID:    "tagging",
		Title: "Image Tagging:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    "batch",
		Title: "Batch Operations:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    "query",
		Title: "Querying:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    "maintenance",
		Title: "Database Maintenance:",
	})

	// Add command
	addCmd := &cobra.Command{
		Use:     "add [image] [tag]",
		Short:   "Add a tag to an image",
		Aliases: []string{"a"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imagePath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			return svc.AddTagsToImage(imagePath, []string{tag})
		},
	}
	addCmd.GroupID = "tagging"
	rootCmd.AddCommand(addCmd)

	// Remove command
	removeCmd := &cobra.Command{
		Use:     "remove [image] [tag]",
		Short:   "Remove a tag from an image",
		Aliases: []string{"rmtag"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imagePath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			return svc.RemoveTagsFromImage(imagePath, []string{tag})
		},
	}
	removeCmd.GroupID = "tagging"
	rootCmd.AddCommand(removeCmd)

	// List tags for image
	listCmd := &cobra.Command{
		Use:     "list [image]",
		Short:   "List tags for an image",
		Aliases: []string{"lsi"},
		Args:    cobra.ExactArgs(1),
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
	listCmd.GroupID = "tagging"
	rootCmd.AddCommand(listCmd)

	// Find images by tag
	findByTagCmd := &cobra.Command{
		Use:     "find-by-tag [tag]",
		Short:   "List images with a given tag",
		Aliases: []string{"find"},
		Args:    cobra.ExactArgs(1),
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
	findByTagCmd.GroupID = "query"
	rootCmd.AddCommand(findByTagCmd)

	// List all tags
	listAllTagsCmd := &cobra.Command{
		Use:     "list-all-tags",
		Short:   "List all tags with image counts",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, err := svc.ListAllTags()
			if err != nil {
				return err
			}
			for _, tag := range tags {
				cmd.Printf("%s (%d)\n", tag.Name, tag.Count)
			}
			cmd.Printf("\nTotal unique tags: %d\n", len(tags))
			return nil
		},
	}
	listAllTagsCmd.GroupID = "query"
	rootCmd.AddCommand(listAllTagsCmd)

	// // Normalize all tags
	// normalizeCmd := &cobra.Command{
	// 	Use:     "normalize",
	// 	Short:   "Normalize all tags to lowercase",
	// 	Aliases: []string{"norm"},
	// 	RunE: func(cmd *cobra.Command, args []string) error {
	// 		return svc.NormalizeAllTags()
	// 	},
	// }
	// normalizeCmd.GroupID = "maintenance"
	// rootCmd.AddCommand(normalizeCmd)

	// Replace tag
	replaceTagCmd := &cobra.Command{
		Use:     "replace-tag [old] [new]",
		Short:   "Replace an old tag with a new tag",
		Aliases: []string{"replace"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.ReplaceTag(args[0], args[1])
		},
	}
	replaceTagCmd.GroupID = "maintenance"
	rootCmd.AddCommand(replaceTagCmd)

	// Batch add tags to directory
	batchAddCmd := &cobra.Command{
		Use:     "batch-add [directory] [tag1,tag2,...]",
		Short:   "Add tags to all images in a directory",
		Aliases: []string{"badd"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tags := strings.Split(args[1], ",")
			return svc.BatchAddTagsToDirectory(ctx, dir, tags)
		},
	}
	batchAddCmd.GroupID = "batch"
	rootCmd.AddCommand(batchAddCmd)

	// Batch remove tags from directory
	batchRemoveCmd := &cobra.Command{
		Use:     "batch-remove [directory] [tag1,tag2,...]",
		Short:   "Remove tags from all images in a directory",
		Aliases: []string{"brm"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tags := strings.Split(args[1], ",")
			return svc.BatchRemoveTagsFromDirectory(ctx, dir, tags)
		},
	}
	batchRemoveCmd.GroupID = "batch"
	rootCmd.AddCommand(batchRemoveCmd)

	// Clean database
	cleanCmd := &cobra.Command{
		Use:     "clean",
		Short:   "Remove tags for missing files and orphaned tags",
		Aliases: []string{"cleanup"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, err := svc.CleanDatabase()
			return err
		},
	}
	cleanCmd.GroupID = "maintenance"
	rootCmd.AddCommand(cleanCmd)

	// Add tags to all images with a specific tag
	addToTaggedCmd := &cobra.Command{
		Use:     "add-to-tagged [existingTag] [tag1,tag2,...]",
		Short:   "Add new tags to all images that already have a specific tag",
		Aliases: []string{"addtagged"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := strings.Split(args[1], ",")
			_, err := svc.AddTagsToTaggedImages(args[0], tags)
			return err
		},
	}
	addToTaggedCmd.GroupID = "batch"
	rootCmd.AddCommand(addToTaggedCmd)

	// Find duplicate images and optionally merge them.
	duplicatesCmd := &cobra.Command{
		Use:   "duplicates [directory]",
		Short: "Find exact duplicate images in the current directory and subfolders",
		Long:  `Scan a directory for exact duplicate image files, including subfolders. By default, this command runs in dry-run mode and only reports what would happen. Use --force to actually merge duplicates by hard-linking them to the representative file.`,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			dir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if svc == nil {
				return fmt.Errorf("service is not initialized")
			}

			if !mergeDuplicatesFlag {
				cmd.Printf("Dry run enabled by default; use --force to actually merge duplicates.\n")
			}

			ctx := context.Background()
			start := time.Now()
			paths := make([]string, 0)
			logFn := func(msg string) {
				if verboseFlag {
					fmt.Fprintf(cmd.ErrOrStderr(), "[log] %s\n", msg)
				}
			}

			files := svc.FileScan.Run(ctx, dir, logFn)
			count := 0
			for item := range files {
				if item.Path != "" {
					paths = append(paths, item.Path)
					count++
					if count%1000 == 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "[log] scanned %d files...\n", count)
					}
				}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "[log] scanned %d image files in %s\n", count, time.Since(start).Round(time.Second))
			if len(paths) == 0 {
				cmd.Printf("No image files found in %s.\n", dir)
				return nil
			}

			groups, err := svc.FindDuplicates(ctx, paths)
			if err != nil {
				return err
			}
			if len(groups) == 0 {
				cmd.Printf("No duplicates found.\n")
				return nil
			}

			for _, group := range groups {
				cmd.Printf("Duplicate group (%d member(s), %s):\n", len(group.Members), group.MatchType)
				for _, member := range group.Members {
					cmd.Printf("  - %s\n", member)
				}
				if mergeDuplicatesFlag {
					result, err := svc.MergeDuplicateGroup(group)
					if err != nil {
						cmd.Printf("  merge failed: %v\n", err)
						continue
					}
					cmd.Printf("  Merged %d file(s) into %s\n", len(result.MergedPaths), result.RepresentativePath)
					if verboseFlag {
						cmd.Printf("[log] merged tags: %s\n", strings.Join(result.MergedTags, ", "))
					}
				} else {
					cmd.Printf("  Dry run: would merge %d file(s) into %s\n", len(group.Members)-1, group.RepresentativePath)
				}
			}
			return nil
		},
	}
	duplicatesCmd.Flags().BoolVarP(&mergeDuplicatesFlag, "force", "f", false, "Actually perform the merge instead of dry-run")
	duplicatesCmd.GroupID = "maintenance"
	rootCmd.AddCommand(duplicatesCmd)

	// Shell completion command
	var completionCmd = &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:
  $ source <(fyslide-cli completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ fyslide-cli completion bash > /etc/bash_completion.d/fyslide-cli
  # macOS:
  $ fyslide-cli completion bash > /usr/local/etc/bash_completion.d/fyslide-cli

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ fyslide-cli completion zsh > "${fpath[1]}/_fyslide-cli"

  # You will need to start a new shell for this setup to take effect.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			}
		},
	}
	rootCmd.AddCommand(completionCmd)

	// Delete files by tag
	deleteCmd := &cobra.Command{
		Use:   "delete [tag]",
		Short: "Delete all files and their tag database entries that match the given tag.",
		Long: `Delete all files from the file system that match the given tag, and remove their tag database entries.
WARNING: This operation is irreversible. There is NO recovery from deletion. Use --dryrun to preview what will be deleted.`,
		Aliases: []string{"del", "rm"},
		Args:    cobra.ExactArgs(1),
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

			if dryRunFlag {
				cmd.Println("[DRY RUN] No files or tags were deleted.")
				return nil
			}

			// If --force is not used, get confirmation.
			if !forceFlag {
				cmd.Printf("WARNING: This operation is IRREVERSIBLE. There is NO recovery from deletion.\n")
				cmd.Printf("Type 'delete' to confirm and proceed: ")
				var response string
				fmt.Fscanln(cmd.InOrStdin(), &response)
				if strings.ToLower(strings.TrimSpace(response)) != "delete" {
					cmd.Println("Aborted.")
					return nil
				}
			} else {
				cmd.Println("WARNING: --force specified. Deleting files immediately.")
			}

			var firstErr error
			deletedCount := 0
			for _, img := range images {
				err := svc.DeleteImageFile(img)
				if err != nil {
					cmd.Printf("Error deleting %s: %v\n", img, err)
					if firstErr == nil {
						firstErr = fmt.Errorf("one or more files could not be deleted")
					}
				} else {
					deletedCount++
					if verboseFlag {
						cmd.Printf("Deleted: %s\n", img)
					}
				}
			}
			cmd.Printf("Deletion complete. %d file(s) deleted.\n", deletedCount)
			return firstErr
		},
	}
	deleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Bypass confirmation prompt and delete files immediately")
	deleteCmd.Flags().BoolVar(&dryRunFlag, "dryrun", false, "Show what would be deleted but do not delete anything")
	deleteCmd.GroupID = "maintenance"
	rootCmd.AddCommand(deleteCmd)

	// Define persistent flags on the rootCmd returned by NewRootCmd
	// This ensures flags are available when NewRootCmd is called from main or tests.
	rootCmd.PersistentFlags().StringVar(&dbDirFlag, "db-dir", "", "Directory to store the tag database file in (defaults to user config dir)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output for all commands")

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
