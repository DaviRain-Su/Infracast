package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProvisionCommandStructure validates provision command structure
func TestProvisionCommandStructure(t *testing.T) {
	cmd := newProvisionCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "provision", cmd.Name())
	assert.Equal(t, "Provision infrastructure resources", cmd.Short)
}

// TestProvisionCommandFlags validates all provision flags with defaults
func TestProvisionCommandFlags(t *testing.T) {
	cmd := newProvisionCommand()

	tests := []struct {
		name     string
		flag     string
		defValue string
	}{
		{"env", "env", "dev"},
		{"config", "config", ""},
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

// TestProvisionConfigShorthand validates config has -c shorthand
func TestProvisionConfigShorthand(t *testing.T) {
	cmd := newProvisionCommand()
	f := cmd.Flags().ShorthandLookup("c")
	assert.NotNil(t, f, "config should have shorthand -c")
}

// TestProvisionCommandRegistered validates provision is in root command
func TestProvisionCommandRegistered(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")
	names := make(map[string]bool)
	for _, cmd := range root.Commands() {
		names[cmd.Name()] = true
	}
	assert.True(t, names["provision"], "provision command should be registered")
}

// TestProvisionRequiresConfig validates provision fails gracefully without infracast.yaml
func TestProvisionRequiresConfig(t *testing.T) {
	err := runProvision("dev", "/nonexistent/infracast.yaml", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECFG001")
}

// TestProvisionRequiresCredentials validates provision fails without cloud credentials
func TestProvisionRequiresCredentials(t *testing.T) {
	// Create a minimal config file for testing
	tmpDir := t.TempDir()
	cfgPath := tmpDir + "/infracast.yaml"
	os.WriteFile(cfgPath, []byte("provider: alicloud\nregion: cn-hangzhou\n"), 0644)

	// Unset credentials
	os.Unsetenv("ALICLOUD_ACCESS_KEY")
	os.Unsetenv("ALICLOUD_ACCESS_KEY_ID")
	os.Unsetenv("ALICLOUD_SECRET_KEY")
	os.Unsetenv("ALICLOUD_ACCESS_KEY_SECRET")

	err := runProvision("dev", cfgPath, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EPROV001")
}
