package commands

import (
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

// TestProvisionIsStub validates provision currently returns nil (stub behavior)
func TestProvisionIsStub(t *testing.T) {
	cmd := newProvisionCommand()
	// Execute with default flags — stub should succeed without side effects
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.NoError(t, err, "provision stub should succeed")
}
