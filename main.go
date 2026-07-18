package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/XenonIsAwesome/docker-image-merge/cmd"
)

type pluginMetadata struct {
	SchemaVersion    string `json:"SchemaVersion"`
	Vendor           string `json:"Vendor"`
	Description      string `json:"Description,omitempty"`
	ShortDescription string `json:"ShortDescription,omitempty"`
	URL              string `json:"URL,omitempty"`
}

func main() {
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

	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
