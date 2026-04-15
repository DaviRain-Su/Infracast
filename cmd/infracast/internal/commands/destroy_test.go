package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDestroyCommandStructure validates destroy command structure
func TestDestroyCommandStructure(t *testing.T) {
	cmd := newDestroyCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "destroy", cmd.Name())
	assert.Equal(t, "Destroy infrastructure resources", cmd.Short)
}

// TestDestroyCommandFlags validates all destroy flags with defaults
func TestDestroyCommandFlags(t *testing.T) {
	cmd := newDestroyCommand()

	tests := []struct {
		name     string
		flag     string
		defValue string
	}{
		{"env", "env", "dev"},
		{"prefix", "prefix", ""},
		{"dry-run", "dry-run", "true"},
		{"apply", "apply", "false"},
		{"keep-vpc", "keep-vpc", "1"},
		{"force", "force", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flag)
			assert.NotNil(t, f, "flag %s should exist", tt.flag)
			assert.Equal(t, tt.defValue, f.DefValue, "flag %s default", tt.flag)
		})
	}
}

// TestDestroyCommandRegistered validates destroy is in root command
func TestDestroyCommandRegistered(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")
	names := make(map[string]bool)
	for _, cmd := range root.Commands() {
		names[cmd.Name()] = true
	}
	assert.True(t, names["destroy"], "destroy command should be registered")
}

// TestDestroyRegionFlag validates region flag has ALICLOUD_REGION default
func TestDestroyRegionFlag(t *testing.T) {
	// Without env var, should default to cn-hangzhou
	os.Unsetenv("ALICLOUD_REGION")
	cmd := newDestroyCommand()
	f := cmd.Flags().Lookup("region")
	assert.NotNil(t, f)
	assert.Equal(t, "cn-hangzhou", f.DefValue)
}

// TestDefaultRegion validates defaultRegion helper
func TestDefaultRegion(t *testing.T) {
	// Default fallback
	os.Unsetenv("ALICLOUD_REGION")
	assert.Equal(t, "cn-hangzhou", defaultRegion())

	// Override via env var
	os.Setenv("ALICLOUD_REGION", "cn-shanghai")
	defer os.Unsetenv("ALICLOUD_REGION")
	assert.Equal(t, "cn-shanghai", defaultRegion())
}

// TestEnvAny validates envAny helper
func TestEnvAny(t *testing.T) {
	os.Unsetenv("TEST_KEY_A")
	os.Unsetenv("TEST_KEY_B")

	// No keys set → empty
	assert.Equal(t, "", envAny("TEST_KEY_A", "TEST_KEY_B"))

	// First key set → returns first
	os.Setenv("TEST_KEY_A", "value-a")
	defer os.Unsetenv("TEST_KEY_A")
	assert.Equal(t, "value-a", envAny("TEST_KEY_A", "TEST_KEY_B"))

	// Both set → returns first
	os.Setenv("TEST_KEY_B", "value-b")
	defer os.Unsetenv("TEST_KEY_B")
	assert.Equal(t, "value-a", envAny("TEST_KEY_A", "TEST_KEY_B"))

	// Only second set → returns second
	os.Unsetenv("TEST_KEY_A")
	assert.Equal(t, "value-b", envAny("TEST_KEY_A", "TEST_KEY_B"))
}

// TestDestroyDryRunByDefault validates dry-run is true by default (safety)
func TestDestroyDryRunByDefault(t *testing.T) {
	cmd := newDestroyCommand()
	f := cmd.Flags().Lookup("dry-run")
	assert.NotNil(t, f)
	assert.Equal(t, "true", f.DefValue, "dry-run must default to true for safety")
}

// TestDestroyApplyDefaultFalse validates apply defaults to false (safety)
func TestDestroyApplyDefaultFalse(t *testing.T) {
	cmd := newDestroyCommand()
	f := cmd.Flags().Lookup("apply")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue, "apply must default to false for safety")
}

// TestDestroyKeepVPCDefault validates keep-vpc defaults to 1
func TestDestroyKeepVPCDefault(t *testing.T) {
	cmd := newDestroyCommand()
	f := cmd.Flags().Lookup("keep-vpc")
	assert.NotNil(t, f)
	assert.Equal(t, "1", f.DefValue, "keep-vpc should default to 1 to preserve VPC")
}

// TestDestroyErrorCodesStructured validates destroy uses structured EDESTROY error codes
func TestDestroyErrorCodesStructured(t *testing.T) {
	cmd := newDestroyCommand()

	// Execute with empty --env to trigger EDESTROY001
	cmd.SetArgs([]string{"--env", ""})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDESTROY001")
}

// TestDestroyCredentialErrorCode validates missing credentials returns EDESTROY003
func TestDestroyCredentialErrorCode(t *testing.T) {
	os.Unsetenv("ALICLOUD_ACCESS_KEY")
	os.Unsetenv("ALICLOUD_ACCESS_KEY_ID")
	os.Unsetenv("ALICLOUD_SECRET_KEY")
	os.Unsetenv("ALICLOUD_ACCESS_KEY_SECRET")

	cmd := newDestroyCommand()
	cmd.SetArgs([]string{"--env", "dev", "--apply"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDESTROY003")
}
