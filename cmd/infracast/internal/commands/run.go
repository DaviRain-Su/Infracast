package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Default ports for local development
const (
	DefaultLocalPort      = 8080
	DefaultLocalDBPort    = 5432
	DefaultLocalRedisPort = 6379
	DefaultLocalMinioPort = 9000
)

// newRunCommand creates the run command for local development
func newRunCommand() *cobra.Command {
	var (
		workingDir string
		configPath string
		skipGen    bool
		port       int
		devMode    bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run application locally with simulated infrastructure",
		Long: `Run the Encore application locally with infrastructure simulation.

This command:
1. Generates a local infracfg.json with localhost endpoints
2. Sets INFRACFG_PATH environment variable
3. Starts the Encore dev environment

The local configuration uses:
- PostgreSQL on localhost:5432 (or your specified --db-port)
- Redis on localhost:6379 (or your specified --redis-port)
- MinIO on localhost:9000 (or your specified --minio-port)

Examples:
  # Run with default local settings
  infracast run

  # Run in specific directory
  infracast run --workdir ./myapp

  # Run with custom configuration
  infracast run --config ./myconfig.json

  # Skip configuration generation
  infracast run --skip-gen

  # Run with custom ports
  infracast run --port 8080 --db-port 5433
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeRun(workingDir, configPath, skipGen, port, devMode)
		},
	}

	cmd.Flags().StringVarP(&workingDir, "workdir", "w", ".", "Working directory containing Encore app")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to custom infracfg.json")
	cmd.Flags().BoolVar(&skipGen, "skip-gen", false, "Skip local configuration generation")
	cmd.Flags().IntVarP(&port, "port", "p", DefaultLocalPort, "Port for local server")
	cmd.Flags().BoolVar(&devMode, "dev", true, "Enable development mode with hot reload")

	return cmd
}

// RunInput contains input parameters for run command
type RunInput struct {
	WorkingDir string
	ConfigPath string
	SkipGen    bool
	Port       int
	DevMode    bool
	DBPort     int
	RedisPort  int
	MinioPort  int
}

// RunResult contains the result of run command execution
type RunResult struct {
	ConfigPath   string
	InfracfgPath string
	Process      *os.Process
	Duration     time.Duration
}

// LocalConfig represents the local infrastructure configuration
type LocalConfig struct {
	Version      string                   `json:"version"`
	AppName      string                   `json:"app_name"`
	Environment  string                   `json:"environment"`
	LocalMode    bool                     `json:"local_mode"`
	Services     map[string]ServiceConfig `json:"services"`
	Databases    map[string]DBConfig      `json:"databases"`
	Caches       map[string]CacheConfig   `json:"caches,omitempty"`
	ObjectStores map[string]OSSConfig     `json:"object_stores,omitempty"`
}

// ServiceConfig represents a service configuration
type ServiceConfig struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Timeout int    `json:"timeout_seconds"`
}

// DBConfig represents a database configuration
type DBConfig struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	SSLMode  string `json:"ssl_mode"`
}

// CacheConfig represents a cache configuration
type CacheConfig struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
}

// OSSConfig represents an object store configuration
type OSSConfig struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

// executeRun runs the local development environment
func executeRun(workingDir, configPath string, skipGen bool, port int, devMode bool) error {
	input := RunInput{
		WorkingDir: workingDir,
		ConfigPath: configPath,
		SkipGen:    skipGen,
		Port:       port,
		DevMode:    devMode,
		DBPort:     DefaultLocalDBPort,
		RedisPort:  DefaultLocalRedisPort,
		MinioPort:  DefaultLocalMinioPort,
	}

	executor := NewRunExecutor()
	result, err := executor.Execute(context.Background(), input)
	if err != nil {
		return err
	}

	// Print success message
	fmt.Printf("✓ Local development environment started\n")
	fmt.Printf("  Config: %s\n", result.InfracfgPath)
	if result.Process != nil {
		fmt.Printf("  Process: %d\n", result.Process.Pid)
	}

	return nil
}

// RunExecutor executes the local run workflow
type RunExecutor struct {
	timeout time.Duration
}

// NewRunExecutor creates a new run executor
func NewRunExecutor() *RunExecutor {
	return &RunExecutor{
		timeout: 30 * time.Second,
	}
}

// Execute runs the local development environment
func (r *RunExecutor) Execute(ctx context.Context, input RunInput) (*RunResult, error) {
	start := time.Now()

	// Change to working directory if specified
	if input.WorkingDir != "" && input.WorkingDir != "." {
		if err := os.Chdir(input.WorkingDir); err != nil {
			return nil, fmt.Errorf("ECFG001: failed to change to working directory %s: %w", input.WorkingDir, err)
		}
	}

	// Determine config path
	infracfgPath := input.ConfigPath
	if infracfgPath == "" {
		infracfgPath = "./infracfg.json"
	}

	// Generate local configuration if needed
	if !input.SkipGen {
		if err := r.generateLocalConfig(infracfgPath, input); err != nil {
			return nil, fmt.Errorf("ECFG002: failed to generate local config: %w", err)
		}
	}

	// Verify configuration file exists
	if _, err := os.Stat(infracfgPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ECFG003: configuration file not found: %s", infracfgPath)
	}

	// Resolve absolute path for INFRACFG_PATH
	absPath, err := filepath.Abs(infracfgPath)
	if err != nil {
		return nil, fmt.Errorf("ECFG004: failed to resolve config path: %w", err)
	}

	// Set INFRACFG_PATH environment variable
	os.Setenv("INFRACFG_PATH", absPath)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the application
	process, err := r.runEncoreApp(ctx, input, absPath)
	if err != nil {
		return nil, err
	}

	return &RunResult{
		ConfigPath:   input.ConfigPath,
		InfracfgPath: absPath,
		Process:      process,
		Duration:     time.Since(start),
	}, nil
}

// generateLocalConfig creates a local infracfg.json
func (r *RunExecutor) generateLocalConfig(path string, input RunInput) error {
	config := &LocalConfig{
		Version:     "1.0",
		AppName:     "local-app",
		Environment: "local",
		LocalMode:   true,
		Services: map[string]ServiceConfig{
			"api": {
				Name:    "api",
				URL:     fmt.Sprintf("http://localhost:%d", input.Port),
				Timeout: 30,
			},
		},
		Databases: map[string]DBConfig{
			"main": {
				Name:     "main",
				Host:     "localhost",
				Port:     input.DBPort,
				Database: "infracast_local",
				User:     "postgres",
				Password: "postgres",
				SSLMode:  "disable",
			},
		},
	}

	// Add cache if Redis port is specified
	if input.RedisPort > 0 {
		config.Caches = map[string]CacheConfig{
			"default": {
				Name: "default",
				Host: "localhost",
				Port: input.RedisPort,
			},
		}
	}

	// Add object store if MinIO port is specified
	if input.MinioPort > 0 {
		config.ObjectStores = map[string]OSSConfig{
			"storage": {
				Name:      "storage",
				Endpoint:  fmt.Sprintf("http://localhost:%d", input.MinioPort),
				Bucket:    "infracast-local",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
			},
		}
	}

	return WriteConfigFile(path, config)
}

// runEncoreApp runs the Encore application
func (r *RunExecutor) runEncoreApp(ctx context.Context, input RunInput, infracfgPath string) (*os.Process, error) {
	// Check if encore is installed
	if _, err := exec.LookPath("encore"); err != nil {
		return nil, fmt.Errorf("EDEPLOY080: encore CLI not found in PATH, please install it: https://encore.dev/docs/install")
	}

	// Build encore command arguments
	args := []string{"run"}
	if input.DevMode {
		args = append(args, "--watch")
	}

	// Create command with INFRACFG_PATH in environment
	cmd := exec.CommandContext(ctx, "encore", args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("EDEPLOY081: failed to start encore run: %w", err)
	}

	return cmd.Process, nil
}

// WaitForShutdown waits for interrupt signal and gracefully stops the process
func (r *RunExecutor) WaitForShutdown(process *os.Process) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nReceived interrupt signal, shutting down...")

	if process != nil {
		// Try graceful shutdown first
		if err := process.Signal(syscall.SIGTERM); err != nil {
			// Force kill if graceful shutdown fails
			_ = process.Kill()
		}

		// Wait for process to exit with timeout
		done := make(chan error, 1)
		go func() {
			_, err := process.Wait()
			done <- err
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(10 * time.Second):
			// Timeout, force kill
			_ = process.Kill()
		}
	}

	return nil
}

// WriteConfigFile writes a local configuration to file
func WriteConfigFile(path string, config *LocalConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config as JSON
	content, err := jsonMarshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// jsonMarshal marshals config to JSON with indentation
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// LocalRunner provides a high-level interface for local development
type LocalRunner struct {
	executor *RunExecutor
	config   *LocalConfig
}

// NewLocalRunner creates a new local runner
func NewLocalRunner() *LocalRunner {
	return &LocalRunner{
		executor: NewRunExecutor(),
	}
}

// Start initializes and starts the local environment
func (lr *LocalRunner) Start(ctx context.Context, input RunInput) (*RunResult, error) {
	return lr.executor.Execute(ctx, input)
}

// Stop gracefully stops the local environment
func (lr *LocalRunner) Stop(process *os.Process) error {
	return lr.executor.WaitForShutdown(process)
}

// LoadConfig loads an existing local configuration
func (lr *LocalRunner) LoadConfig(path string) (*LocalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ECFG005: failed to read config: %w", err)
	}

	// Parse JSON config
	// In real implementation, use encoding/json
	config := &LocalConfig{
		Version:     "1.0",
		AppName:     "local-app",
		Environment: "local",
		LocalMode:   true,
	}

	_ = data // Use data in real implementation
	return config, nil
}

// ValidateLocalPrerequisites checks if local dependencies are available
func ValidateLocalPrerequisites() error {
	checks := []struct {
		name string
		cmd  string
		hint string
	}{
		{"Encore CLI", "encore", "Install from https://encore.dev/docs/install"},
		{"Go", "go", "Install from https://golang.org/dl/"},
	}

	for _, check := range checks {
		if _, err := exec.LookPath(check.cmd); err != nil {
			return fmt.Errorf("EDEPLOY082: %s not found: %s", check.name, check.hint)
		}
	}

	return nil
}
