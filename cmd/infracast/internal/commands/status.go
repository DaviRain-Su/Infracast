package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newStatusCommand creates the status command
func newStatusCommand() *cobra.Command {
	var env string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show infrastructure status",
		Long:  "Show the status of infrastructure resources for the specified environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status query
			fmt.Printf("Showing infrastructure status for environment: %s\n", env)
			return nil
		},
	}

	cmd.Flags().StringVar(&env, "env", "dev", "Target environment")

	return cmd
}
