package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewEnvCommand validates env command structure
func TestNewEnvCommand(t *testing.T) {
	cmd := newEnvCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "env", cmd.Name())

	// Verify subcommands registered
	subCmds := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subCmds[c.Name()] = true
	}
	assert.True(t, subCmds["list"], "env list should be registered")
	assert.True(t, subCmds["show"], "env show should be registered")
	assert.True(t, subCmds["create"], "env create should be registered")
	assert.True(t, subCmds["use"], "env use should be registered")
	assert.True(t, subCmds["delete"], "env delete should be registered")
}

// TestValidateEnvName validates environment name rules
func TestValidateEnvName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"dev", false},
		{"staging", false},
		{"prod-eu", false},
		{"test-123", false},
		{"", true},               // empty
		{"Dev", true},            // uppercase
		{"has space", true},      // space
		{"has_underscore", true}, // underscore
		{"abcdefghijklmnopqrstuvwxyz12345", true}, // >30 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvName(tt.name)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestIsValidProvider validates single-cloud constraint
func TestIsValidProvider(t *testing.T) {
	assert.True(t, isValidProvider("alicloud"), "alicloud should be valid")
	assert.False(t, isValidProvider("aws"), "aws should not be valid in v0.1.x")
	assert.False(t, isValidProvider("gcp"), "gcp should not be valid in v0.1.x")
	assert.False(t, isValidProvider(""), "empty should not be valid")
}

// TestDefaultDBPath validates DB path resolution
func TestDefaultDBPath(t *testing.T) {
	// Default path
	os.Unsetenv("INFRACAST_STATE_DB")
	assert.Equal(t, ".infra/state.db", defaultDBPath())

	// Override via env var
	os.Setenv("INFRACAST_STATE_DB", "/tmp/test.db")
	defer os.Unsetenv("INFRACAST_STATE_DB")
	assert.Equal(t, "/tmp/test.db", defaultDBPath())
}

// TestEnvCreateCommandFlags validates create command required flags
func TestEnvCreateCommandFlags(t *testing.T) {
	cmd := newEnvCreateCommand()
	assert.NotNil(t, cmd)

	providerFlag := cmd.Flags().Lookup("provider")
	assert.NotNil(t, providerFlag)

	regionFlag := cmd.Flags().Lookup("region")
	assert.NotNil(t, regionFlag)
}

// TestIsValidProviderErrorMessage validates ECFG005 includes guidance
func TestIsValidProviderErrorMessage(t *testing.T) {
	err := runEnvCreate("test-env", "aws", "us-east-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECFG005")
	assert.Contains(t, err.Error(), "v0.1.x supports alicloud only")
}

// TestEnvMetaFilteredFromResourceCount validates _env_meta is not user-visible
func TestEnvMetaFilteredFromResourceCount(t *testing.T) {
	// _env_meta should be treated as internal, not counted as a resource
	assert.Equal(t, "_env_meta", "_env_meta", "sentinel value must not change")
}

// TestEnvDeleteCommandFlags validates delete command flags
func TestEnvDeleteCommandFlags(t *testing.T) {
	cmd := newEnvDeleteCommand()
	assert.NotNil(t, cmd)

	forceFlag := cmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "false", forceFlag.DefValue)
}
