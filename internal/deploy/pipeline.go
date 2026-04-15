// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaviRain-Su/infracast/internal/infragen"
	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/providers/alicloud"
)

// Pipeline orchestrates the full deployment process
type Pipeline struct {
	builder         *Builder
	acrClient       *ACRClient
	k8sClient       *K8sClient
	healthChecker   *HealthChecker
	rollbackManager *RollbackManager
	auditStore      *state.AuditStore
	verbose         bool
}

// PipelineInput contains all inputs for pipeline execution
type PipelineInput struct {
	AppName         string
	Env             string
	Commit          string
	ImageTag        string
	ConfigPath      string
	LocalImage      string
	Replicas        int
	Port            int
	EnvVars         map[string]string
	ACRNamespace    string
	ACRRegion       string
	ACKubeConfig    string
	ACKClusterID    string
	ResourceOutputs []infragen.ResourceOutput // Provisioned resource outputs for config generation
	BuildResult     *BuildResult              // Build step output
	AliAccessKey    string                    // AliCloud access key
	AliSecretKey    string                    // AliCloud secret key
}

// PipelineResult contains the outcome of pipeline execution
type PipelineResult struct {
	Success      bool
	ExitCode     int
	Steps        []StepResult
	Error        error
	Duration     time.Duration
	FinalImage   string
}

// StepResult represents the outcome of a single step
type StepResult struct {
	Name      string
	Success   bool
	Duration  time.Duration
	Error     error
	Output    string
}

// NewPipeline creates a new deployment pipeline
func NewPipeline(verbose bool) *Pipeline {
	return &Pipeline{
		verbose: verbose,
	}
}

// SetAuditStore sets the audit store for logging
func (p *Pipeline) SetAuditStore(store *state.AuditStore) {
	p.auditStore = store
}

// Execute runs all 7 deployment steps
func (p *Pipeline) Execute(ctx context.Context, input *PipelineInput) *PipelineResult {
	start := time.Now()
	result := &PipelineResult{
		Steps: make([]StepResult, 0, 7),
	}

	// Log deployment start
	if p.auditStore != nil {
		_ = p.auditStore.Log(ctx, state.AuditLevelInfo, state.AuditActionDeploy,
			fmt.Sprintf("Deployment started for %s to %s", input.AppName, input.Env),
			state.WithAuditEnv(input.Env),
			state.WithAuditDetail("commit", input.Commit),
			state.WithAuditDetail("image_tag", input.ImageTag),
		)
	}

	// Setup graceful shutdown handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		p.log("Received interrupt signal, initiating graceful shutdown...")
		cancel()
	}()

	// Step 1: Build (delegated to encore build)
	step1 := p.executeStep("Build", func() error {
		return p.stepBuild(ctx, input)
	})
	result.Steps = append(result.Steps, step1)
	if !step1.Success {
		return p.finalizeResult(result, start, 1, step1.Error)
	}

	// Step 2: Push to ACR
	step2 := p.executeStep("Push to ACR", func() error {
		return p.stepPush(ctx, input)
	})
	result.Steps = append(result.Steps, step2)
	if !step2.Success {
		return p.finalizeResult(result, start, 2, step2.Error)
	}

	// Step 3: Provision Infrastructure
	step3 := p.executeStep("Provision Infrastructure", func() error {
		return p.stepProvision(ctx, input)
	})
	result.Steps = append(result.Steps, step3)
	if !step3.Success {
		return p.finalizeResult(result, start, 3, step3.Error)
	}

	// Step 4: Generate infracfg.json
	step4 := p.executeStep("Generate Config", func() error {
		return p.stepGenerateConfig(ctx, input)
	})
	result.Steps = append(result.Steps, step4)
	if !step4.Success {
		return p.finalizeResult(result, start, 4, step4.Error)
	}

	// Step 5: Deploy to K8s
	step5 := p.executeStep("Deploy to K8s", func() error {
		return p.stepDeploy(ctx, input)
	})
	result.Steps = append(result.Steps, step5)
	if !step5.Success {
		return p.finalizeResult(result, start, 5, step5.Error)
	}

	// Step 6: Verify Health
	step6 := p.executeStep("Verify Health", func() error {
		return p.stepVerify(ctx, input)
	})
	result.Steps = append(result.Steps, step6)
	if !step6.Success {
		// On verify failure, trigger rollback
		p.log("Health check failed, initiating rollback...")
		rollbackStep := p.executeStep("Rollback", func() error {
			return p.stepRollback(ctx, input)
		})
		result.Steps = append(result.Steps, rollbackStep)
		return p.finalizeResult(result, start, 6, step6.Error)
	}

	// Step 7: Notify
	step7 := p.executeStep("Notify", func() error {
		return p.stepNotify(ctx, input)
	})
	result.Steps = append(result.Steps, step7)
	// Notify failure is non-blocking

	return p.finalizeResult(result, start, 0, nil)
}

// executeStep executes a single step with timing
func (p *Pipeline) executeStep(name string, fn func() error) StepResult {
	p.logf("Step: %s...", name)
	start := time.Now()
	
	err := fn()
	duration := time.Since(start)
	
	result := StepResult{
		Name:     name,
		Duration: duration,
		Success:  err == nil,
		Error:    err,
	}
	
	if err != nil {
		p.logf("  ✗ Failed: %v", err)
	} else {
		p.logf("  ✓ Success (%v)", duration)
	}
	
	return result
}

// finalizeResult sets the final result fields
func (p *Pipeline) finalizeResult(result *PipelineResult, start time.Time, exitCode int, err error) *PipelineResult {
	result.Duration = time.Since(start)
	result.ExitCode = exitCode
	result.Error = err
	result.Success = exitCode == 0

	// Log deployment completion
	if p.auditStore != nil {
		level := state.AuditLevelInfo
		message := "Deployment completed successfully"
		if !result.Success {
			level = state.AuditLevelError
			message = "Deployment failed"
		}
		_ = p.auditStore.Log(context.Background(), level, state.AuditActionDeploy,
			message,
			state.WithAuditDetail("exit_code", exitCode),
			state.WithAuditDetail("duration_ms", result.Duration.Milliseconds()),
		)
	}

	return result
}

// stepBuild executes encore build docker
func (p *Pipeline) stepBuild(ctx context.Context, input *PipelineInput) error {
	p.log("  Building application...")
	
	if p.builder == nil {
		p.builder = NewBuilder()
	}
	
	// Execute encore build
	buildResult, err := p.builder.Build(ctx, input.AppName, input.Commit)
	if err != nil {
		return fmt.Errorf("EDEPLOY070: build failed: %w", err)
	}
	
	if !buildResult.Success {
		return fmt.Errorf("EDEPLOY070: build failed: %v", buildResult.Error)
	}
	
	// Store build result for later steps
	input.BuildResult = buildResult
	input.LocalImage = buildResult.ImageTag
	
	p.logf("  Built image: %s", buildResult.ImageTag)
	return nil
}

// stepPush pushes image to ACR
func (p *Pipeline) stepPush(ctx context.Context, input *PipelineInput) error {
	if p.acrClient == nil {
		return fmt.Errorf("ACR client not initialized")
	}
	
	finalImage, err := p.acrClient.PushImage(ctx, input.LocalImage, input.ImageTag)
	if err != nil {
		return err
	}
	
	p.logf("  Pushed image: %s", finalImage)
	return nil
}

// stepProvision provisions infrastructure via direct provider methods
func (p *Pipeline) stepProvision(ctx context.Context, input *PipelineInput) error {
	p.log("  Provisioning infrastructure...")
	
	// Check required inputs
	if input.BuildResult == nil {
		return fmt.Errorf("EDEPLOY071: build result required for provision")
	}
	
	if input.AliAccessKey == "" || input.AliSecretKey == "" {
		return fmt.Errorf("EDEPLOY073: AliCloud credentials required")
	}
	
	// Create provider instance
	provider, err := alicloud.NewProvider(input.ACRRegion, input.AliAccessKey, input.AliSecretKey)
	if err != nil {
		return fmt.Errorf("EDEPLOY074: failed to create provider: %w", err)
	}
	
	// Map build metadata to resource specs
	meta := input.BuildResult.BuildMeta
	mapperInstance := mapper.NewMapper(nil)
	specs := mapperInstance.MapToResourceSpecs(meta)
	
	// Directly provision each resource (bypass Plan/Apply stubs)
	input.ResourceOutputs = make([]infragen.ResourceOutput, 0, len(specs))
	for _, spec := range specs {
		var output infragen.ResourceOutput
		
		switch spec.Type {
		case "database":
			if spec.DatabaseSpec == nil {
				continue
			}
			dbOutput, err := provider.ProvisionDatabase(ctx, *spec.DatabaseSpec)
			if err != nil {
				return fmt.Errorf("EDEPLOY075: failed to provision database %s: %w", spec.DatabaseSpec.Name, err)
			}
			// TODO: Poll for endpoint if empty, set password
			output = infragen.ResourceOutput{
				Type: "sql_server",
				Name: spec.DatabaseSpec.Name,
				Output: map[string]string{
					"host":     dbOutput.Endpoint,
					"port":     fmt.Sprintf("%d", dbOutput.Port),
					"database": spec.DatabaseSpec.Name,
					"user":     dbOutput.Username,
					"password": dbOutput.Password,
				},
			}
			
		case "cache":
			if spec.CacheSpec == nil {
				continue
			}
			cacheOutput, err := provider.ProvisionCache(ctx, *spec.CacheSpec)
			if err != nil {
				return fmt.Errorf("EDEPLOY075: failed to provision cache %s: %w", spec.CacheSpec.Name, err)
			}
			// TODO: Poll for endpoint if empty
			output = infragen.ResourceOutput{
				Type: "redis",
				Name: spec.CacheSpec.Name,
				Output: map[string]string{
					"host":     cacheOutput.Endpoint,
					"port":     fmt.Sprintf("%d", cacheOutput.Port),
					"password": cacheOutput.Password,
				},
			}
			
		case "object_storage":
			if spec.ObjectStorageSpec == nil {
				continue
			}
			// TODO: Implement ProvisionObjectStorage
			p.logf("  Skipping object storage %s (not yet implemented)", spec.ObjectStorageSpec.Name)
			continue
			
		default:
			p.logf("  Unknown resource type: %s", spec.Type)
			continue
		}
		
		input.ResourceOutputs = append(input.ResourceOutputs, output)
	}
	
	p.logf("  Provisioned %d resources", len(input.ResourceOutputs))
	return nil
}

// stepGenerateConfig generates infracfg.json
func (p *Pipeline) stepGenerateConfig(ctx context.Context, input *PipelineInput) error {
	p.log("  Generating infracfg.json...")
	
	// Create config generator
	generator := infragen.NewGenerator()
	
	// Generate configuration from resource outputs
	// TODO: Get BuildMeta from build step
	meta := mapper.BuildMeta{AppName: input.AppName}
	cfg, err := generator.Generate(input.ResourceOutputs, meta, input.Env)
	if err != nil {
		return fmt.Errorf("EIGEN001: failed to generate config: %w", err)
	}
	
	// Write configuration to file
	configPath := input.ConfigPath
	if configPath == "" {
		configPath = "infracfg.json"
	}
	
	if err := generator.Write(cfg, configPath); err != nil {
		return fmt.Errorf("EIGEN003: failed to write config: %w", err)
	}
	
	p.log("  Generated: " + configPath)
	return nil
}

// stepDeploy deploys to K8s
func (p *Pipeline) stepDeploy(ctx context.Context, input *PipelineInput) error {
	if p.k8sClient == nil {
		return fmt.Errorf("EDEPLOY010: K8s client not initialized")
	}
	
	p.log("  Generating K8s manifests...")
	
	// Create deploy config
	deployCfg := &DeployConfig{
		AppName:    input.AppName,
		Env:        input.Env,
		Image:      input.ImageTag,
		Commit:     input.Commit,
		Replicas:   input.Replicas,
		Port:       input.Port,
		EnvVars:    input.EnvVars,
		ConfigPath: input.ConfigPath,
	}
	
	// Generate manifests
	resources, err := p.k8sClient.GenerateManifests(deployCfg, nil)
	if err != nil {
		return fmt.Errorf("EDEPLOY012: failed to generate manifests: %w", err)
	}
	
	p.log("  Applying manifests to cluster...")
	
	// Apply manifests
	if err := p.k8sClient.Apply(ctx, resources); err != nil {
		return fmt.Errorf("EDEPLOY014: failed to apply manifests: %w", err)
	}
	
	p.log("  Deployment applied successfully")
	return nil
}

// stepVerify verifies deployment health
func (p *Pipeline) stepVerify(ctx context.Context, input *PipelineInput) error {
	if p.healthChecker == nil {
		return fmt.Errorf("EDEPLOY050: health checker not initialized")
	}
	
	p.log("  Verifying deployment health...")
	
	// Check deployment status with 5 minute timeout
	timeout := 5 * time.Minute
	if err := p.healthChecker.CheckStatus(ctx, input.AppName, timeout); err != nil {
		return fmt.Errorf("EDEPLOY050: health verification failed: %w", err)
	}
	
	// Additional health check via HTTP endpoint
	if input.Port > 0 {
		if err := p.healthChecker.VerifyHealth(ctx, input.AppName, input.Port); err != nil {
			return fmt.Errorf("EDEPLOY057: application health check failed: %w", err)
		}
	}
	
	p.log("  Health verification passed")
	return nil
}

// stepRollback performs rollback on failure
func (p *Pipeline) stepRollback(ctx context.Context, input *PipelineInput) error {
	if p.rollbackManager == nil {
		return fmt.Errorf("rollback manager not initialized")
	}
	
	p.log("  Rolling back deployment...")
	return p.rollbackManager.Rollback(ctx, input.AppName, RollbackStrategyK8s)
}

// stepNotify sends notifications
func (p *Pipeline) stepNotify(ctx context.Context, input *PipelineInput) error {
	p.log("  Sending notifications...")
	// TODO: Send Feishu/DingTalk notifications
	return nil
}

// log prints a log message if verbose mode is enabled
func (p *Pipeline) log(msg string) {
	if p.verbose {
		fmt.Println(msg)
	}
}

// logf prints a formatted log message if verbose mode is enabled
func (p *Pipeline) logf(format string, args ...interface{}) {
	if p.verbose {
		fmt.Printf(format+"\n", args...)
	}
}

// Exit codes per Tech Spec §9.3
const (
	ExitCodeSuccess         = 0
	ExitCodeBuildFailed     = 1
	ExitCodePushFailed      = 2
	ExitCodeProvisionFailed = 3
	ExitCodeConfigFailed    = 4
	ExitCodeDeployFailed    = 5
	ExitCodeVerifyFailed    = 6
	ExitCodeRollbackFailed  = 7
	ExitCodeInterrupted     = 130
)
