package flags

import "github.com/spf13/cobra"

type Strategy string

const (
	StrategyInteractive Strategy = "interactive"
	StrategyAutoA       Strategy = "auto-a"
	StrategyAutoB       Strategy = "auto-b"
	StrategyFail        Strategy = "fail"
)

type MetadataSource string

const (
	MetadataFromA MetadataSource = "a"
	MetadataFromB MetadataSource = "b"
)

type Options struct {
	Strategy   Strategy
	Message    string
	Changes    []string
	Platform   string
	MetaFrom   MetadataSource
	Squash     bool
	Verbose    bool
}

func AddFlags(cmd *cobra.Command, opts *Options) {
	cmd.Flags().StringVarP((*string)(&opts.Strategy), "strategy", "s", string(StrategyInteractive),
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
