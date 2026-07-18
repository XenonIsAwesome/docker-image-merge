package flags

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestStrategyConstants(t *testing.T) {
	if StrategyInteractive != "interactive" {
		t.Errorf("StrategyInteractive = %q, want %q", StrategyInteractive, "interactive")
	}
	if StrategyAutoA != "auto-a" {
		t.Errorf("StrategyAutoA = %q, want %q", StrategyAutoA, "auto-a")
	}
	if StrategyAutoB != "auto-b" {
		t.Errorf("StrategyAutoB = %q, want %q", StrategyAutoB, "auto-b")
	}
	if StrategyFail != "fail" {
		t.Errorf("StrategyFail = %q, want %q", StrategyFail, "fail")
	}
}

func TestMetadataSourceConstants(t *testing.T) {
	if MetadataFromA != "a" {
		t.Errorf("MetadataFromA = %q, want %q", MetadataFromA, "a")
	}
	if MetadataFromB != "b" {
		t.Errorf("MetadataFromB = %q, want %q", MetadataFromB, "b")
	}
}

func TestAddFlags_Defaults(t *testing.T) {
	cmd := &cobra.Command{}
	opts := &Options{}
	AddFlags(cmd, opts)

	// Check defaults.
	if opts.Strategy != StrategyInteractive {
		t.Errorf("default Strategy = %q, want %q", opts.Strategy, StrategyInteractive)
	}
	if opts.MetaFrom != MetadataFromA {
		t.Errorf("default MetaFrom = %q, want %q", opts.MetaFrom, MetadataFromA)
	}
	if opts.Squash != false {
		t.Errorf("default Squash = %v, want false", opts.Squash)
	}
	if opts.Verbose != false {
		t.Errorf("default Verbose = %v, want false", opts.Verbose)
	}
	if opts.Message != "" {
		t.Errorf("default Message = %q, want empty", opts.Message)
	}
	if opts.Platform != "" {
		t.Errorf("default Platform = %q, want empty", opts.Platform)
	}
}

func TestAddFlags_SetValues(t *testing.T) {
	cmd := &cobra.Command{}
	opts := &Options{}
	AddFlags(cmd, opts)

	_ = cmd.Flags().Set("strategy", "auto-b")
	_ = cmd.Flags().Set("metadata-from", "b")
	_ = cmd.Flags().Set("squash", "true")
	_ = cmd.Flags().Set("verbose", "true")
	_ = cmd.Flags().Set("message", "test message")
	_ = cmd.Flags().Set("platform", "linux/arm64")
	_ = cmd.Flags().Set("change", "ENV FOO=bar")

	if opts.Strategy != StrategyAutoB {
		t.Errorf("Strategy = %q, want %q", opts.Strategy, StrategyAutoB)
	}
	if opts.MetaFrom != MetadataFromB {
		t.Errorf("MetaFrom = %q, want %q", opts.MetaFrom, MetadataFromB)
	}
	if !opts.Squash {
		t.Error("Squash should be true")
	}
	if !opts.Verbose {
		t.Error("Verbose should be true")
	}
	if opts.Message != "test message" {
		t.Errorf("Message = %q, want %q", opts.Message, "test message")
	}
	if opts.Platform != "linux/arm64" {
		t.Errorf("Platform = %q, want %q", opts.Platform, "linux/arm64")
	}
	if len(opts.Changes) != 1 || opts.Changes[0] != "ENV FOO=bar" {
		t.Errorf("Changes = %v, want [ENV FOO=bar]", opts.Changes)
	}
}

func TestOptionsStruct_Fields(t *testing.T) {
	opts := &Options{
		Strategy: StrategyAutoA,
		Message:  "custom message",
		Changes:  []string{"EXPOSE 8080"},
		Platform: "linux/amd64",
		MetaFrom: MetadataFromB,
		Squash:   true,
		Verbose:  true,
	}

	if opts.Strategy != StrategyAutoA {
		t.Error("Strategy mismatch")
	}
	if opts.Message != "custom message" {
		t.Error("Message mismatch")
	}
	if len(opts.Changes) != 1 {
		t.Error("Changes mismatch")
	}
	if opts.Platform != "linux/amd64" {
		t.Error("Platform mismatch")
	}
	if opts.MetaFrom != MetadataFromB {
		t.Error("MetaFrom mismatch")
	}
	if !opts.Squash {
		t.Error("Squash mismatch")
	}
	if !opts.Verbose {
		t.Error("Verbose mismatch")
	}
}
