package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DaviRain-Su/infracast/internal/config"
	"github.com/DaviRain-Su/infracast/internal/credentials"
	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/internal/provisioner"
	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/providers"
	alicloudprovider "github.com/DaviRain-Su/infracast/providers/alicloud"
)

// newProvisionCommand creates the provision command
func newProvisionCommand() *cobra.Command {
	var (
		env     string
		cfgPath string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision infrastructure resources",
		Long:  "Provision infrastructure resources (databases, caches, object storage) for the specified environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProvision(env, cfgPath, dryRun)
		},
	}

	cmd.Flags().StringVar(&env, "env", "dev", "Target environment")
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "Config file path (default: infracast.yaml)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")

	return cmd
}

// runProvision executes infrastructure provisioning
func runProvision(env, cfgPath string, dryRun bool) error {
	// Load configuration
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("ECFG001: failed to load config: %w", err)
	}

	provider := cfg.Provider
	region := cfg.Region
	if envCfg, ok := cfg.Environments[env]; ok {
		if envCfg.Provider != "" {
			provider = envCfg.Provider
		}
		if envCfg.Region != "" {
			region = envCfg.Region
		}
	}
	if provider == "" {
		provider = "alicloud"
	}
	if region == "" {
		region = "cn-hangzhou"
	}

	// Validate provider (single-cloud constraint)
	if !isValidProvider(provider) {
		return fmt.Errorf("ECFG005: unsupported provider: %s (v0.1.x supports alicloud only)", provider)
	}

	// Load credentials from environment
	accessKey := envAny("ALICLOUD_ACCESS_KEY", "ALICLOUD_ACCESS_KEY_ID")
	secretKey := envAny("ALICLOUD_SECRET_KEY", "ALICLOUD_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("EPROV001: missing credentials — set ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY environment variables")
	}

	// Open state store
	store, err := state.NewStore(defaultDBPath())
	if err != nil {
		return fmt.Errorf("ESTATE001: failed to open state database: %w", err)
	}
	defer store.Close()

	// Setup credentials manager
	credsMgr := credentials.NewManager()
	if err := credsMgr.Store(provider, accessKey, secretKey, region); err != nil {
		return fmt.Errorf("EPROV002: %w", err)
	}

	// Create cloud provider
	cloudProvider, err := alicloudprovider.NewProvider(region, accessKey, secretKey)
	if err != nil {
		return fmt.Errorf("EPROV003: failed to create provider: %w", err)
	}

	// Map config to resource specs
	m := mapper.NewMapper(providers.NewRegistry())
	buildMeta := mapper.BuildMeta{
		AppName: cfg.AppName(),
	}
	// Populate BuildMeta from config overrides
	for name := range cfg.Overrides.Databases {
		buildMeta.Databases = append(buildMeta.Databases, name)
	}
	for name := range cfg.Overrides.Cache {
		buildMeta.Caches = append(buildMeta.Caches, name)
	}
	for name := range cfg.Overrides.ObjectStorage {
		buildMeta.ObjectStores = append(buildMeta.ObjectStores, name)
	}
	// Default resources if none configured
	if len(buildMeta.Databases) == 0 {
		buildMeta.Databases = []string{"main"}
	}
	if len(buildMeta.Caches) == 0 {
		buildMeta.Caches = []string{"cache"}
	}
	specs := m.MapToResourceSpecs(buildMeta)

	// Print banner
	mode := "APPLY"
	if dryRun {
		mode = "DRY-RUN"
	}
	fmt.Printf("[Provision] env=%s provider=%s region=%s mode=%s\n", env, provider, region, mode)
	fmt.Printf("Resources to provision: %d\n\n", len(specs))

	// Create provisioner and execute
	prov := provisioner.NewProvisioner(store, credsMgr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := prov.Provision(ctx, provisioner.ProvisionInput{
		EnvID:     env,
		BuildMeta: buildMeta,
		Resources: specs,
		Provider:  cloudProvider,
		DryRun:    dryRun,
		Credentials: credentials.CredentialConfig{
			Provider: provider,
			Region:   region,
		},
	})

	// Print results
	if result != nil {
		fmt.Println()
		fmt.Println("─── Provision Summary ───")
		fmt.Printf("  Created:  %d\n", result.Summary.Created)
		fmt.Printf("  Updated:  %d\n", result.Summary.Updated)
		fmt.Printf("  Skipped:  %d\n", result.Summary.Skipped)
		fmt.Printf("  Failed:   %d\n", result.Summary.Failed)
		fmt.Printf("  Total:    %d\n", result.Summary.Total)
		fmt.Println("─────────────────────────")

		if result.Summary.Failed > 0 {
			color.Red("\n✗ Provisioning completed with %d failure(s)", result.Summary.Failed)
			for _, e := range result.Errors {
				fmt.Printf("  - %s\n", e.Error())
			}
		} else if result.Success {
			color.Green("\n✓ Provisioning completed successfully!")
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  - Deploy: infracast deploy --env %s\n", env)
			fmt.Printf("  - Status: infracast status --env %s\n", env)
		}
	}

	return err
}
