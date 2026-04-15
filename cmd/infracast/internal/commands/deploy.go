package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DaviRain-Su/infracast/internal/state"
)

// newDeployCommand creates the deploy command with progress tracking
func newDeployCommand() *cobra.Command {
	var (
		env        string
		verbose    bool
		skipBuild  bool
		skipVerify bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy application to cloud environment",
		Long: `Deploy the Encore application to the specified cloud environment.

This command executes the full deployment pipeline:
1. Build Docker image (encore build)
2. Push to container registry
3. Provision infrastructure resources
4. Deploy to Kubernetes
5. Verify health checks

Examples:
  # Deploy to dev environment
  infracast deploy

  # Deploy to specific environment
  infracast deploy --env production

  # Verbose output with full logs
  infracast deploy --verbose

  # Skip build (use existing image)
  infracast deploy --skip-build

  # Dry run (show what would be deployed)
  infracast deploy --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(DeployOptions{
				Env:        env,
				Verbose:    verbose,
				SkipBuild:  skipBuild,
				SkipVerify: skipVerify,
				DryRun:     dryRun,
			})
		},
	}

	cmd.Flags().StringVarP(&env, "env", "e", "dev", "Target environment (dev, staging, production)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip Docker build (use existing image)")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip health check verification")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deployed without executing")

	return cmd
}

// DeployOptions contains deployment options
type DeployOptions struct {
	Env        string
	Verbose    bool
	SkipBuild  bool
	SkipVerify bool
	DryRun     bool
}

// DeployStep represents a deployment step
type DeployStep struct {
	Name        string
	Description string
	Run         func(ctx context.Context) error
}

// runDeploy executes the deployment workflow
func runDeploy(opts DeployOptions) error {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupted. Cleaning up...")
		cancel()
	}()

	// Print deployment banner
	printDeployBanner(opts)

	// Validate environment
	if err := validateEnvironment(opts.Env); err != nil {
		return fmt.Errorf("EDEPLOY001: %w", err)
	}

	// Load configuration
	config, err := loadDeployConfig(opts.Env)
	if err != nil {
		return fmt.Errorf("ECFG001: failed to load config: %w", err)
	}

	// Dry run mode
	if opts.DryRun {
		return runDryRun(config)
	}

	// Execute deployment steps
	steps := buildDeploySteps(opts, config)

	// Open audit store for logging
	auditStore, auditDB := openAuditStore()
	if auditDB != nil {
		defer auditDB.Close()
	}
	traceID := ""
	if auditStore != nil {
		traceID = state.GenerateTraceID()
		fmt.Printf("Trace ID: %s\n\n", traceID)
	}

	if opts.Verbose {
		return runDeployVerbose(ctx, steps, auditStore, traceID, opts.Env)
	}

	return runDeployWithProgress(ctx, steps, auditStore, traceID, opts.Env)
}

// printDeployBanner prints the deployment banner
func printDeployBanner(opts DeployOptions) {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Println("┌─────────────────────────────────────┐")
	cyan.Printf("│  Deploying to %-22s│\n", opts.Env)
	cyan.Println("└─────────────────────────────────────┘")
	fmt.Println()
}

// buildDeploySteps builds the deployment steps based on options
func buildDeploySteps(opts DeployOptions, config *DeployConfig) []DeployStep {
	var steps []DeployStep

	if !opts.SkipBuild {
		steps = append(steps, DeployStep{
			Name:        "build",
			Description: "Building Docker image",
			Run:         func(ctx context.Context) error { return runBuildStep(ctx, config) },
		})
	}

	steps = append(steps,
		DeployStep{
			Name:        "push",
			Description: "Pushing image to registry",
			Run:         func(ctx context.Context) error { return runPushStep(ctx, config) },
		},
		DeployStep{
			Name:        "provision",
			Description: "Provisioning infrastructure",
			Run:         func(ctx context.Context) error { return runProvisionStep(ctx, config) },
		},
		DeployStep{
			Name:        "deploy",
			Description: "Deploying to Kubernetes",
			Run:         func(ctx context.Context) error { return runK8sDeployStep(ctx, config) },
		},
	)

	if !opts.SkipVerify {
		steps = append(steps, DeployStep{
			Name:        "verify",
			Description: "Verifying deployment health",
			Run:         func(ctx context.Context) error { return runVerifyStep(ctx, config) },
		})
	}

	return steps
}

// stepResult tracks the outcome of a single deploy step
type stepResult struct {
	Name     string
	Ok       bool
	Duration time.Duration
	Err      error
}

// runDeployVerbose runs deployment with verbose output
func runDeployVerbose(ctx context.Context, steps []DeployStep, audit *state.AuditStore, traceID, env string) error {
	results := make([]stepResult, 0, len(steps))
	deployStart := time.Now()

	for i, step := range steps {
		fmt.Printf("[%d/%d] %s...\n", i+1, len(steps), step.Description)

		stepStart := time.Now()
		err := step.Run(ctx)
		elapsed := time.Since(stepStart)

		if err != nil {
			results = append(results, stepResult{Name: step.Name, Ok: false, Duration: elapsed, Err: err})
			color.Red("✗ %s failed", step.Name)
			printErrorHint(err)
			fmt.Printf("  Error: %v\n", err)
			logAuditStep(audit, ctx, traceID, env, step.Name, "fail", elapsed, err)
			printDeploySummary(results, time.Since(deployStart))
			return fmt.Errorf("step %s failed: %w", step.Name, err)
		}

		results = append(results, stepResult{Name: step.Name, Ok: true, Duration: elapsed})
		logAuditStep(audit, ctx, traceID, env, step.Name, "ok", elapsed, nil)
		color.Green("✓ %s completed", step.Name)
		fmt.Println()
	}

	printDeploySummary(results, time.Since(deployStart))
	printDeploySuccess()
	return nil
}

// runDeployWithProgress runs deployment with spinner progress
func runDeployWithProgress(ctx context.Context, steps []DeployStep, audit *state.AuditStore, traceID, env string) error {
	results := make([]stepResult, 0, len(steps))
	deployStart := time.Now()

	for i, step := range steps {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = fmt.Sprintf("[%d/%d] %s ", i+1, len(steps), step.Description)
		s.Start()

		stepStart := time.Now()
		stepDone := make(chan error, 1)
		go func() {
			stepDone <- step.Run(ctx)
		}()

		select {
		case err := <-stepDone:
			s.Stop()
			elapsed := time.Since(stepStart)
			if err != nil {
				results = append(results, stepResult{Name: step.Name, Ok: false, Duration: elapsed, Err: err})
				color.Red("✗ %s", step.Description)
				printErrorHint(err)
				fmt.Printf("  Error: %v\n", err)
				logAuditStep(audit, ctx, traceID, env, step.Name, "fail", elapsed, err)
				printDeploySummary(results, time.Since(deployStart))
				return fmt.Errorf("step %s failed: %w", step.Name, err)
			}
			results = append(results, stepResult{Name: step.Name, Ok: true, Duration: elapsed})
			logAuditStep(audit, ctx, traceID, env, step.Name, "ok", elapsed, nil)
			color.Green("✓ %s", step.Description)

		case <-ctx.Done():
			s.Stop()
			color.Yellow("⚠ %s cancelled", step.Description)
			return fmt.Errorf("deployment cancelled")
		}
	}

	fmt.Println()
	printDeploySummary(results, time.Since(deployStart))
	printDeploySuccess()
	return nil
}

// runDryRun shows what would be deployed
func runDryRun(config *DeployConfig) error {
	color.Yellow("Dry Run Mode - No changes will be made")
	fmt.Println()

	fmt.Println("Configuration:")
	fmt.Printf("  App Name:    %s\n", config.AppName)
	fmt.Printf("  Environment: %s\n", config.Environment)
	fmt.Printf("  Provider:    %s\n", config.Provider)
	fmt.Printf("  Region:      %s\n", config.Region)
	fmt.Println()

	fmt.Println("Deployment Plan:")
	fmt.Println("  1. Build Docker image")
	fmt.Println("  2. Push to container registry")
	fmt.Println("  3. Provision infrastructure resources:")
	for _, res := range config.Resources {
		fmt.Printf("     - %s: %s\n", res.Type, res.Name)
	}
	fmt.Println("  4. Deploy to Kubernetes")
	fmt.Println("  5. Verify health checks")
	fmt.Println()

	color.Green("✓ Dry run complete")
	return nil
}

// printDeploySuccess prints the success message
func printDeploySuccess() {
	color.Green("✓ Deployment completed successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  - Check application status: infracast status")
	fmt.Println("  - View logs: infracast logs")
	fmt.Println("  - Open application: infracast open")
}

// DeployConfig represents deployment configuration
type DeployConfig struct {
	AppName     string
	Environment string
	Provider    string
	Region      string
	Resources   []ResourceInfo
}

// ResourceInfo represents a resource
type ResourceInfo struct {
	Type string
	Name string
}

// validateEnvironment validates the environment name
func validateEnvironment(env string) error {
	validEnvs := []string{"dev", "staging", "production", "local"}
	for _, valid := range validEnvs {
		if valid == env {
			return nil
		}
	}
	return fmt.Errorf("invalid environment: %s (must be one of: dev, staging, production, local)", env)
}

// loadDeployConfig loads the deployment configuration
func loadDeployConfig(env string) (*DeployConfig, error) {
	// TODO: Load from infracast.yaml
	return &DeployConfig{
		AppName:     "my-app",
		Environment: env,
		Provider:    "alicloud",
		Region:      "cn-hangzhou",
		Resources: []ResourceInfo{
			{Type: "sql_server", Name: "main"},
			{Type: "redis", Name: "cache"},
		},
	}, nil
}

// Deployment step implementations (placeholders)

func runBuildStep(ctx context.Context, config *DeployConfig) error {
	// TODO: Implement actual build
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

func runPushStep(ctx context.Context, config *DeployConfig) error {
	// TODO: Implement actual push
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

func runProvisionStep(ctx context.Context, config *DeployConfig) error {
	// TODO: Implement actual provisioning
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

func runK8sDeployStep(ctx context.Context, config *DeployConfig) error {
	// TODO: Implement actual K8s deployment
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

func runVerifyStep(ctx context.Context, config *DeployConfig) error {
	// TODO: Implement actual verification
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

// openAuditStore opens the state DB and returns an audit store (best-effort, nil if unavailable)
func openAuditStore() (*state.AuditStore, *sql.DB) {
	db, err := openStateDB()
	if err != nil {
		return nil, nil
	}
	store := state.NewAuditStore(db)
	if err := store.InitAuditTable(); err != nil {
		db.Close()
		return nil, nil
	}
	return store, db
}

// logAuditStep logs a single deploy step to the audit store
func logAuditStep(audit *state.AuditStore, ctx context.Context, traceID, env, stepName, status string, duration time.Duration, err error) {
	if audit == nil {
		return
	}
	opts := []state.AuditLogOption{
		state.WithAuditTraceID(traceID),
		state.WithAuditStep(stepName),
		state.WithAuditStatus(status),
		state.WithAuditEnv(env),
	}
	if err != nil {
		code := extractErrorCode(err.Error())
		if code != "" {
			opts = append(opts, state.WithAuditErrorCode(code))
		}
		reqID := extractRequestID(err.Error())
		if reqID != "" {
			opts = append(opts, state.WithAuditRequestID(reqID))
		}
	}
	msg := stepName
	if err != nil {
		msg = err.Error()
	}
	audit.LogOperation(ctx, state.AuditActionDeploy, duration, err, opts...)
	_ = msg // message is set by LogOperation via err
}

// extractErrorCode extracts a structured error code (e.g. "EDEPLOY001") from an error message
func extractErrorCode(msg string) string {
	// Match patterns like ECFG001, EDEPLOY001, EPROV003
	prefixes := []string{"ECFG", "EDEPLOY", "EPROV", "EIGEN", "ESTATE"}
	for _, p := range prefixes {
		idx := strings.Index(msg, p)
		if idx >= 0 {
			end := idx + len(p)
			for end < len(msg) && msg[end] >= '0' && msg[end] <= '9' {
				end++
			}
			if end > idx+len(p) {
				return msg[idx:end]
			}
		}
	}
	return ""
}

// extractRequestID extracts a cloud provider request ID from an error message
func extractRequestID(msg string) string {
	// Match patterns like RequestId: XXXX or requestId=XXXX
	for _, pattern := range []string{"RequestId: ", "requestId=", "RequestID: "} {
		idx := strings.Index(msg, pattern)
		if idx >= 0 {
			start := idx + len(pattern)
			end := start
			for end < len(msg) && msg[end] != ' ' && msg[end] != ',' && msg[end] != '\n' {
				end++
			}
			if end > start {
				return msg[start:end]
			}
		}
	}
	return ""
}

// printDeploySummary prints a summary table of all step results
func printDeploySummary(results []stepResult, total time.Duration) {
	fmt.Println()
	fmt.Println("─── Deploy Summary ───")
	passed, failed := 0, 0
	for _, r := range results {
		status := color.GreenString("OK")
		if !r.Ok {
			status = color.RedString("FAIL")
			failed++
		} else {
			passed++
		}
		fmt.Printf("  [%s] %-30s %s\n", status, r.Name, r.Duration.Round(time.Millisecond))
	}
	fmt.Printf("  Total: %d passed, %d failed, %s elapsed\n", passed, failed, total.Round(time.Millisecond))
	fmt.Println("──────────────────────")
}

// printErrorHint prints actionable suggestions for known error patterns
func printErrorHint(err error) {
	msg := err.Error()
	hints := []struct {
		pattern string
		hint    string
	}{
		{"ECFG001", "  Hint: Check that infracast.yaml exists and is valid YAML."},
		{"ECFG002", "  Hint: Run 'infracast env list' to see available environments."},
		{"EDEPLOY001", "  Hint: Valid environments are: dev, staging, production, local."},
		{"KUBECONFIG", "  Hint: Set KUBECONFIG to your cluster config file, e.g. export KUBECONFIG=~/.kube/config"},
		{"NotEnoughBalance", "  Hint: Your cloud account balance is insufficient for node provisioning.\n  Top up your account or try spot instances to lower the cost threshold."},
		{"docker", "  Hint: Ensure Docker daemon is running: 'docker info'"},
		{"registry", "  Hint: Check registry credentials: 'docker login <registry-url>'"},
		{"unauthorized", "  Hint: Your cloud credentials may be expired. Re-authenticate with your provider."},
		{"timeout", "  Hint: The operation timed out. Check network connectivity and retry."},
	}

	for _, h := range hints {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(h.pattern)) {
			color.Yellow(h.hint)
			return
		}
	}
}
