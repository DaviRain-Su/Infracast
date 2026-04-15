// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	provisionStore  *state.Store
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
	Success    bool
	ExitCode   int
	Steps      []StepResult
	Error      error
	Duration   time.Duration
	FinalImage string
}

// StepResult represents the outcome of a single step
type StepResult struct {
	Name     string
	Success  bool
	Duration time.Duration
	Error    error
	Output   string
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

	// Keep state store lifecycle at Execute scope (not per step).
	dbPath := strings.TrimSpace(os.Getenv("INFRACAST_STATE_DB"))
	if dbPath != "" && p.provisionStore == nil {
		if store, err := state.NewStore(dbPath); err == nil {
			p.provisionStore = store
			defer func() {
				_ = p.provisionStore.Close()
				p.provisionStore = nil
			}()
		} else {
			p.logf("state store init skipped: %v", err)
		}
	}

	// Pre-initialize clients when sufficient configuration is present.
	// Missing config is deferred to step-level validation for clearer errors.
	if p.builder == nil {
		p.builder = NewBuilder()
	}
	if input.AliAccessKey != "" && input.AliSecretKey != "" {
		if err := p.initACRClient(input); err != nil {
			p.logf("ACR client pre-init skipped: %v", err)
		}
	}
	if input.ACKubeConfig != "" || os.Getenv("KUBECONFIG") != "" {
		if err := p.initK8sRuntime(input); err != nil {
			p.logf("K8s client pre-init skipped: %v", err)
		}
	}

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

		// Log rollback to audit store for observability
		if p.auditStore != nil {
			rbStatus := "success"
			var rbErr error
			if !rollbackStep.Success {
				rbStatus = "failure"
				rbErr = rollbackStep.Error
			}
			_ = p.auditStore.Log(ctx, state.AuditLevelWarning, state.AuditActionRollback,
				fmt.Sprintf("Rollback triggered for %s/%s (health check failed)", input.AppName, input.Env),
				state.WithAuditEnv(input.Env),
				state.WithAuditStep("rollback"),
				state.WithAuditStatus(rbStatus),
			)
			if rbErr != nil {
				_ = p.auditStore.Log(ctx, state.AuditLevelError, state.AuditActionRollback,
					fmt.Sprintf("Rollback failed: %v", rbErr),
					state.WithAuditEnv(input.Env),
					state.WithAuditStep("rollback"),
					state.WithAuditStatus("failure"),
				)
			}
		}

		return p.finalizeResult(result, start, 6, step6.Error)
	}

	// Step 7: Notify
	step7 := p.executeStep("Notify", func() error {
		return p.stepNotify(ctx, input)
	})
	result.Steps = append(result.Steps, step7)
	// Notify failure is non-blocking
	result.FinalImage = input.ImageTag

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
		if err := p.initACRClient(input); err != nil {
			return err
		}
	}

	localImage := strings.TrimSpace(input.LocalImage)
	if localImage == "" && input.BuildResult != nil {
		localImage = strings.TrimSpace(input.BuildResult.ImageTag)
	}
	if localImage == "" {
		return fmt.Errorf("EDEPLOY040: local image is empty")
	}

	pushTag := strings.TrimSpace(input.ImageTag)
	if idx := strings.LastIndex(pushTag, ":"); idx >= 0 && idx < len(pushTag)-1 {
		pushTag = pushTag[idx+1:]
	}
	if pushTag == "" {
		pushTag = strings.TrimSpace(input.Commit)
	}
	if len(pushTag) > 32 {
		pushTag = pushTag[:32]
	}

	finalImage, err := p.acrClient.PushImage(ctx, localImage, pushTag)
	if err != nil {
		return err
	}

	input.ImageTag = finalImage
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
	provider.SetEnvironment(input.Env)
	if p.provisionStore != nil {
		provider.SetStateStore(p.provisionStore)
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
			if dbOutput.Endpoint == "" {
				return fmt.Errorf("EDEPLOY076: database %s provisioned but endpoint is empty — resource may still be initializing", spec.DatabaseSpec.Name)
			}
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
			if cacheOutput.Endpoint == "" {
				return fmt.Errorf("EDEPLOY076: cache %s provisioned but endpoint is empty — resource may still be initializing", spec.CacheSpec.Name)
			}
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
			ossOutput, err := provider.ProvisionObjectStorage(ctx, *spec.ObjectStorageSpec)
			if err != nil {
				return fmt.Errorf("EDEPLOY075: failed to provision OSS %s: %w", spec.ObjectStorageSpec.Name, err)
			}
			output = infragen.ResourceOutput{
				Type: "object_storage",
				Name: spec.ObjectStorageSpec.Name,
				Output: map[string]string{
					"endpoint": ossOutput.Endpoint,
					"bucket":   ossOutput.BucketName,
					"region":   ossOutput.Region,
				},
			}

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

	// Use rich BuildMeta from build step when available, fall back to AppName only
	meta := mapper.BuildMeta{AppName: input.AppName}
	if input.BuildResult != nil {
		meta = input.BuildResult.BuildMeta
	}
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
	if err := p.initK8sRuntime(input); err != nil {
		return err
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
	if err := p.initK8sRuntime(input); err != nil {
		return err
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
	if err := p.initK8sRuntime(input); err != nil {
		return err
	}

	p.log("  Rolling back deployment...")
	return p.rollbackManager.Rollback(ctx, input.AppName, RollbackStrategyK8s)
}

func (p *Pipeline) initACRClient(input *PipelineInput) error {
	if p.acrClient != nil {
		return nil
	}

	if strings.TrimSpace(input.AliAccessKey) == "" || strings.TrimSpace(input.AliSecretKey) == "" {
		return fmt.Errorf("EDEPLOY072: ACR requires AliCloud credentials")
	}

	region := strings.TrimSpace(input.ACRRegion)
	if region == "" {
		region = "cn-hangzhou"
	}
	namespace := strings.TrimSpace(input.ACRNamespace)
	if namespace == "" {
		namespace = "infracast"
	}

	client, err := NewACRClient(region, input.AliAccessKey, input.AliSecretKey, namespace)
	if err != nil {
		return fmt.Errorf("EDEPLOY072: failed to initialize ACR client: %w", err)
	}
	p.acrClient = client
	return nil
}

func (p *Pipeline) initK8sRuntime(input *PipelineInput) error {
	if p.k8sClient != nil && p.healthChecker != nil && p.rollbackManager != nil {
		return nil
	}

	kubeConfig := strings.TrimSpace(input.ACKubeConfig)
	if kubeConfig == "" {
		kubeConfig = strings.TrimSpace(os.Getenv("KUBECONFIG"))
	}
	if kubeConfig == "" {
		return fmt.Errorf("EDEPLOY011: kubeconfig required (ACKubeConfig or KUBECONFIG)")
	}

	region := strings.TrimSpace(input.ACRRegion)
	if region == "" {
		region = "cn-hangzhou"
	}
	namespace := strings.TrimSpace(input.Env)
	if namespace == "" {
		namespace = "default"
	}

	if p.k8sClient == nil {
		client, err := NewK8sClient(namespace, &K8sConfig{
			KubeConfigPath: kubeConfig,
			ClusterID:      input.ACKClusterID,
			Region:         region,
		})
		if err != nil {
			return err
		}
		p.k8sClient = client
	}
	if p.healthChecker == nil {
		p.healthChecker = NewHealthChecker(p.k8sClient)
	}
	if p.rollbackManager == nil {
		p.rollbackManager = NewRollbackManager(p.k8sClient)
	}

	return nil
}

// stepNotify sends notifications
func (p *Pipeline) stepNotify(ctx context.Context, input *PipelineInput) error {
	p.log("  Sending notifications...")
	message := fmt.Sprintf("infracast deploy success: app=%s env=%s image=%s", input.AppName, input.Env, input.ImageTag)
	urls := p.notifyWebhooks()
	if len(urls) == 0 {
		p.log("  Notification skipped: no webhook configured")
		return nil
	}

	var firstErr error
	for _, webhook := range urls {
		if err := p.postWebhook(ctx, webhook, message); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			p.logf("  Notification failed for %s: %v", webhook, err)
		}
	}
	if firstErr != nil {
		return fmt.Errorf("EDEPLOY080: notify failed: %w", firstErr)
	}
	p.log("  Notification sent")
	return nil
}

func (p *Pipeline) notifyWebhooks() []string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("INFRACAST_NOTIFY_WEBHOOK")),
		strings.TrimSpace(os.Getenv("DINGTALK_WEBHOOK")),
		strings.TrimSpace(os.Getenv("FEISHU_WEBHOOK")),
	}
	urls := make([]string, 0, len(candidates))
	seen := make(map[string]struct{})
	for _, u := range candidates {
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	return urls
}

func (p *Pipeline) postWebhook(ctx context.Context, webhookURL, message string) error {
	payload := map[string]interface{}{
		"msg_type": "text", // Feishu
		"msgtype":  "text", // DingTalk
		"text": map[string]string{
			"content": message,
		},
		"content": map[string]string{
			"text": message,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("status=%d", resp.StatusCode)
	}
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
