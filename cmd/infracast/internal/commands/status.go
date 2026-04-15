package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/DaviRain-Su/infracast/internal/state"
)

// newStatusCommand creates the status command
func newStatusCommand() *cobra.Command {
	var (
		env    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show infrastructure status",
		Long: `Show the status of infrastructure resources for the specified environment.

If no environment is specified, shows a summary of all environments.

Examples:
  # Show all environments
  infracast status

  # Show resources for a specific environment
  infracast status --env dev

  # JSON output for CI/CD
  infracast status --output json

  # YAML output
  infracast status --env dev --output yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(env, output)
		},
	}

	cmd.Flags().StringVar(&env, "env", "", "Target environment (omit for all)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: json, yaml (default: table)")

	return cmd
}

// StatusOutput is the structured output for status command
type StatusOutput struct {
	Environments []EnvStatusOutput `json:"environments" yaml:"environments"`
}

// EnvStatusOutput represents a single environment's status
type EnvStatusOutput struct {
	Name      string                 `json:"name" yaml:"name"`
	Total     int                    `json:"total" yaml:"total"`
	Ready     int                    `json:"ready" yaml:"ready"`
	Failed    int                    `json:"failed" yaml:"failed"`
	Resources []ResourceStatusOutput `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// ResourceStatusOutput represents a single resource's status
type ResourceStatusOutput struct {
	Type       string `json:"type" yaml:"type"`
	Name       string `json:"name" yaml:"name"`
	Status     string `json:"status" yaml:"status"`
	ProviderID string `json:"provider_id,omitempty" yaml:"provider_id,omitempty"`
	UpdatedAt  string `json:"updated_at" yaml:"updated_at"`
	ErrorMsg   string `json:"error_msg,omitempty" yaml:"error_msg,omitempty"`
	ErrorHint  string `json:"error_hint,omitempty" yaml:"error_hint,omitempty"`
}

func runStatus(env, output string) error {
	store, err := state.NewStore(defaultDBPath())
	if err != nil {
		return fmt.Errorf("ESTATE001: failed to open state database: %w", err)
	}
	defer store.Close()

	ctx := context.Background()

	if env != "" {
		return showEnvStatus(ctx, store, env, output)
	}
	return showAllStatus(ctx, store, output)
}

func showAllStatus(ctx context.Context, store *state.Store, output string) error {
	envs, err := store.ListEnvironments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		if output == "json" || output == "yaml" {
			return renderOutput(StatusOutput{Environments: []EnvStatusOutput{}}, output)
		}
		fmt.Println("No environments found in state database.")
		fmt.Println("Create one with: infracast env create <name> --provider alicloud --region cn-hangzhou")
		return nil
	}

	statusOutput := StatusOutput{
		Environments: make([]EnvStatusOutput, 0, len(envs)),
	}

	for _, envName := range envs {
		resources, err := store.ListResourcesByEnv(ctx, envName)
		if err != nil {
			statusOutput.Environments = append(statusOutput.Environments, EnvStatusOutput{
				Name: envName,
			})
			continue
		}

		envStatus := buildEnvStatus(envName, resources, false)
		statusOutput.Environments = append(statusOutput.Environments, envStatus)
	}

	if output == "json" || output == "yaml" {
		return renderOutput(statusOutput, output)
	}

	// Default table output
	fmt.Println()
	color.Cyan("Infrastructure Status")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ENVIRONMENT\tRESOURCES\tREADY\tFAILED")
	fmt.Fprintln(w, "-----------\t---------\t-----\t------")

	for _, es := range statusOutput.Environments {
		failedStr := fmt.Sprintf("%d", es.Failed)
		if es.Failed > 0 {
			failedStr = color.RedString("%d", es.Failed)
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", es.Name, es.Total, es.Ready, failedStr)
	}

	w.Flush()
	fmt.Println()
	fmt.Println("Use 'infracast status --env <name>' for resource details")
	return nil
}

func showEnvStatus(ctx context.Context, store *state.Store, env, output string) error {
	resources, err := store.ListResourcesByEnv(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to list resources for %s: %w", env, err)
	}

	envStatus := buildEnvStatus(env, resources, true)

	if output == "json" || output == "yaml" {
		return renderOutput(StatusOutput{
			Environments: []EnvStatusOutput{envStatus},
		}, output)
	}

	// Default table output
	fmt.Println()
	color.Cyan("Environment: %s", env)
	fmt.Println()

	if len(envStatus.Resources) == 0 {
		fmt.Println("No resources found.")
		fmt.Printf("Run 'infracast provision --env %s' to create resources.\n", env)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tNAME\tSTATUS\tPROVIDER ID\tUPDATED")
	fmt.Fprintln(w, "----\t----\t------\t-----------\t-------")

	for _, r := range envStatus.Resources {
		statusStr := r.Status
		switch r.Status {
		case "ready", "created":
			statusStr = color.GreenString(r.Status)
		case "failed", "error":
			statusStr = color.RedString(r.Status)
		default:
			statusStr = color.YellowString(r.Status)
		}

		providerID := r.ProviderID
		if providerID == "" {
			providerID = "-"
		}
		if len(providerID) > 30 {
			providerID = providerID[:27] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.Type,
			r.Name,
			statusStr,
			providerID,
			r.UpdatedAt,
		)
	}

	w.Flush()

	// Show error details for failed resources
	var failedResources []ResourceStatusOutput
	for _, r := range envStatus.Resources {
		if r.Status == "failed" || r.Status == "error" {
			failedResources = append(failedResources, r)
		}
	}
	if len(failedResources) > 0 {
		fmt.Println()
		color.Red("Failed Resources:")
		for _, r := range failedResources {
			fmt.Printf("  %s/%s: %s\n", r.Type, r.Name, r.ErrorMsg)
			if r.ErrorHint != "" {
				color.Yellow("    Hint: %s", r.ErrorHint)
			}
		}
	}

	fmt.Println()
	return nil
}

// buildEnvStatus constructs EnvStatusOutput from raw resources
func buildEnvStatus(envName string, resources []*state.InfraResource, includeResources bool) EnvStatusOutput {
	es := EnvStatusOutput{Name: envName}

	for _, r := range resources {
		if r.ResourceName == "_env_meta" {
			continue
		}
		es.Total++
		switch r.Status {
		case "ready", "created":
			es.Ready++
		case "failed", "error":
			es.Failed++
		}

		if includeResources {
			ro := ResourceStatusOutput{
				Type:       r.ResourceType,
				Name:       r.ResourceName,
				Status:     r.Status,
				ProviderID: r.ProviderResourceID,
				UpdatedAt:  r.UpdatedAt.Format("2006-01-02 15:04"),
			}
			if r.ErrorMsg != "" {
				ro.ErrorMsg = r.ErrorMsg
				ro.ErrorHint = statusErrorHint(r.ErrorMsg)
			}
			es.Resources = append(es.Resources, ro)
		}
	}

	return es
}

// statusErrorHint returns an actionable hint for known error patterns
func statusErrorHint(errMsg string) string {
	lower := strings.ToLower(errMsg)
	hints := []struct {
		pattern string
		hint    string
	}{
		{"eprov001", "Check ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY environment variables"},
		{"eprov003", "Verify Alicloud credentials and region configuration"},
		{"edeploy076", "Resource may still be initializing — retry after a few minutes"},
		{"notenoughbalance", "Top up your Alicloud account balance or try spot instances"},
		{"timeout", "Check network connectivity and retry"},
		{"unauthorized", "Cloud credentials may be expired — re-authenticate with your provider"},
		{"quota", "Resource quota exceeded — request a quota increase from Alicloud console"},
	}

	for _, h := range hints {
		if strings.Contains(lower, h.pattern) {
			return h.hint
		}
	}
	return ""
}

// renderOutput renders StatusOutput as JSON or YAML
func renderOutput(data StatusOutput, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		err := enc.Encode(data)
		enc.Close()
		return err
	default:
		return fmt.Errorf("unsupported output format: %s (use json or yaml)", format)
	}
}
