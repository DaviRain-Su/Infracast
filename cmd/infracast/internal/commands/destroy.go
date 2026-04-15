package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DaviRain-Su/infracast/providers/alicloud"

	"github.com/spf13/cobra"
)

// newDestroyCommand creates the destroy command
func newDestroyCommand() *cobra.Command {
	var (
		env     string
		region  string
		prefix  string
		dryRun  bool
		apply   bool
		keepVPC int
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy infrastructure resources",
		Long:  "Destroy all infrastructure resources for the specified environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if env == "" {
				return fmt.Errorf("--env is required")
			}

			// Safety: only --apply performs deletion. Otherwise force dry-run.
			if !apply {
				dryRun = true
			}

			resourcePrefix := strings.TrimSpace(prefix)
			if resourcePrefix == "" {
				resourcePrefix = fmt.Sprintf("infracast-%s", env)
			}

			// Safety guard: broad prefix delete requires explicit force.
			if apply && !force && (resourcePrefix == "infracast" || resourcePrefix == "infra") {
				return fmt.Errorf("refusing broad prefix delete for prefix=%q without --force", resourcePrefix)
			}

			accessKey := envAny("ALICLOUD_ACCESS_KEY", "ALICLOUD_ACCESS_KEY_ID")
			secretKey := envAny("ALICLOUD_SECRET_KEY", "ALICLOUD_ACCESS_KEY_SECRET")
			if accessKey == "" || secretKey == "" {
				return fmt.Errorf("missing credentials: set ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY")
			}

			provider, err := alicloud.NewProvider(region, accessKey, secretKey)
			if err != nil {
				return fmt.Errorf("create alicloud provider: %w", err)
			}

			mode := "DRY-RUN"
			if apply && !dryRun {
				mode = "APPLY"
			}
			fmt.Printf("[Destroy] env=%s region=%s mode=%s prefix=%s keep-vpc=%d\n",
				env, region, mode, resourcePrefix, keepVPC)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			result, err := provider.DestroyEnvironment(ctx, env, alicloud.DestroyOptions{
				DryRun:  dryRun,
				Prefix:  resourcePrefix,
				KeepVPC: keepVPC,
			})

			if result != nil {
				fmt.Println("\n=== Destroy Summary ===")
				fmt.Printf("Mode:        %s\n", mode)
				fmt.Printf("Duration:    %v\n", result.Duration)
				fmt.Printf("Deleted:     %d\n", len(result.Deleted))
				fmt.Printf("Failed:      %d\n", len(result.Failed))
				fmt.Printf("Skipped:     %d\n", len(result.Skipped))
			}
			if err != nil {
				return err
			}
			if result != nil && len(result.Failed) > 0 {
				return fmt.Errorf("destroy completed with %d failed resources", len(result.Failed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&env, "env", "dev", "Target environment")
	cmd.Flags().StringVar(&region, "region", defaultRegion(), "AliCloud region")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Resource prefix (default: infracast-<env>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Preview deletion without applying changes")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply real deletion (otherwise dry-run)")
	cmd.Flags().IntVar(&keepVPC, "keep-vpc", 1, "Number of matching VPCs to keep for reuse")
	cmd.Flags().BoolVar(&force, "force", false, "Allow dangerous broad-prefix deletion")

	return cmd
}

func envAny(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func defaultRegion() string {
	if v := envAny("ALICLOUD_REGION"); v != "" {
		return v
	}
	return "cn-hangzhou"
}
