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

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DaviRain-Su/infracast/internal/config"
	"github.com/DaviRain-Su/infracast/internal/deploy"
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

	// Execute the pipeline once (was: 4 redundant full-pipeline calls)
	return runDeployPipeline(ctx, opts, config, auditStore, traceID)
}

// printDeployBanner prints the deployment banner
func printDeployBanner(opts DeployOptions) {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Println("┌─────────────────────────────────────┐")
	cyan.Printf("│  Deploying to %-22s│\n", opts.Env)
	cyan.Println("└─────────────────────────────────────┘")
	fmt.Println()
}

// stepResult tracks the outcome of a single deploy step
type stepResult struct {
	Name     string
	Ok       bool
	Duration time.Duration
	Err      error
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
// Checks the state store first, then falls back to well-known defaults
func validateEnvironment(env string) error {
	if env == "" {
		return fmt.Errorf("environment name cannot be empty")
	}

	// Check state store for user-created environments
	store, err := state.NewStore(defaultDBPath())
	if err == nil {
		defer store.Close()
		ctx := context.Background()
		envs, err := store.ListEnvironments(ctx)
		if err == nil {
			for _, e := range envs {
				if e == env {
					return nil
				}
			}
		}
	}

	// Fall back to well-known defaults (for use before first env create)
	wellKnown := []string{"dev", "staging", "production", "local"}
	for _, valid := range wellKnown {
		if valid == env {
			return nil
		}
	}

	return fmt.Errorf("environment not found: %s (create it with 'infracast env create %s --provider alicloud --region cn-hangzhou')", env, env)
}

// loadDeployConfig loads the deployment configuration from infracast.yaml
func loadDeployConfig(env string) (*DeployConfig, error) {
	cfg, err := config.Load("")
	if err != nil {
		// Fall back to defaults if config file not found
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

	provider := cfg.Provider
	region := cfg.Region

	// Check for environment-specific overrides
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

	return &DeployConfig{
		AppName:     "my-app",
		Environment: env,
		Provider:    provider,
		Region:      region,
		Resources: []ResourceInfo{
			{Type: "sql_server", Name: "main"},
			{Type: "redis", Name: "cache"},
		},
	}, nil
}

// runDeployPipeline executes the deploy pipeline once and reports per-step results.
// Before v0.1.4, each CLI step ran the full 7-step pipeline independently (4x redundancy).
func runDeployPipeline(ctx context.Context, opts DeployOptions, cfg *DeployConfig, audit *state.AuditStore, traceID string) error {
	pipeline := deploy.NewPipeline(opts.Verbose)
	input := buildPipelineInput(cfg)

	result := pipeline.Execute(ctx, input)

	// Map pipeline step results to CLI display
	var results []stepResult
	for _, s := range result.Steps {
		sr := stepResult{Name: s.Name, Ok: s.Success, Duration: s.Duration, Err: s.Error}
		results = append(results, sr)

		if s.Success {
			logAuditStep(audit, ctx, traceID, opts.Env, s.Name, "ok", s.Duration, nil)
			color.Green("✓ %s (%s)", s.Name, s.Duration.Round(time.Millisecond))
		} else {
			logAuditStep(audit, ctx, traceID, opts.Env, s.Name, "fail", s.Duration, s.Error)
			color.Red("✗ %s failed", s.Name)
			if s.Error != nil {
				printErrorHint(s.Error)
				fmt.Printf("  Error: %v\n", s.Error)
			}
			printDeploySummary(results, result.Duration)
			return fmt.Errorf("step %s failed: %w", s.Name, s.Error)
		}
	}

	printDeploySummary(results, result.Duration)
	if result.Success {
		printDeploySuccess()
	}
	return result.Error
}

// buildPipelineInput converts DeployConfig to PipelineInput
func buildPipelineInput(cfg *DeployConfig) *deploy.PipelineInput {
	accessKey := os.Getenv("ALICLOUD_ACCESS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("ALICLOUD_ACCESS_KEY_ID")
	}
	secretKey := os.Getenv("ALICLOUD_SECRET_KEY")
	if secretKey == "" {
		secretKey = os.Getenv("ALICLOUD_ACCESS_KEY_SECRET")
	}

	return &deploy.PipelineInput{
		AppName:      cfg.AppName,
		Env:          cfg.Environment,
		ConfigPath:   "infracast.yaml",
		Replicas:     1,
		Port:         8080,
		ACRRegion:    cfg.Region,
		AliAccessKey: accessKey,
		AliSecretKey: secretKey,
	}
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
