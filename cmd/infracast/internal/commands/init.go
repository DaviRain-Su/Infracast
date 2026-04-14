package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newInitCommand creates the init command
func newInitCommand() *cobra.Command {
	var provider, region string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new infracast project",
		Long:  "Initialize a new infracast project with configuration file and directory structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement project initialization
			fmt.Printf("Initializing infracast project...\n")
			fmt.Printf("Provider: %s\n", provider)
			fmt.Printf("Region: %s\n", region)
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cloud provider (required)")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region (required)")
	cmd.MarkFlagRequired("provider")
	cmd.MarkFlagRequired("region")

	return cmd
}
