package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// newEnvCommand creates the env management command
func newEnvCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage deployment environments",
		Long: `Manage deployment environments for your Infracast project.

Environments represent isolated deployment targets (dev, staging, production).
Each environment can have different resource configurations and credentials.

Examples:
  # List all environments
  infracast env list

  # Show environment details
  infracast env show dev

  # Create a new environment
  infracast env create staging --provider alicloud --region cn-shanghai

  # Set default environment
  infracast env use production

  # Delete an environment
  infracast env delete dev`,
	}

	cmd.AddCommand(newEnvListCommand())
	cmd.AddCommand(newEnvShowCommand())
	cmd.AddCommand(newEnvCreateCommand())
	cmd.AddCommand(newEnvUseCommand())
	cmd.AddCommand(newEnvDeleteCommand())

	return cmd
}

// newEnvListCommand creates the env list command
func newEnvListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvList()
		},
	}
}

// newEnvShowCommand creates the env show command
func newEnvShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show [env-name]",
		Short: "Show environment details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvShow(args[0])
		},
	}
}

// newEnvCreateCommand creates the env create command
func newEnvCreateCommand() *cobra.Command {
	var (
		provider string
		region   string
	)

	cmd := &cobra.Command{
		Use:   "create [env-name]",
		Short: "Create a new environment",
		Long: `Create a new deployment environment.

The environment name should be descriptive (e.g., dev, staging, prod-eu).
Environment names must be unique within the project.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvCreate(args[0], provider, region)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cloud provider (required)")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region (required)")
	cmd.MarkFlagRequired("provider")
	cmd.MarkFlagRequired("region")

	return cmd
}

// newEnvUseCommand creates the env use command
func newEnvUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use [env-name]",
		Short: "Set default environment",
		Long:  "Set the default environment for subsequent commands.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvUse(args[0])
		},
	}
}

// newEnvDeleteCommand creates the env delete command
func newEnvDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [env-name]",
		Short: "Delete an environment",
		Long:  "Delete an environment and its configuration. This does not destroy provisioned resources.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvDelete(args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

// Environment represents a deployment environment
type Environment struct {
	Name        string
	Provider    string
	Region      string
	Description string
	Default     bool
	Resources   []EnvResource
}

// EnvResource represents a resource in an environment
type EnvResource struct {
	Type   string
	Name   string
	Status string
}

// runEnvList lists all environments
func runEnvList() error {
	envs, err := loadEnvironments()
	if err != nil {
		return fmt.Errorf("ECFG001: failed to load environments: %w", err)
	}

	if len(envs) == 0 {
		fmt.Println("No environments found.")
		fmt.Println("Create one with: infracast env create <name>")
		return nil
	}

	// Print header
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ENVIRONMENT\tPROVIDER\tREGION\tDEFAULT\tRESOURCES")
	fmt.Fprintln(w, "-----------\t--------\t------\t-------\t---------")

	// Print environments
	for _, env := range envs {
		defaultMarker := ""
		if env.Default {
			defaultMarker = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			env.Name,
			env.Provider,
			env.Region,
			defaultMarker,
			len(env.Resources),
		)
	}

	w.Flush()
	fmt.Println()
	fmt.Println("Use 'infracast env show <name>' for details")
	return nil
}

// runEnvShow shows environment details
func runEnvShow(name string) error {
	env, err := loadEnvironment(name)
	if err != nil {
		return fmt.Errorf("ECFG002: environment not found: %s", name)
	}

	// Print header
	fmt.Println()
	if env.Default {
		color.Green("Environment: %s (default)\n", env.Name)
	} else {
		color.Cyan("Environment: %s\n", env.Name)
	}
	fmt.Println()

	// Print details
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Provider:\t%s\n", env.Provider)
	fmt.Fprintf(w, "Region:\t%s\n", env.Region)
	if env.Description != "" {
		fmt.Fprintf(w, "Description:\t%s\n", env.Description)
	}
	w.Flush()

	// Print resources
	if len(env.Resources) > 0 {
		fmt.Println()
		fmt.Println("Resources:")
		for _, res := range env.Resources {
			statusColor := color.GreenString
			if res.Status != "ready" {
				statusColor = color.YellowString
			}
			fmt.Printf("  • %s/%s: %s\n", res.Type, res.Name, statusColor(res.Status))
		}
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Printf("  Deploy:    infracast deploy --env %s\n", env.Name)
	fmt.Printf("  Provision: infracast provision --env %s\n", env.Name)
	fmt.Printf("  Status:    infracast status --env %s\n", env.Name)
	fmt.Println()

	return nil
}

// runEnvCreate creates a new environment
func runEnvCreate(name, provider, region string) error {
	// Validate name
	if err := validateEnvName(name); err != nil {
		return fmt.Errorf("ECFG003: invalid environment name: %w", err)
	}

	// Check if exists
	if envExists(name) {
		return fmt.Errorf("ECFG004: environment already exists: %s", name)
	}

	// Validate provider
	if !isValidProvider(provider) {
		return fmt.Errorf("ECFG005: unsupported provider: %s", provider)
	}

	// Create environment
	env := Environment{
		Name:     name,
		Provider: provider,
		Region:   region,
	}

	if err := saveEnvironment(env); err != nil {
		return fmt.Errorf("ECFG006: failed to save environment: %w", err)
	}

	color.Green("✓ Environment '%s' created successfully", name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Configure resources in infracast.yaml\n")
	fmt.Printf("  2. Run 'infracast provision --env %s' to create infrastructure\n", name)
	fmt.Printf("  3. Run 'infracast deploy --env %s' to deploy your app\n", name)

	return nil
}

// runEnvUse sets the default environment
func runEnvUse(name string) error {
	if !envExists(name) {
		return fmt.Errorf("ECFG002: environment not found: %s", name)
	}

	if err := setDefaultEnvironment(name); err != nil {
		return fmt.Errorf("ECFG007: failed to set default environment: %w", err)
	}

	color.Green("✓ Default environment set to '%s'", name)
	return nil
}

// runEnvDelete deletes an environment
func runEnvDelete(name string, force bool) error {
	if !envExists(name) {
		return fmt.Errorf("ECFG002: environment not found: %s", name)
	}

	// Confirm deletion
	if !force {
		fmt.Printf("Are you sure you want to delete environment '%s'? [y/N]: ", name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := deleteEnvironment(name); err != nil {
		return fmt.Errorf("ECFG008: failed to delete environment: %w", err)
	}

	color.Green("✓ Environment '%s' deleted", name)
	fmt.Println("Note: Provisioned resources were not destroyed. Use 'infracast destroy' to clean up.")

	return nil
}

// Helper functions (placeholders for actual implementation)

func loadEnvironments() ([]Environment, error) {
	// TODO: Load from infracast.yaml or state store
	return []Environment{
		{
			Name:        "dev",
			Provider:    "alicloud",
			Region:      "cn-hangzhou",
			Description: "Development environment",
			Default:     true,
			Resources: []EnvResource{
				{Type: "sql_server", Name: "main", Status: "ready"},
				{Type: "redis", Name: "cache", Status: "ready"},
			},
		},
		{
			Name:        "staging",
			Provider:    "alicloud",
			Region:      "cn-shanghai",
			Description: "Staging environment",
			Default:     false,
			Resources: []EnvResource{
				{Type: "sql_server", Name: "main", Status: "ready"},
			},
		},
	}, nil
}

func loadEnvironment(name string) (Environment, error) {
	envs, err := loadEnvironments()
	if err != nil {
		return Environment{}, err
	}

	for _, env := range envs {
		if env.Name == name {
			return env, nil
		}
	}

	return Environment{}, fmt.Errorf("not found")
}

func envExists(name string) bool {
	_, err := loadEnvironment(name)
	return err == nil
}

func saveEnvironment(env Environment) error {
	// TODO: Save to infracast.yaml or state store
	return nil
}

func deleteEnvironment(name string) error {
	// TODO: Remove from infracast.yaml or state store
	return nil
}

func setDefaultEnvironment(name string) error {
	// TODO: Update default in configuration
	return nil
}

func validateEnvName(name string) error {
	if name == "" {
		return fmt.Errorf("environment name cannot be empty")
	}

	if len(name) > 30 {
		return fmt.Errorf("environment name must be 30 characters or less")
	}

	// Allow lowercase letters, numbers, hyphens
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return fmt.Errorf("environment name can only contain lowercase letters, numbers, and hyphens")
		}
	}

	return nil
}

func isValidProvider(provider string) bool {
	validProviders := []string{"alicloud", "aws", "gcp"}
	for _, p := range validProviders {
		if p == provider {
			return true
		}
	}
	return false
}
