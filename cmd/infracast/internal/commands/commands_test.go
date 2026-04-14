package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "dev", flag.DefValue)
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
