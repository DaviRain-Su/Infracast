package commands

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// TestNewProvisionCommand validates provision command structure
func TestNewProvisionCommand(t *testing.T) {
	cmd := newProvisionCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "provision", cmd.Name())
	assert.Equal(t, "Provision infrastructure resources", cmd.Short)

	// Check flags
	flag := cmd.Flags().Lookup("env")
	assert.NotNil(t, flag)
	assert.Equal(t, "dev", flag.DefValue)

	flag = cmd.Flags().Lookup("config")
	assert.NotNil(t, flag)

	flag = cmd.Flags().Lookup("dry-run")
	assert.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
}

// TestNewDestroyCommand validates destroy command structure
func TestNewDestroyCommand(t *testing.T) {
	cmd := newDestroyCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "destroy", cmd.Name())
	assert.Equal(t, "Destroy infrastructure resources", cmd.Short)

	// Check flags
	flag := cmd.Flags().Lookup("env")
	assert.NotNil(t, flag)
	assert.Equal(t, "dev", flag.DefValue)

	flag = cmd.Flags().Lookup("force")
	assert.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
}

// TestNewStatusCommand validates status command structure
func TestNewStatusCommand(t *testing.T) {
	cmd := newStatusCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "status", cmd.Name())
	assert.Equal(t, "Show infrastructure status", cmd.Short)

	// Check flags
	flag := cmd.Flags().Lookup("env")
	assert.NotNil(t, flag)
	assert.Equal(t, "", flag.DefValue)

	// v0.2.0: --output flag
	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "", outputFlag.DefValue)
}

// TestNewRootCommand validates root command includes new subcommands
func TestNewRootCommand(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")
	assert.NotNil(t, root)

	// Check that new commands are registered
	commands := root.Commands()
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	assert.True(t, commandNames["provision"], "provision command should be registered")
	assert.True(t, commandNames["destroy"], "destroy command should be registered")
	assert.True(t, commandNames["status"], "status command should be registered")
}

// TestGlobalFlags validates global flags are properly set
func TestGlobalFlags(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")

	// Check global flags
	configFlag := root.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "c", configFlag.Shorthand)

	envFlag := root.PersistentFlags().Lookup("env")
	assert.NotNil(t, envFlag)
	assert.Equal(t, "e", envFlag.Shorthand)
	assert.Equal(t, "dev", envFlag.DefValue)

	verboseFlag := root.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
	assert.Equal(t, "v", verboseFlag.Shorthand)
}

// v0.2.0 status --output regression tests

func TestBuildEnvStatus_FiltersEnvMeta(t *testing.T) {
	resources := []*state.InfraResource{
		{ResourceName: "_env_meta", ResourceType: "environment", Status: "created", UpdatedAt: time.Now()},
		{ResourceName: "main", ResourceType: "sql_server", Status: "ready", UpdatedAt: time.Now()},
		{ResourceName: "cache", ResourceType: "redis", Status: "failed", ErrorMsg: "EPROV001: timeout", UpdatedAt: time.Now()},
	}

	es := buildEnvStatus("dev", resources, true)
	assert.Equal(t, "dev", es.Name)
	assert.Equal(t, 2, es.Total)
	assert.Equal(t, 1, es.Ready)
	assert.Equal(t, 1, es.Failed)
	assert.Len(t, es.Resources, 2)
}

func TestBuildEnvStatus_WithoutResources(t *testing.T) {
	resources := []*state.InfraResource{
		{ResourceName: "main", ResourceType: "sql_server", Status: "ready", UpdatedAt: time.Now()},
	}

	es := buildEnvStatus("staging", resources, false)
	assert.Equal(t, 1, es.Total)
	assert.Nil(t, es.Resources, "should not include resources when includeResources=false")
}

func TestStatusErrorHint(t *testing.T) {
	tests := []struct {
		errMsg   string
		contains string
	}{
		{"EPROV001: missing credentials", "ALICLOUD_ACCESS_KEY"},
		{"EDEPLOY076: endpoint empty", "still be initializing"},
		{"NotEnoughBalance in region", "Top up"},
		{"some random error", ""},
	}
	for _, tt := range tests {
		hint := statusErrorHint(tt.errMsg)
		if tt.contains == "" {
			assert.Empty(t, hint)
		} else {
			assert.Contains(t, hint, tt.contains)
		}
	}
}

func TestRenderOutput_JSON(t *testing.T) {
	data := StatusOutput{
		Environments: []EnvStatusOutput{
			{Name: "dev", Total: 2, Ready: 1, Failed: 1},
		},
	}

	// Verify it marshals to valid JSON
	b, err := json.MarshalIndent(data, "", "  ")
	assert.NoError(t, err)

	var parsed StatusOutput
	err = json.Unmarshal(b, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "dev", parsed.Environments[0].Name)
	assert.Equal(t, 2, parsed.Environments[0].Total)
}

func TestRenderOutput_YAML(t *testing.T) {
	data := StatusOutput{
		Environments: []EnvStatusOutput{
			{Name: "production", Total: 3, Ready: 3, Failed: 0},
		},
	}

	b, err := yaml.Marshal(data)
	assert.NoError(t, err)

	var parsed StatusOutput
	err = yaml.Unmarshal(b, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "production", parsed.Environments[0].Name)
	assert.Equal(t, 3, parsed.Environments[0].Ready)
}

func TestBuildEnvStatus_ErrorHintPopulated(t *testing.T) {
	resources := []*state.InfraResource{
		{
			ResourceName: "db",
			ResourceType: "sql_server",
			Status:       "failed",
			ErrorMsg:     "EPROV001: missing credentials",
			UpdatedAt:    time.Now(),
		},
	}

	es := buildEnvStatus("dev", resources, true)
	assert.Len(t, es.Resources, 1)
	assert.Equal(t, "EPROV001: missing credentials", es.Resources[0].ErrorMsg)
	assert.Contains(t, es.Resources[0].ErrorHint, "ALICLOUD_ACCESS_KEY")
}

// v0.2.0 --set config override regression tests

func TestParseSetValues_Valid(t *testing.T) {
	vals, err := parseSetValues([]string{"region=cn-shanghai", "replicas=3"})
	assert.NoError(t, err)
	assert.Equal(t, "cn-shanghai", vals["region"])
	assert.Equal(t, "3", vals["replicas"])
}

func TestParseSetValues_EmptySlice(t *testing.T) {
	vals, err := parseSetValues(nil)
	assert.NoError(t, err)
	assert.Empty(t, vals)
}

func TestParseSetValues_InvalidFormat(t *testing.T) {
	_, err := parseSetValues([]string{"no-equals-sign"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECFG006")
}

func TestParseSetValues_EmptyKey(t *testing.T) {
	_, err := parseSetValues([]string{"=value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECFG006")
}

func TestParseSetValues_ValueWithEquals(t *testing.T) {
	vals, err := parseSetValues([]string{"key=val=ue"})
	assert.NoError(t, err)
	assert.Equal(t, "val=ue", vals["key"])
}

func TestApplyOverrides_Region(t *testing.T) {
	dc := &DeployConfig{Region: "cn-hangzhou"}
	applyOverrides(dc, map[string]string{"region": "cn-shanghai"})
	assert.Equal(t, "cn-shanghai", dc.Region)
}

func TestApplyOverrides_Nil(t *testing.T) {
	dc := &DeployConfig{Region: "cn-hangzhou"}
	applyOverrides(dc, nil)
	assert.Equal(t, "cn-hangzhou", dc.Region, "nil overrides should not change config")
}

func TestDeployCommand_SetFlag(t *testing.T) {
	cmd := newDeployCommand()
	flag := cmd.Flags().Lookup("set")
	assert.NotNil(t, flag, "--set flag should exist")
}

func TestNewRollbackCommand(t *testing.T) {
	cmd := newRollbackCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "rollback", cmd.Name())
	assert.Equal(t, "Rollback deployment to a previous image", cmd.Short)

	envFlag := cmd.Flags().Lookup("env")
	assert.NotNil(t, envFlag)
	assert.Equal(t, "dev", envFlag.DefValue)

	imageFlag := cmd.Flags().Lookup("image")
	assert.NotNil(t, imageFlag)
	assert.Equal(t, "", imageFlag.DefValue)
}

func TestRollbackCommandRegistered(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")
	commands := root.Commands()
	found := false
	for _, cmd := range commands {
		if cmd.Name() == "rollback" {
			found = true
			break
		}
	}
	assert.True(t, found, "rollback command should be registered in root")
}

func TestLoadDeployConfig_OverridePriority(t *testing.T) {
	// Without config file, defaults + overrides
	cfg, err := loadDeployConfig("dev", map[string]string{"region": "cn-beijing"})
	assert.NoError(t, err)
	assert.Equal(t, "cn-beijing", cfg.Region, "--set region should override default")
}

func TestBuildPipelineInput_ReplicasOverride(t *testing.T) {
	cfg := &DeployConfig{
		AppName:     "test-app",
		Environment: "dev",
		Region:      "cn-hangzhou",
	}

	// Default replicas = 1
	input := buildPipelineInput(cfg, nil)
	assert.Equal(t, 1, input.Replicas)

	// Override replicas = 5
	input = buildPipelineInput(cfg, map[string]string{"replicas": "5"})
	assert.Equal(t, 5, input.Replicas)

	// Invalid replicas ignored
	input = buildPipelineInput(cfg, map[string]string{"replicas": "abc"})
	assert.Equal(t, 1, input.Replicas)
}
