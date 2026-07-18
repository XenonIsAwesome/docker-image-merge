// Package flags defines CLI flag types and validation for docker-image-merge.
//
// Flags are shared between the Cobra command definition and the merge pipeline.
// The Options struct holds all parsed flag values and is passed through the
// entire execution flow.
package flags

import "github.com/spf13/cobra"

// Strategy defines how file conflicts between two images are resolved.
type Strategy string

const (
	// StrategyInteractive launches a TUI for per-file conflict resolution (default).
	StrategyInteractive Strategy = "interactive"

	// StrategyAutoA automatically takes image A's version for every conflict.
	StrategyAutoA Strategy = "auto-a"

	// StrategyAutoB automatically takes image B's version for every conflict.
	StrategyAutoB Strategy = "auto-b"

	// StrategyFail exits with an error if any conflicts are found.
	StrategyFail Strategy = "fail"
)

// MetadataSource indicates which image's configuration metadata to inherit.
type MetadataSource string

const (
	// MetadataFromA inherits ENV, CMD, ENTRYPOINT, etc. from image A (default).
	MetadataFromA MetadataSource = "a"

	// MetadataFromB inherits ENV, CMD, ENTRYPOINT, etc. from image B.
	MetadataFromB MetadataSource = "b"
)

// Options holds all parsed CLI flags for a merge invocation.
type Options struct {
	// Strategy selects the conflict resolution mode.
	Strategy Strategy

	// Message is the commit message stored in the output image's history.
	Message string

	// Changes holds Dockerfile-style instructions applied during import
	// (e.g. "ENV FOO=bar"). These are passed to docker import --change.
	Changes []string

	// Platform optionally constrains the target platform (e.g. "linux/amd64").
	Platform string

	// MetaFrom selects which image's runtime config to inherit.
	MetaFrom MetadataSource

	// Squash forces a single-layer output via docker import, discarding
	// the original layer structure. When false (default), the tool attempts
	// a layered build that preserves image A's layers.
	Squash bool

	// Verbose enables detailed diff statistics on stderr.
	Verbose bool
}

// AddFlags registers all CLI flags on the given Cobra command and binds them
// to the corresponding fields in opts.
func AddFlags(cmd *cobra.Command, opts *Options) {
	cmd.Flags().StringVarP((*string)(&opts.Strategy), "strategy", "s",
		string(StrategyInteractive),
		"Conflict resolution strategy: interactive, auto-a, auto-b, fail")
	cmd.Flags().StringVarP(&opts.Message, "message", "m", "",
		"Commit message for the new image")
	cmd.Flags().StringArrayVar(&opts.Changes, "change", nil,
		"Dockerfile instructions to apply (like docker import --change)")
	cmd.Flags().StringVar(&opts.Platform, "platform", "",
		"Target platform (e.g., linux/amd64)")
	cmd.Flags().StringVar((*string)(&opts.MetaFrom), "metadata-from", string(MetadataFromA),
		"Which image's metadata to inherit: a, b")
	cmd.Flags().BoolVar(&opts.Squash, "squash", false,
		"Force single-layer output (loses layer history)")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false,
		"Verbose output")
}
