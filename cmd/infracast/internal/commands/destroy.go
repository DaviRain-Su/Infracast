package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newDestroyCommand creates the destroy command
func newDestroyCommand() *cobra.Command {
	var (
		env   string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy infrastructure resources",
		Long:  "Destroy all infrastructure resources for the specified environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement infrastructure destruction
			fmt.Printf("Destroying infrastructure for environment: %s\n", env)
			
			if !force {
				fmt.Println("This will destroy all resources. Use --force to skip confirmation.")
				// TODO: Add interactive confirmation
			}
			
			return nil
		},
	}

	cmd.Flags().StringVar(&env, "env", "dev", "Target environment")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}
