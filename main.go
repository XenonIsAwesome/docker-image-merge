// Package main provides the entry point for the docker-image-merge CLI plugin.
//
// It supports two invocation modes:
//  1. Docker CLI plugin mode: When invoked as "docker-image-merge", it registers
//     as a Docker CLI plugin via the docker-cli-plugin-metadata subcommand.
//  2. Standalone mode: When invoked directly as "docker-image-merge", it runs
//     the merge command via Cobra.
//
// Usage:
//
//	docker image-merge <image-a> <image-b> <output-image>
//	docker-image-merge <image-a> <image-b> <output-image>
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/XenonIsAwesome/docker-image-merge/cmd"
)

// pluginMetadata defines the JSON schema returned by the Docker CLI plugin
// metadata subcommand. This is required for Docker CLI plugin registration.
type pluginMetadata struct {
	// SchemaVersion is the plugin schema version (currently "0.1.0").
	SchemaVersion string `json:"SchemaVersion"`

	// Vendor is the plugin vendor/author identifier.
	Vendor string `json:"Vendor"`

	// Description is a long description of the plugin's purpose.
	Description string `json:"Description,omitempty"`

	// ShortDescription is a brief one-line description for help output.
	ShortDescription string `json:"ShortDescription,omitempty"`

	// URL is an optional link to the plugin's homepage or repository.
	URL string `json:"URL,omitempty"`
}

// main is the program entry point. It checks for the Docker CLI plugin metadata
// subcommand first, and falls back to the Cobra command root otherwise.
func main() {
	// Docker CLI plugins must respond to this subcommand with JSON metadata.
	// See: https://docs.docker.com/engine/reference/commandline/cli_plugins/
	if len(os.Args) > 1 && os.Args[1] == "docker-cli-plugin-metadata" {
		meta := pluginMetadata{
			SchemaVersion:    "0.1.0",
			Vendor:           "docker-image-merge",
			Description:      "Merge the filesystems of two Docker images",
			ShortDescription: "Merge Docker image filesystems",
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "     ")
		enc.Encode(meta)
		return
	}

	// Normal execution: build and run the Cobra command tree.
	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
