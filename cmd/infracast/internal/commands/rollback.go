package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DaviRain-Su/infracast/internal/deploy"
	"github.com/DaviRain-Su/infracast/internal/state"
)

// newRollbackCommand creates the rollback command
func newRollbackCommand() *cobra.Command {
	var (
		env   string
		image string
	)

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback deployment to a previous image",
		Long: `Rollback the application to a specific container image version.

This command updates the Kubernetes deployment to use the specified image
and waits for the rollout to stabilize. The operation is logged to the
audit store for traceability.

Examples:
  # Rollback to a specific image tag
  infracast rollback --env dev --image registry.cn-hangzhou.aliyuncs.com/infracast/my-app:abc1234

  # Rollback production
  infracast rollback --env production --image registry.cn-hangzhou.aliyuncs.com/infracast/my-app:v1.2.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRollback(env, image)
		},
	}

	cmd.Flags().StringVarP(&env, "env", "e", "dev", "Target environment")
	cmd.Flags().StringVar(&image, "image", "", "Target image to rollback to (required)")
	_ = cmd.MarkFlagRequired("image")

	return cmd
}

// runRollback executes the image-based rollback workflow
func runRollback(env, image string) error {
	// Validate inputs
	if err := validateEnvironment(env); err != nil {
		return fmt.Errorf("EDEPLOY001: %w", err)
	}
	if image == "" {
		return fmt.Errorf("EDEPLOY069: --image is required")
	}

	// Load config to get app name
	cfg, err := loadDeployConfig(env, nil)
	if err != nil {
		return fmt.Errorf("ECFG001: failed to load config: %w", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupted. Cleaning up...")
		cancel()
	}()

	// Open audit store
	auditStore, auditDB := openAuditStore()
	if auditDB != nil {
		defer auditDB.Close()
	}
	traceID := ""
	if auditStore != nil {
		traceID = state.GenerateTraceID()
		fmt.Printf("Trace ID: %s\n\n", traceID)
	}

	color.Cyan("Rolling back %s/%s to image: %s", cfg.AppName, env, image)
	fmt.Println()

	start := time.Now()

	// Initialize K8s client via pipeline helper
	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		return fmt.Errorf("EDEPLOY011: KUBECONFIG environment variable required")
	}

	namespace := env
	if namespace == "" {
		namespace = "default"
	}

	k8sClient, err := deploy.NewK8sClient(namespace, &deploy.K8sConfig{
		KubeConfigPath: kubeConfig,
		Region:         cfg.Region,
	})
	if err != nil {
		return fmt.Errorf("EDEPLOY011: failed to initialize K8s client: %w", err)
	}

	// Execute image-based rollback
	rollbackMgr := deploy.NewRollbackManager(k8sClient)
	rollbackMgr.SetTargetImage(image)

	err = rollbackMgr.Rollback(ctx, cfg.AppName, deploy.RollbackStrategyImage)
	duration := time.Since(start)

	// Log to audit store
	if auditStore != nil {
		rbStatus := "success"
		if err != nil {
			rbStatus = "failure"
		}
		auditStore.LogOperation(ctx, state.AuditActionRollback, duration, err,
			state.WithAuditTraceID(traceID),
			state.WithAuditEnv(env),
			state.WithAuditStep("rollback-image"),
			state.WithAuditStatus(rbStatus),
		)
	}

	if err != nil {
		color.Red("✗ Rollback failed (%s)", duration.Round(time.Millisecond))
		fmt.Printf("  Error: %v\n", err)
		return err
	}

	// Verify health after rollback
	color.Yellow("Verifying deployment health...")
	healthChecker := deploy.NewHealthChecker(k8sClient)
	healthErr := healthChecker.CheckStatus(ctx, cfg.AppName, 5*time.Minute)

	if healthErr != nil {
		color.Red("✗ Health check failed after rollback: %v", healthErr)
		if auditStore != nil {
			auditStore.LogOperation(ctx, state.AuditActionRollback, time.Since(start), healthErr,
				state.WithAuditTraceID(traceID),
				state.WithAuditEnv(env),
				state.WithAuditStep("rollback-verify"),
				state.WithAuditStatus("failure"),
			)
		}
		return fmt.Errorf("EDEPLOY050: rollback deployed but health check failed: %w", healthErr)
	}

	color.Green("✓ Rollback completed successfully (%s)", duration.Round(time.Millisecond))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  - Check status: infracast status --env", env)
	fmt.Println("  - View logs:    infracast logs --trace", traceID)
	return nil
}
