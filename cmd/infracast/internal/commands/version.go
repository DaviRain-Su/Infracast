package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCommand creates the version command
func newVersionCommand(version, commit, buildTime string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print detailed version information about infracast CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("infracast version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  build time: %s\n", buildTime)
		},
	}
}
