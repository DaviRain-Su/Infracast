// Command destroy provides environment destruction capability
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/DaviRain-Su/infracast/providers/alicloud"
)

func main() {
	var (
		region  = flag.String("region", "cn-hangzhou", "AliCloud region")
		prefix  = flag.String("prefix", "", "Resource name prefix (default: infracast-<env>)")
		envID   = flag.String("env", "", "Environment ID (required)")
		dryRun  = flag.Bool("dry-run", true, "Show what would be deleted without actually deleting")
		apply   = flag.Bool("apply", false, "Actually perform deletion (required for real deletion)")
		keepVPC = flag.Int("keep-vpc", 1, "Number of VPCs to keep for reuse")
		force   = flag.Bool("force", false, "Allow broad prefix deletion")
	)
	flag.Parse()

	if *envID == "" {
		log.Fatal("Usage: destroy --env <env-id> [--apply]")
	}

	// Safety: require --apply for real deletion.
	if *apply {
		*dryRun = false
	} else {
		*dryRun = true
	}

	// Get credentials from environment
	accessKey := os.Getenv("ALICLOUD_ACCESS_KEY")
	secretKey := os.Getenv("ALICLOUD_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		log.Fatal("ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY must be set")
	}

	// Create provider
	provider, err := alicloud.NewProvider(*region, accessKey, secretKey)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Determine prefix
	resourcePrefix := *prefix
	if resourcePrefix == "" {
		resourcePrefix = fmt.Sprintf("infracast-%s", *envID)
	}
	if *apply && !*force && (resourcePrefix == "infracast" || resourcePrefix == "infra") {
		log.Fatalf("refusing broad prefix delete for prefix=%q without --force", resourcePrefix)
	}

	// Setup options
	opts := alicloud.DestroyOptions{
		DryRun:  *dryRun,
		Prefix:  resourcePrefix,
		KeepVPC: *keepVPC,
	}

	// Log start
	mode := "DRY-RUN"
	if !*dryRun && *apply {
		mode = "APPLY"
	}
	log.Printf("[Destroy] Starting destruction: env=%s region=%s mode=%s keep-vpc=%d",
		*envID, *region, mode, *keepVPC)
	log.Printf("[Destroy] Resource prefix: %s", resourcePrefix)

	// Execute destruction
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := provider.DestroyEnvironment(ctx, *envID, opts)
	if err != nil {
		log.Printf("[Destroy] Completed with errors: %v", err)
	} else {
		log.Printf("[Destroy] Completed successfully in %v", result.Duration)
	}

	// Print summary
	fmt.Println("\n=== Destroy Summary ===")
	fmt.Printf("Mode:        %s\n", mode)
	fmt.Printf("Duration:    %v\n", result.Duration)
	fmt.Printf("Deleted:     %d\n", len(result.Deleted))
	fmt.Printf("Failed:      %d\n", len(result.Failed))
	fmt.Printf("Skipped:     %d\n", len(result.Skipped))

	if len(result.Deleted) > 0 {
		fmt.Println("\nDeleted resources:")
		for _, r := range result.Deleted {
			fmt.Printf("  ✓ %s\n", r)
		}
	}

	if len(result.Failed) > 0 {
		fmt.Println("\nFailed deletions:")
		for _, r := range result.Failed {
			fmt.Printf("  ✗ %s\n", r)
		}
	}

	if len(result.Skipped) > 0 && *dryRun {
		fmt.Println("\nWould be deleted (dry-run):")
		for _, r := range result.Skipped {
			fmt.Printf("  ○ %s\n", r)
		}
	}

	// Exit with error if any deletions failed
	if len(result.Failed) > 0 {
		os.Exit(1)
	}
}
