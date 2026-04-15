package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewDeployCommand validates deploy command structure
func TestNewDeployCommand(t *testing.T) {
	cmd := newDeployCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "deploy", cmd.Name())
	assert.Equal(t, "Deploy application to cloud environment", cmd.Short)
}

// TestDeployCommandFlags validates all deploy flags with defaults
func TestDeployCommandFlags(t *testing.T) {
	cmd := newDeployCommand()

	tests := []struct {
		name     string
		flag     string
		defValue string
	}{
		{"env", "env", "dev"},
		{"verbose", "verbose", "false"},
		{"skip-build", "skip-build", "false"},
		{"skip-verify", "skip-verify", "false"},
		{"dry-run", "dry-run", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flag)
			assert.NotNil(t, f, "flag %s should exist", tt.flag)
			assert.Equal(t, tt.defValue, f.DefValue, "flag %s default", tt.flag)
		})
	}
}

// TestDeployCommandShorthands validates flag shorthands
func TestDeployCommandShorthands(t *testing.T) {
	cmd := newDeployCommand()

	envFlag := cmd.Flags().ShorthandLookup("e")
	assert.NotNil(t, envFlag, "env should have shorthand -e")

	verboseFlag := cmd.Flags().ShorthandLookup("v")
	assert.NotNil(t, verboseFlag, "verbose should have shorthand -v")
}

// TestDeployCommandRegistered validates deploy is in root command
func TestDeployCommandRegistered(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")
	names := make(map[string]bool)
	for _, cmd := range root.Commands() {
		names[cmd.Name()] = true
	}
	assert.True(t, names["deploy"], "deploy command should be registered")
}

// TestValidateEnvironment validates environment name validation
func TestValidateEnvironment(t *testing.T) {
	// Well-known defaults should always pass (even without state store)
	tests := []struct {
		env     string
		wantErr bool
	}{
		{"dev", false},
		{"staging", false},
		{"production", false},
		{"local", false},
		{"", true},
	}

	for _, tt := range tests {
		name := tt.env
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			err := validateEnvironment(tt.env)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateEnvironmentUnknownGivesGuidance validates error message for unknown env
func TestValidateEnvironmentUnknownGivesGuidance(t *testing.T) {
	err := validateEnvironment("nonexistent-env-xyz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "env create")
	assert.Contains(t, err.Error(), "nonexistent-env-xyz")
}

// TestBuildDeploySteps validates step construction based on options
func TestBuildDeploySteps(t *testing.T) {
	config := &DeployConfig{
		AppName:     "test-app",
		Environment: "dev",
		Provider:    "alicloud",
		Region:      "cn-hangzhou",
	}

	t.Run("all steps enabled", func(t *testing.T) {
		opts := DeployOptions{Env: "dev"}
		steps := buildDeploySteps(opts, config)
		assert.Len(t, steps, 5) // build, push, provision, deploy, verify
		assert.Equal(t, "build", steps[0].Name)
		assert.Equal(t, "verify", steps[4].Name)
	})

	t.Run("skip build", func(t *testing.T) {
		opts := DeployOptions{Env: "dev", SkipBuild: true}
		steps := buildDeploySteps(opts, config)
		assert.Len(t, steps, 4) // push, provision, deploy, verify
		assert.Equal(t, "push", steps[0].Name)
	})

	t.Run("skip verify", func(t *testing.T) {
		opts := DeployOptions{Env: "dev", SkipVerify: true}
		steps := buildDeploySteps(opts, config)
		assert.Len(t, steps, 4) // build, push, provision, deploy
		assert.Equal(t, "deploy", steps[3].Name)
	})

	t.Run("skip both", func(t *testing.T) {
		opts := DeployOptions{Env: "dev", SkipBuild: true, SkipVerify: true}
		steps := buildDeploySteps(opts, config)
		assert.Len(t, steps, 3) // push, provision, deploy
	})
}

// TestBuildPipelineInput validates pipeline input construction
func TestBuildPipelineInput(t *testing.T) {
	cfg := &DeployConfig{
		AppName:     "test-app",
		Environment: "staging",
		Provider:    "alicloud",
		Region:      "cn-shanghai",
	}

	input := buildPipelineInput(cfg)
	assert.Equal(t, "test-app", input.AppName)
	assert.Equal(t, "staging", input.Env)
	assert.Equal(t, "cn-shanghai", input.ACRRegion)
	assert.Equal(t, 1, input.Replicas)
	assert.Equal(t, 8080, input.Port)
}

// TestExtractErrorCode validates error code extraction
func TestExtractErrorCode(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{"ECFG code", "ECFG001: config not found", "ECFG001"},
		{"EDEPLOY code", "step failed: EDEPLOY003: timeout", "EDEPLOY003"},
		{"EPROV code", "EPROV010: resource creation failed", "EPROV010"},
		{"EIGEN code", "EIGEN002: generation error", "EIGEN002"},
		{"ESTATE code", "ESTATE005: state mismatch", "ESTATE005"},
		{"no code", "some random error", ""},
		{"prefix only", "ECFG without digits", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractErrorCode(tt.msg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExtractRequestID validates request ID extraction
func TestExtractRequestID(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{"RequestId colon", "Error: RequestId: ABC-123-DEF something", "ABC-123-DEF"},
		{"requestId equals", "requestId=XYZ789 other", "XYZ789"},
		{"RequestID colon", "RequestID: ID-456", "ID-456"},
		{"no request ID", "just an error", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRequestID(tt.msg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDeployOptionsFields validates DeployOptions struct
func TestDeployOptionsFields(t *testing.T) {
	opts := DeployOptions{
		Env:        "staging",
		Verbose:    true,
		SkipBuild:  true,
		SkipVerify: false,
		DryRun:     true,
	}

	assert.Equal(t, "staging", opts.Env)
	assert.True(t, opts.Verbose)
	assert.True(t, opts.SkipBuild)
	assert.False(t, opts.SkipVerify)
	assert.True(t, opts.DryRun)
}

// TestLoadDeployConfig validates config loading falls back to defaults when no config file
func TestLoadDeployConfig(t *testing.T) {
	// Without infracast.yaml, should fall back to defaults
	cfg, err := loadDeployConfig("dev")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "dev", cfg.Environment)
	assert.Equal(t, "alicloud", cfg.Provider)
	assert.Equal(t, "cn-hangzhou", cfg.Region)
	assert.NotEmpty(t, cfg.Resources)
}

// TestDeployConfigResourceInfo validates ResourceInfo struct
func TestDeployConfigResourceInfo(t *testing.T) {
	config, _ := loadDeployConfig("dev")
	assert.GreaterOrEqual(t, len(config.Resources), 1)

	for _, r := range config.Resources {
		assert.NotEmpty(t, r.Type)
		assert.NotEmpty(t, r.Name)
	}
}
