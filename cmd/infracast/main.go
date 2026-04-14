package main

import (
	"fmt"
	"os"

	_ "github.com/DaviRain-Su/infracast/providers/alicloud"

	"github.com/DaviRain-Su/infracast/cmd/infracast/internal/commands"
)

var (
	// Version is the build version, set by ldflags
	Version = "dev"
	// Commit is the git commit hash, set by ldflags
	Commit = "unknown"
	// BuildTime is the build timestamp, set by ldflags
	BuildTime = "unknown"
)

func main() {
	rootCmd := commands.NewRootCommand(Version, Commit, BuildTime)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
