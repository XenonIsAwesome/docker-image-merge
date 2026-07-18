// Package cmd implements the root Cobra command for docker-image-merge.
//
// It orchestrates the full merge pipeline:
//  1. Pull/inspect both images
//  2. Extract filesystems via temporary containers
//  3. Diff the two trees and detect conflicts
//  4. Resolve conflicts (interactive TUI or automatic strategy)
//  5. Apply resolutions and produce the merged tree
//  6. Import/build the output image
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/XenonIsAwesome/docker-image-merge/internal/docker"
	"github.com/XenonIsAwesome/docker-image-merge/internal/flags"
	"github.com/XenonIsAwesome/docker-image-merge/internal/merge"
	"github.com/XenonIsAwesome/docker-image-merge/internal/tui"
)

// NewRootCmd creates the root Cobra command for the docker-image-merge CLI.
//
// The command expects exactly three positional arguments:
//   - image-a: the "base" image whose filesystem starts as the merged result
//   - image-b: the "incoming" image whose changes are overlaid
//   - output-image: the tag for the newly created merged image
//
// Flags control conflict resolution strategy, metadata inheritance, squashing,
// and verbosity. Run with --help for the full flag listing.
func NewRootCmd() *cobra.Command {
	opts := &flags.Options{}

	cmd := &cobra.Command{
		Use:   "imagemerge <image-a> <image-b> <output-image>",
		Short: "Merge the filesystems of two Docker images",
		Long: `Merge the filesystems of two Docker images into a new image.

The tool extracts the filesystem from each image, detects conflicts
(content changes, permission changes, type changes), and presents an
interactive TUI for resolution. The merged result is imported as a
new Docker image.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMerge(args[0], args[1], args[2], opts)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags.AddFlags(cmd, opts)
	return cmd
}

// runMerge is the core pipeline that extracts two images, diffs them,
// resolves conflicts, and produces the merged output image.
//
// It connects to the Docker daemon, inspects both images for metadata,
// exports their filesystems to temporary directories, runs the diff engine,
// applies the chosen conflict-resolution strategy, and finally creates the
// output image via either a layered build or a squashed import.
func runMerge(imageA, imageB, outputImage string, opts *flags.Options) error {
	ctx := context.Background()

	// Validate flags before doing any work.
	if err := validateStrategy(opts.Strategy); err != nil {
		return err
	}
	if err := validateMetadataSource(opts.MetaFrom); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Merging %s + %s -> %s\n", imageA, imageB, outputImage)
	fmt.Fprintf(os.Stderr, "Strategy: %s\n", opts.Strategy)

	// Connect to the local Docker daemon via the docker CLI.
	dockerClient, err := docker.NewClient(opts.Platform)
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dockerClient.Close()

	// Ensure both images are available locally (pulls if needed).
	fmt.Fprintf(os.Stderr, "Ensuring images are available...\n")
	if err := dockerClient.EnsureImage(ctx, imageA); err != nil {
		return fmt.Errorf("preparing image A: %w", err)
	}
	if err := dockerClient.EnsureImage(ctx, imageB); err != nil {
		return fmt.Errorf("preparing image B: %w", err)
	}

	// Inspect both images to capture config metadata (ENV, CMD, etc.).
	metaA, err := dockerClient.InspectImage(ctx, imageA)
	if err != nil {
		return fmt.Errorf("inspecting image A: %w", err)
	}
	metaB, err := dockerClient.InspectImage(ctx, imageB)
	if err != nil {
		return fmt.Errorf("inspecting image B: %w", err)
	}

	// Create a temporary workspace for extraction and merging.
	tmpDir, err := os.MkdirTemp("", "docker-image-merge-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rootA := filepath.Join(tmpDir, "image-a")
	rootB := filepath.Join(tmpDir, "image-b")

	// Extract each image's flattened filesystem.
	fmt.Fprintf(os.Stderr, "Extracting filesystem from %s...\n", imageA)
	if err := dockerClient.ExtractFilesystem(ctx, imageA, rootA); err != nil {
		return fmt.Errorf("extracting image A: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Extracting filesystem from %s...\n", imageB)
	if err := dockerClient.ExtractFilesystem(ctx, imageB, rootB); err != nil {
		return fmt.Errorf("extracting image B: %w", err)
	}

	// Run the diff engine to detect all conflicts between the two trees.
	fmt.Fprintf(os.Stderr, "Comparing filesystems...\n")
	diffEngine := merge.NewDiffEngine(rootA, rootB)
	diffResult, err := diffEngine.Run()
	if err != nil {
		return fmt.Errorf("comparing filesystems: %w", err)
	}

	if opts.Verbose {
		printDiffStats(diffResult.Stats)
	}

	// If the images are identical, there is nothing to merge.
	if len(diffResult.Conflicts) == 0 {
		fmt.Fprintf(os.Stderr, "No differences found. Images are identical.\n")
		return nil
	}

	// Count conflicts that need user/strategy resolution.
	conflictCount := 0
	for _, c := range diffResult.Conflicts {
		if c.Kind.NeedsResolution() {
			conflictCount++
		}
	}

	// Auto-resolve non-conflicting differences (files only in A or B).
	if conflictCount == 0 {
		fmt.Fprintf(os.Stderr, "Found %d differences, no conflicts.\n", len(diffResult.Conflicts))
		for _, c := range diffResult.Conflicts {
			switch c.Kind {
			case merge.OnlyA:
				c.Resolution = merge.ResolutionTakeA
			case merge.OnlyB:
				c.Resolution = merge.ResolutionTakeB
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "Found %d differences (%d need resolution)\n",
			len(diffResult.Conflicts), conflictCount)
	}

	// Apply the chosen conflict resolution strategy.
	switch opts.Strategy {
	case flags.StrategyFail:
		// Print conflicts and exit with an error.
		fmt.Fprintf(os.Stderr, "\nConflicts found:\n")
		for _, c := range diffResult.Conflicts {
			if c.Kind.NeedsResolution() {
				fmt.Fprintf(os.Stderr, "  - %s\n", c.Summary())
			}
		}
		return fmt.Errorf("%d conflicts found (strategy: fail)", conflictCount)

	case flags.StrategyAutoA:
		// Automatically take image A's version for every conflict.
		for _, c := range diffResult.Conflicts {
			if c.Kind.NeedsResolution() {
				c.Resolution = merge.ResolutionTakeA
			} else if c.Kind == merge.OnlyB {
				c.Resolution = merge.ResolutionTakeB
			} else {
				c.Resolution = merge.ResolutionTakeA
			}
		}

	case flags.StrategyAutoB:
		// Automatically take image B's version for every conflict.
		for _, c := range diffResult.Conflicts {
			if c.Kind.NeedsResolution() {
				c.Resolution = merge.ResolutionTakeB
			} else if c.Kind == merge.OnlyB {
				c.Resolution = merge.ResolutionTakeB
			} else {
				c.Resolution = merge.ResolutionTakeA
			}
		}

	case flags.StrategyInteractive:
		// Launch the BubbleTea TUI for per-file conflict resolution.
		confirmed, err := tui.Run(diffResult.Conflicts)
		if err != nil {
			return fmt.Errorf("running conflict resolver: %w", err)
		}
		if !confirmed {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			return nil
		}
	}

	// Apply the resolved choices to produce the merged directory tree.
	fmt.Fprintf(os.Stderr, "Applying resolutions...\n")
	applyResult, err := merge.ApplyResolutions(rootA, rootB, diffResult.Conflicts, tmpDir)
	if err != nil {
		return fmt.Errorf("applying resolutions: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Merged %d files (A: %d, B: %d, Skipped: %d)\n",
		applyResult.TotalFiles, applyResult.FromA, applyResult.FromB, applyResult.Skipped)

	// Choose which image's metadata (ENV, CMD, etc.) to inherit.
	var metaToUse *docker.ImageMetadata
	if opts.MetaFrom == flags.MetadataFromB {
		metaToUse = metaB
	} else {
		metaToUse = metaA
	}

	// Create the output image.
	fmt.Fprintf(os.Stderr, "Creating output image...\n")

	if opts.Squash {
		// Squashed mode: single flattened layer via docker import.
		imageID, err := dockerClient.ImportSquashed(ctx, applyResult.MergedDir,
			outputImage, metaToUse, opts.Changes, opts.Message)
		if err != nil {
			return fmt.Errorf("importing merged image: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Created image %s (%s)\n", outputImage, imageID[:12])
	} else {
		// Layered mode: build on top of image A to preserve its layers.
		imageID, err := dockerClient.BuildLayered(ctx, imageA, applyResult.MergedDir,
			outputImage, metaToUse, opts.Changes)
		if err != nil {
			// Fall back to squashed if the layered build fails.
			fmt.Fprintf(os.Stderr, "Layered build failed, falling back to squash: %v\n", err)
			imageID, err = dockerClient.ImportSquashed(ctx, applyResult.MergedDir,
				outputImage, metaToUse, opts.Changes, opts.Message)
			if err != nil {
				return fmt.Errorf("importing merged image: %w", err)
			}
		}
		fmt.Fprintf(os.Stderr, "Created image %s (%s)\n", outputImage, imageID[:12])
	}

	return nil
}

// validateStrategy checks that the given strategy string is one of the
// supported values: "interactive", "auto-a", "auto-b", "fail".
func validateStrategy(s flags.Strategy) error {
	switch s {
	case flags.StrategyInteractive, flags.StrategyAutoA, flags.StrategyAutoB, flags.StrategyFail:
		return nil
	default:
		return fmt.Errorf("unknown strategy: %s (valid: interactive, auto-a, auto-b, fail)", s)
	}
}

// validateMetadataSource checks that the metadata source is either "a" or "b".
func validateMetadataSource(ms flags.MetadataSource) error {
	switch ms {
	case flags.MetadataFromA, flags.MetadataFromB:
		return nil
	default:
		return fmt.Errorf("unknown metadata source: %s (valid: a, b)", ms)
	}
}

// printDiffStats writes a human-readable summary of the diff results to stderr.
func printDiffStats(stats merge.DiffStats) {
	fmt.Fprintf(os.Stderr, "  Only in A:      %d\n", stats.OnlyA)
	fmt.Fprintf(os.Stderr, "  Only in B:      %d\n", stats.OnlyB)
	fmt.Fprintf(os.Stderr, "  Identical:      %d\n", stats.Same)
	fmt.Fprintf(os.Stderr, "  Conflicts:      %d\n", stats.Conflicts)
	fmt.Fprintf(os.Stderr, "  Type changes:   %d\n", stats.TypeChange)
	fmt.Fprintf(os.Stderr, "  Perm only:      %d\n", stats.PermOnly)
	fmt.Fprintf(os.Stderr, "  Both deleted:   %d\n", stats.BothDeleted)
}
