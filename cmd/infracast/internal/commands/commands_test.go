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
