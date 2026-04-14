package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// newInitCommand creates the init command with interactive UX
func newInitCommand() *cobra.Command {
	var (
		provider string
		region   string
		appName  string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "init [app-name]",
		Short: "Initialize a new infracast project",
		Long: `Initialize a new infracast project with configuration file and directory structure.

This command creates:
- infracast.yaml: Project configuration
- .infra/: Infrastructure state directory
- examples/: Sample Encore application

Examples:
  # Interactive initialization
  infracast init my-app

  # Non-interactive with flags
  infracast init my-app --provider alicloud --region cn-hangzhou --yes

  # List available providers
  infracast init --list-providers`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get app name from args or prompt
			if len(args) > 0 {
				appName = args[0]
			}

			return runInit(InitOptions{
				AppName:        appName,
				Provider:       provider,
				Region:         region,
				NonInteractive: nonInteractive,
			})
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cloud provider (alicloud, aws, gcp)")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region (e.g., cn-hangzhou, us-east-1)")
	cmd.Flags().BoolVarP(&nonInteractive, "yes", "y", false, "Non-interactive mode (use defaults)")
	cmd.Flags().Bool("list-providers", false, "List available cloud providers")

	return cmd
}

// InitOptions contains initialization options
type InitOptions struct {
	AppName        string
	Provider       string
	Region         string
	NonInteractive bool
}

// ProviderInfo describes a supported cloud provider
type ProviderInfo struct {
	Name        string
	DisplayName string
	Regions     []string
}

// SupportedProviders lists all supported cloud providers
var SupportedProviders = []ProviderInfo{
	{
		Name:        "alicloud",
		DisplayName: "Alibaba Cloud",
		Regions:     []string{"cn-hangzhou", "cn-beijing", "cn-shanghai", "cn-shenzhen"},
	},
	{
		Name:        "aws",
		DisplayName: "Amazon Web Services",
		Regions:     []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"},
	},
	{
		Name:        "gcp",
		DisplayName: "Google Cloud Platform",
		Regions:     []string{"us-central1", "us-east1", "europe-west1", "asia-east1"},
	},
}

// runInit executes the initialization workflow
func runInit(opts InitOptions) error {
	reader := bufio.NewReader(os.Stdin)

	// Print welcome banner
	printInitBanner()

	// Get app name
	if opts.AppName == "" {
		if opts.NonInteractive {
			return fmt.Errorf("ECFG001: app name is required (provide as argument or use interactive mode)")
		}
		name, err := promptString(reader, "Application name", "my-app")
		if err != nil {
			return err
		}
		opts.AppName = name
	}

	// Validate app name
	if err := validateAppName(opts.AppName); err != nil {
		return fmt.Errorf("ECFG002: invalid app name: %w", err)
	}

	// Get provider
	if opts.Provider == "" {
		if opts.NonInteractive {
			opts.Provider = "alicloud" // default
		} else {
			provider, err := promptProvider(reader)
			if err != nil {
				return err
			}
			opts.Provider = provider
		}
	}

	// Validate provider
	providerInfo, err := getProviderInfo(opts.Provider)
	if err != nil {
		return fmt.Errorf("ECFG003: %w", err)
	}

	// Get region
	if opts.Region == "" {
		if opts.NonInteractive {
			opts.Region = providerInfo.Regions[0] // default to first region
		} else {
			region, err := promptRegion(reader, providerInfo)
			if err != nil {
				return err
			}
			opts.Region = region
		}
	}

	// Show summary and confirm
	if !opts.NonInteractive {
		printInitSummary(opts)
		confirmed, err := promptConfirm(reader, "Create project with these settings?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Create project structure
	if err := createProjectStructure(opts); err != nil {
		return fmt.Errorf("ECFG004: failed to create project: %w", err)
	}

	// Print success message
	printInitSuccess(opts)

	return nil
}

// printInitBanner prints the welcome banner
func printInitBanner() {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Println("┌─────────────────────────────────────┐")
	cyan.Println("│  Welcome to Infracast               │")
	cyan.Println("│  Infrastructure for Encore Apps     │")
	cyan.Println("└─────────────────────────────────────┘")
	fmt.Println()
}

// printInitSummary prints the configuration summary
func printInitSummary(opts InitOptions) {
	fmt.Println()
	color.Yellow("Configuration Summary:")
	fmt.Printf("  Application: %s\n", color.GreenString(opts.AppName))
	fmt.Printf("  Provider:    %s\n", color.GreenString(opts.Provider))
	fmt.Printf("  Region:      %s\n", color.GreenString(opts.Region))
	fmt.Println()
}

// printInitSuccess prints the success message with next steps
func printInitSuccess(opts InitOptions) {
	fmt.Println()
	color.Green("✓ Project initialized successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. cd %s\n", opts.AppName)
	fmt.Println("  2. Edit infracast.yaml to configure your resources")
	fmt.Println("  3. Run 'infracast provision' to create infrastructure")
	fmt.Println("  4. Run 'infracast deploy' to deploy your application")
	fmt.Println()
	fmt.Println("Documentation:")
	fmt.Println("  https://github.com/DaviRain-Su/Infracast/docs")
}

// promptString prompts for a string value with a default
func promptString(reader *bufio.Reader, prompt, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, color.CyanString(defaultValue))
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

// promptProvider prompts for cloud provider selection
func promptProvider(reader *bufio.Reader) (string, error) {
	fmt.Println()
	color.Yellow("Select cloud provider:")
	for i, p := range SupportedProviders {
		fmt.Printf("  %d. %s (%s)\n", i+1, p.DisplayName, p.Name)
	}
	fmt.Println()

	choice, err := promptString(reader, "Provider number or name", "1")
	if err != nil {
		return "", err
	}

	// Try to parse as number
	var index int
	if _, err := fmt.Sscanf(choice, "%d", &index); err == nil {
		if index >= 1 && index <= len(SupportedProviders) {
			return SupportedProviders[index-1].Name, nil
		}
	}

	// Try to match by name
	choice = strings.ToLower(choice)
	for _, p := range SupportedProviders {
		if p.Name == choice {
			return p.Name, nil
		}
	}

	return "", fmt.Errorf("invalid provider: %s", choice)
}

// promptRegion prompts for region selection
func promptRegion(reader *bufio.Reader, provider ProviderInfo) (string, error) {
	fmt.Println()
	color.Yellow("Select region for %s:", provider.DisplayName)
	for i, r := range provider.Regions {
		fmt.Printf("  %d. %s\n", i+1, r)
	}
	fmt.Println()

	choice, err := promptString(reader, "Region number or name", "1")
	if err != nil {
		return "", err
	}

	// Try to parse as number
	var index int
	if _, err := fmt.Sscanf(choice, "%d", &index); err == nil {
		if index >= 1 && index <= len(provider.Regions) {
			return provider.Regions[index-1], nil
		}
	}

	// Validate region name
	for _, r := range provider.Regions {
		if r == choice {
			return r, nil
		}
	}

	return "", fmt.Errorf("invalid region: %s", choice)
}

// promptConfirm prompts for yes/no confirmation
func promptConfirm(reader *bufio.Reader, prompt string) (bool, error) {
	response, err := promptString(reader, prompt+" [Y/n]", "Y")
	if err != nil {
		return false, err
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes" || response == "", nil
}

// validateAppName validates the application name
func validateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	if len(name) > 50 {
		return fmt.Errorf("app name must be 50 characters or less")
	}

	// Allow letters, numbers, hyphens, underscores
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("app name can only contain letters, numbers, hyphens, and underscores")
		}
	}

	return nil
}

// getProviderInfo returns provider info by name
func getProviderInfo(name string) (ProviderInfo, error) {
	name = strings.ToLower(name)
	for _, p := range SupportedProviders {
		if p.Name == name {
			return p, nil
		}
	}
	return ProviderInfo{}, fmt.Errorf("unsupported provider: %s", name)
}

// createProjectStructure creates the project directory structure
func createProjectStructure(opts InitOptions) error {
	// Create project directory
	if err := os.MkdirAll(opts.AppName, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create .infra directory
	infraDir := filepath.Join(opts.AppName, ".infra")
	if err := os.MkdirAll(infraDir, 0755); err != nil {
		return fmt.Errorf("failed to create .infra directory: %w", err)
	}

	// Create infracast.yaml
	configContent := generateConfigContent(opts)
	configPath := filepath.Join(opts.AppName, "infracast.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create .gitignore
	gitignoreContent := `# Infracast
.infra/
*.log
.env
.env.local
`
	gitignorePath := filepath.Join(opts.AppName, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s

Infracast project for deploying Encore applications to %s.

## Quick Start

1. Configure resources in `+"`"+`infracast.yaml`+"`"+`
2. Run `+"`"+`infracast provision`+"`"+` to create infrastructure
3. Run `+"`"+`infracast deploy`+"`"+` to deploy your application

## Documentation

See https://github.com/DaviRain-Su/Infracast/docs for detailed documentation.
`, opts.AppName, opts.Provider)
	readmePath := filepath.Join(opts.AppName, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	return nil
}

// generateConfigContent generates the initial config file content
func generateConfigContent(opts InitOptions) string {
	return fmt.Sprintf(`# Infracast Configuration
# https://github.com/DaviRain-Su/Infracast/docs

app_name: %s
provider: %s
region: %s

# Environment configuration
environments:
  dev:
    description: Development environment
    # Add resource overrides for dev here
  
  staging:
    description: Staging environment
    # Add resource overrides for staging here
  
  production:
    description: Production environment
    # Add resource overrides for production here

# Resource defaults (applied to all environments)
resources:
  # PostgreSQL database
  # sql_servers:
  #   main:
  #     instance_class: pg.n2.medium.1
  #     storage: 20
  
  # Redis cache
  # redis:
  #   cache:
  #     node_type: redis.master.small.default
  
  # Object storage
  # object_storage:
  #   assets:
  #     storage_class: STANDARD

# Notification settings (optional)
# notifications:
#   feishu:
#     webhook_url: "https://open.feishu.cn/..."
#   dingtalk:
#     webhook_url: "https://oapi.dingtalk.com/..."
`, opts.AppName, opts.Provider, opts.Region)
}
