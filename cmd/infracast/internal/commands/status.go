package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/DaviRain-Su/infracast/internal/state"
)

// newStatusCommand creates the status command
func newStatusCommand() *cobra.Command {
	var env string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show infrastructure status",
		Long: `Show the status of infrastructure resources for the specified environment.

If no environment is specified, shows a summary of all environments.

Examples:
  # Show all environments
  infracast status

  # Show resources for a specific environment
  infracast status --env dev`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(env)
		},
	}

	cmd.Flags().StringVar(&env, "env", "", "Target environment (omit for all)")

	return cmd
}

func runStatus(env string) error {
	store, err := state.NewStore(defaultDBPath())
	if err != nil {
		return fmt.Errorf("ESTATE001: failed to open state database: %w", err)
	}

	ctx := context.Background()

	if env != "" {
		return showEnvStatus(ctx, store, env)
	}
	return showAllStatus(ctx, store)
}

func showAllStatus(ctx context.Context, store *state.Store) error {
	envs, err := store.ListEnvironments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		fmt.Println("No environments found in state database.")
		fmt.Println("Create one with: infracast env create <name> --provider alicloud --region cn-hangzhou")
		return nil
	}

	fmt.Println()
	color.Cyan("Infrastructure Status")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ENVIRONMENT\tRESOURCES\tREADY\tFAILED")
	fmt.Fprintln(w, "-----------\t---------\t-----\t------")

	for _, envName := range envs {
		resources, err := store.ListResourcesByEnv(ctx, envName)
		if err != nil {
			fmt.Fprintf(w, "%s\t-\t-\t(error: %v)\n", envName, err)
			continue
		}

		total, ready, failed := 0, 0, 0
		for _, r := range resources {
			if r.ResourceName == "_env_meta" {
				continue
			}
			total++
			switch r.Status {
			case "ready", "created":
				ready++
			case "failed", "error":
				failed++
			}
		}

		failedStr := fmt.Sprintf("%d", failed)
		if failed > 0 {
			failedStr = color.RedString("%d", failed)
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", envName, total, ready, failedStr)
	}

	w.Flush()
	fmt.Println()
	fmt.Println("Use 'infracast status --env <name>' for resource details")
	return nil
}

func showEnvStatus(ctx context.Context, store *state.Store, env string) error {
	resources, err := store.ListResourcesByEnv(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to list resources for %s: %w", env, err)
	}

	fmt.Println()
	color.Cyan("Environment: %s", env)
	fmt.Println()

	// Filter out internal _env_meta records
	var visible []*state.InfraResource
	for _, r := range resources {
		if r.ResourceName != "_env_meta" {
			visible = append(visible, r)
		}
	}

	if len(visible) == 0 {
		fmt.Println("No resources found.")
		fmt.Printf("Run 'infracast provision --env %s' to create resources.\n", env)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tNAME\tSTATUS\tPROVIDER ID\tUPDATED")
	fmt.Fprintln(w, "----\t----\t------\t-----------\t-------")

	for _, r := range visible {
		statusStr := r.Status
		switch r.Status {
		case "ready", "created":
			statusStr = color.GreenString(r.Status)
		case "failed", "error":
			statusStr = color.RedString(r.Status)
		default:
			statusStr = color.YellowString(r.Status)
		}

		providerID := r.ProviderResourceID
		if providerID == "" {
			providerID = "-"
		}
		if len(providerID) > 30 {
			providerID = providerID[:27] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.ResourceType,
			r.ResourceName,
			statusStr,
			providerID,
			r.UpdatedAt.Format("2006-01-02 15:04"),
		)
	}

	w.Flush()
	fmt.Println()
	return nil
}
