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

func NewRootCmd() *cobra.Command {
	opts := &flags.Options{}

	cmd := &cobra.Command{
		Use:   "docker-image-merge <image-a> <image-b> <output-image>",
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

func runMerge(imageA, imageB, outputImage string, opts *flags.Options) error {
	ctx := context.Background()

	if err := validateStrategy(opts.Strategy); err != nil {
		return err
	}

	if err := validateMetadataSource(opts.MetaFrom); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Merging %s + %s -> %s\n", imageA, imageB, outputImage)
	fmt.Fprintf(os.Stderr, "Strategy: %s\n", opts.Strategy)

	dockerClient, err := docker.NewClient(opts.Platform)
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dockerClient.Close()

	fmt.Fprintf(os.Stderr, "Ensuring images are available...\n")
	if err := dockerClient.EnsureImage(ctx, imageA); err != nil {
		return fmt.Errorf("preparing image A: %w", err)
	}
	if err := dockerClient.EnsureImage(ctx, imageB); err != nil {
		return fmt.Errorf("preparing image B: %w", err)
	}

	metaA, err := dockerClient.InspectImage(ctx, imageA)
	if err != nil {
		return fmt.Errorf("inspecting image A: %w", err)
	}

	metaB, err := dockerClient.InspectImage(ctx, imageB)
	if err != nil {
		return fmt.Errorf("inspecting image B: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "docker-image-merge-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rootA := filepath.Join(tmpDir, "image-a")
	rootB := filepath.Join(tmpDir, "image-b")

	fmt.Fprintf(os.Stderr, "Extracting filesystem from %s...\n", imageA)
	if err := dockerClient.ExtractFilesystem(ctx, imageA, rootA); err != nil {
		return fmt.Errorf("extracting image A: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Extracting filesystem from %s...\n", imageB)
	if err := dockerClient.ExtractFilesystem(ctx, imageB, rootB); err != nil {
		return fmt.Errorf("extracting image B: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Comparing filesystems...\n")
	diffEngine := merge.NewDiffEngine(rootA, rootB)
	diffResult, err := diffEngine.Run()
	if err != nil {
		return fmt.Errorf("comparing filesystems: %w", err)
	}

	if opts.Verbose {
		printDiffStats(diffResult.Stats)
	}

	if len(diffResult.Conflicts) == 0 {
		fmt.Fprintf(os.Stderr, "No differences found. Images are identical.\n")
		return nil
	}

	conflictCount := 0
	for _, c := range diffResult.Conflicts {
		if c.Kind.NeedsResolution() {
			conflictCount++
		}
	}

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
		fmt.Fprintf(os.Stderr, "Found %d differences (%d need resolution)\n", len(diffResult.Conflicts), conflictCount)
	}

	switch opts.Strategy {
	case flags.StrategyFail:
		fmt.Fprintf(os.Stderr, "\nConflicts found:\n")
		for _, c := range diffResult.Conflicts {
			if c.Kind.NeedsResolution() {
				fmt.Fprintf(os.Stderr, "  - %s\n", c.Summary())
			}
		}
		return fmt.Errorf("%d conflicts found (strategy: fail)", conflictCount)

	case flags.StrategyAutoA:
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
		confirmed, err := tui.Run(diffResult.Conflicts)
		if err != nil {
			return fmt.Errorf("running conflict resolver: %w", err)
		}
		if !confirmed {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "Applying resolutions...\n")
	applyResult, err := merge.ApplyResolutions(rootA, rootB, diffResult.Conflicts, tmpDir)
	if err != nil {
		return fmt.Errorf("applying resolutions: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Merged %d files (A: %d, B: %d, Skipped: %d)\n",
		applyResult.TotalFiles, applyResult.FromA, applyResult.FromB, applyResult.Skipped)

	var metaToUse *docker.ImageMetadata
	if opts.MetaFrom == flags.MetadataFromB {
		metaToUse = metaB
	} else {
		metaToUse = metaA
	}

	fmt.Fprintf(os.Stderr, "Creating output image...\n")

	if opts.Squash {
		imageID, err := dockerClient.ImportSquashed(ctx, applyResult.MergedDir, outputImage, metaToUse, opts.Changes, opts.Message)
		if err != nil {
			return fmt.Errorf("importing merged image: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Created image %s (%s)\n", outputImage, imageID[:12])
	} else {
		imageID, err := dockerClient.BuildLayered(ctx, imageA, applyResult.MergedDir, outputImage, metaToUse, opts.Changes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Layered build failed, falling back to squash: %v\n", err)
			imageID, err = dockerClient.ImportSquashed(ctx, applyResult.MergedDir, outputImage, metaToUse, opts.Changes, opts.Message)
			if err != nil {
				return fmt.Errorf("importing merged image: %w", err)
			}
		}
		fmt.Fprintf(os.Stderr, "Created image %s (%s)\n", outputImage, imageID[:12])
	}

	return nil
}

func validateStrategy(s flags.Strategy) error {
	switch s {
	case flags.StrategyInteractive, flags.StrategyAutoA, flags.StrategyAutoB, flags.StrategyFail:
		return nil
	default:
		return fmt.Errorf("unknown strategy: %s (valid: interactive, auto-a, auto-b, fail)", s)
	}
}

func validateMetadataSource(ms flags.MetadataSource) error {
	switch ms {
	case flags.MetadataFromA, flags.MetadataFromB:
		return nil
	default:
		return fmt.Errorf("unknown metadata source: %s (valid: a, b)", ms)
	}
}

func printDiffStats(stats merge.DiffStats) {
	fmt.Fprintf(os.Stderr, "  Only in A:      %d\n", stats.OnlyA)
	fmt.Fprintf(os.Stderr, "  Only in B:      %d\n", stats.OnlyB)
	fmt.Fprintf(os.Stderr, "  Identical:      %d\n", stats.Same)
	fmt.Fprintf(os.Stderr, "  Conflicts:      %d\n", stats.Conflicts)
	fmt.Fprintf(os.Stderr, "  Type changes:   %d\n", stats.TypeChange)
	fmt.Fprintf(os.Stderr, "  Perm only:      %d\n", stats.PermOnly)
	fmt.Fprintf(os.Stderr, "  Both deleted:   %d\n", stats.BothDeleted)
}
